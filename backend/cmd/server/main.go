package main

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/epw80/chat-analytics-platform/pkg/analytics"
	"github.com/epw80/chat-analytics-platform/pkg/client"
	"github.com/epw80/chat-analytics-platform/pkg/config"
	"github.com/epw80/chat-analytics-platform/pkg/hub"
	"github.com/epw80/chat-analytics-platform/pkg/storage"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		// Allow all origins in development
		// TODO: Restrict in production
		return true
	},
}

type Server struct {
	hub       *hub.Hub
	storage   storage.MessageRepository
	analytics *analytics.Tracker
	logger    *slog.Logger
}

func NewServer(logger *slog.Logger, repo storage.MessageRepository) *Server {
	tracker := analytics.New()
	h := hub.New(logger)
	h.SetAnalytics(tracker)
	return &Server{
		hub:       h,
		storage:   repo,
		analytics: tracker,
		logger:    logger,
	}
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	// Storage is an optional dependency; report its reachability so the
	// endpoint can be used as a readiness probe rather than a bare liveness ping.
	storageStatus := "disabled"
	if s.storage != nil {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		if err := s.storage.HealthCheck(ctx); err != nil {
			storageStatus = "unavailable"
		} else {
			storageStatus = "ok"
		}
	}

	resp := map[string]any{
		"status":  "ok",
		"clients": s.hub.ClientCount(),
		"storage": storageStatus,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		s.logger.Error("failed to encode health response", slog.String("error", err.Error()))
	}
}

func (s *Server) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	// Extract user info from query params (in production, use proper auth)
	userID := r.URL.Query().Get("userId")
	username := r.URL.Query().Get("username")
	room := r.URL.Query().Get("room")

	if userID == "" {
		userID = "anonymous"
	}
	if username == "" {
		username = "Anonymous"
	}

	// Upgrade connection
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		s.logger.Error("failed to upgrade connection",
			slog.String("error", err.Error()))
		return
	}

	// Create and register client
	c := client.New(s.hub, conn, userID, username, s.logger)
	if s.storage != nil {
		c.SetStorage(s.storage)
	}
	c.SetAnalytics(s.analytics)
	c.SetRoom(room) // empty room falls back to the client's default
	s.hub.Register(c)
	c.Start()

	s.logger.Info("new websocket connection",
		slog.String("userID", userID),
		slog.String("username", username),
		slog.String("roomID", c.RoomID()),
		slog.String("clientID", c.ID()))
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) setupRoutes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", s.handleHealth)
	mux.HandleFunc("/ws", s.handleWebSocket)
	mux.HandleFunc("/api/analytics", analytics.NewHandler(s.analytics))
	return corsMiddleware(mux)
}

func main() {
	// Load configuration
	cfg := config.Load()

	// Setup logger with configured level
	logLevel := slog.LevelInfo
	switch cfg.LogLevel {
	case "debug":
		logLevel = slog.LevelDebug
	case "warn":
		logLevel = slog.LevelWarn
	case "error":
		logLevel = slog.LevelError
	}

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: logLevel,
	}))

	logger.Info("loaded configuration",
		slog.String("port", cfg.Port),
		slog.String("dynamodb_endpoint", cfg.DynamoDBEndpoint),
		slog.String("dynamodb_region", cfg.DynamoDBRegion),
		slog.String("log_level", cfg.LogLevel))

	// Initialize DynamoDB storage (graceful degradation if unavailable).
	// repo is kept as the interface type and only assigned on success, so a
	// failed init leaves it as a true nil interface (avoiding the typed-nil
	// trap where a nil *DynamoDBRepository would still compare != nil).
	var repo storage.MessageRepository
	if cfg.DynamoDBEndpoint != "" || cfg.DynamoDBRegion != "" {
		dbCtx, dbCancel := context.WithTimeout(context.Background(), 90*time.Second)
		defer dbCancel()
		db, err := storage.NewDynamoDBRepository(dbCtx, cfg, logger)
		if err != nil {
			logger.Warn("DynamoDB unavailable, running without persistence",
				slog.String("error", err.Error()))
		} else {
			repo = db
		}
	}

	// Create server
	srv := NewServer(logger, repo)

	// Start hub
	go srv.hub.Run()

	// Setup HTTP server
	httpServer := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      srv.setupRoutes(),
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Start HTTP server in goroutine
	go func() {
		logger.Info("starting server", slog.String("port", cfg.Port))
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("server error", slog.String("error", err.Error()))
			os.Exit(1)
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("shutting down server...")

	// Graceful shutdown with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Shutdown HTTP server
	if err := httpServer.Shutdown(ctx); err != nil {
		logger.Error("server forced to shutdown", slog.String("error", err.Error()))
	}

	// Shutdown hub
	srv.hub.Shutdown()

	// Release storage resources
	if srv.storage != nil {
		if err := srv.storage.Close(); err != nil {
			logger.Error("error closing storage", slog.String("error", err.Error()))
		}
	}

	logger.Info("server exited")
}
