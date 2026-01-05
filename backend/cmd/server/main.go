package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/epw80/chat-analytics-platform/pkg/client"
	"github.com/epw80/chat-analytics-platform/pkg/config"
	"github.com/epw80/chat-analytics-platform/pkg/hub"
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
	hub    *hub.Hub
	logger *slog.Logger
}

func NewServer(logger *slog.Logger) *Server {
	return &Server{
		hub:    hub.New(logger),
		logger: logger,
	}
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, `{"status":"ok","clients":%d}`, s.hub.ClientCount())
}

func (s *Server) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	// Extract user info from query params (in production, use proper auth)
	userID := r.URL.Query().Get("userId")
	username := r.URL.Query().Get("username")

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
	s.hub.Register(c)
	c.Start()

	s.logger.Info("new websocket connection",
		slog.String("userID", userID),
		slog.String("username", username),
		slog.String("clientID", c.ID()))
}

func (s *Server) setupRoutes() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", s.handleHealth)
	mux.HandleFunc("/ws", s.handleWebSocket)
	return mux
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

	// Create server
	srv := NewServer(logger)

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

	logger.Info("server exited")
}
