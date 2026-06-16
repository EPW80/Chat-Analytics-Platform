package main

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/epw80/chat-analytics-platform/pkg/analytics"
	"github.com/epw80/chat-analytics-platform/pkg/auth"
	"github.com/epw80/chat-analytics-platform/pkg/client"
	"github.com/epw80/chat-analytics-platform/pkg/config"
	"github.com/epw80/chat-analytics-platform/pkg/hub"
	"github.com/epw80/chat-analytics-platform/pkg/message"
	"github.com/epw80/chat-analytics-platform/pkg/persist"
	"github.com/epw80/chat-analytics-platform/pkg/ratelimit"
	"github.com/epw80/chat-analytics-platform/pkg/storage"
	"github.com/gorilla/websocket"
)

const (
	// Number of recent messages replayed to a client when it joins a room.
	defaultHistoryLimit = 50

	// Upper bound on the number of messages a read API request may return.
	maxHistoryLimit = 200
)

// healthResponse is the JSON body returned by the health endpoint.
type healthResponse struct {
	Status  string `json:"status"`
	Clients int    `json:"clients"`
	Storage string `json:"storage"`
}

// messagesResponse is the JSON body returned by the message history endpoints.
type messagesResponse struct {
	RoomID   string             `json:"roomId,omitempty"`
	UserID   string             `json:"userId,omitempty"`
	Count    int                `json:"count"`
	Messages []*message.Message `json:"messages"`
}

type Server struct {
	hub       *hub.Hub
	storage   storage.MessageRepository
	persister *persist.Writer
	analytics *analytics.Tracker
	auth      *auth.Authenticator
	upgrader  websocket.Upgrader
	logger    *slog.Logger

	allowedOrigins  []string
	rateLimitPerSec float64
	rateLimitBurst  float64
}

func NewServer(logger *slog.Logger, repo storage.MessageRepository, cfg *config.Config) *Server {
	tracker := analytics.New()
	h := hub.New(logger)
	h.SetAnalytics(tracker)

	s := &Server{
		hub:             h,
		storage:         repo,
		analytics:       tracker,
		logger:          logger,
		allowedOrigins:  cfg.AllowedOrigins,
		rateLimitPerSec: cfg.RateLimitPerSec,
		rateLimitBurst:  cfg.RateLimitBurst,
	}

	// Persist via a bounded worker pool only when storage is available.
	if repo != nil {
		s.persister = persist.New(repo, logger, persist.Config{
			Workers:   cfg.PersistWorkers,
			BatchSize: cfg.PersistBatchSize,
			QueueSize: cfg.PersistQueueSize,
		})
	}

	// Enable token auth only when a secret is configured.
	if cfg.AuthSecret != "" {
		s.auth = auth.New(cfg.AuthSecret)
	}

	s.upgrader = websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
		CheckOrigin:     s.originAllowed,
	}

	return s
}

// originAllowed reports whether the request's Origin is permitted by the
// configured allowlist. A "*" entry allows any origin.
func (s *Server) originAllowed(r *http.Request) bool {
	origin := r.Header.Get("Origin")
	for _, o := range s.allowedOrigins {
		if o == "*" || o == origin {
			return true
		}
	}
	return false
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

	s.writeJSON(w, http.StatusOK, healthResponse{
		Status:  "ok",
		Clients: s.hub.ClientCount(),
		Storage: storageStatus,
	})
}

// handleRoomMessages serves the recent message history for a room.
func (s *Server) handleRoomMessages(w http.ResponseWriter, r *http.Request) {
	if s.storage == nil {
		http.Error(w, "message history is unavailable", http.StatusServiceUnavailable)
		return
	}

	roomID := r.PathValue("id")
	if roomID == "" {
		http.Error(w, "room id is required", http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	msgs, err := s.storage.GetRecentMessages(ctx, roomID, parseLimit(r))
	if err != nil {
		s.logger.Error("failed to fetch room messages",
			slog.String("roomID", roomID),
			slog.String("error", err.Error()))
		http.Error(w, "failed to fetch messages", http.StatusInternalServerError)
		return
	}

	s.writeJSON(w, http.StatusOK, messagesResponse{
		RoomID:   roomID,
		Count:    len(msgs),
		Messages: msgs,
	})
}

// handleUserMessages serves the message history for a single user.
func (s *Server) handleUserMessages(w http.ResponseWriter, r *http.Request) {
	if s.storage == nil {
		http.Error(w, "message history is unavailable", http.StatusServiceUnavailable)
		return
	}

	userID := r.PathValue("id")
	if userID == "" {
		http.Error(w, "user id is required", http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	msgs, err := s.storage.GetMessagesByUser(ctx, userID, parseLimit(r))
	if err != nil {
		s.logger.Error("failed to fetch user messages",
			slog.String("userID", userID),
			slog.String("error", err.Error()))
		http.Error(w, "failed to fetch messages", http.StatusInternalServerError)
		return
	}

	s.writeJSON(w, http.StatusOK, messagesResponse{
		UserID:   userID,
		Count:    len(msgs),
		Messages: msgs,
	})
}

// writeJSON encodes v as a JSON response with the given status code.
func (s *Server) writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		s.logger.Error("failed to encode response", slog.String("error", err.Error()))
	}
}

// parseLimit reads the "limit" query parameter, falling back to the default
// and clamping to the configured maximum.
func parseLimit(r *http.Request) int {
	limit := defaultHistoryLimit
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			limit = n
		}
	}
	if limit > maxHistoryLimit {
		limit = maxHistoryLimit
	}
	return limit
}

func (s *Server) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	username := r.URL.Query().Get("username")
	room := r.URL.Query().Get("room")
	if username == "" {
		username = "Anonymous"
	}

	// Resolve the user identity. With auth enabled the userID comes from a
	// verified token; otherwise it falls back to the (spoofable) query param.
	userID, ok := s.authenticate(r)
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	// Upgrade connection
	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		s.logger.Error("failed to upgrade connection",
			slog.String("error", err.Error()))
		return
	}

	// Create and register client
	c := client.New(s.hub, conn, userID, username, s.logger)
	if s.persister != nil {
		c.SetPersister(s.persister)
	}
	if s.rateLimitPerSec > 0 {
		c.SetRateLimiter(ratelimit.NewTokenBucket(s.rateLimitBurst, s.rateLimitPerSec))
	}
	c.SetAnalytics(s.analytics)
	c.SetRoom(room) // empty room falls back to the client's default

	// Replay recent room history to this client before it joins the live
	// broadcast set, so the backlog is queued ahead of any live messages.
	if s.storage != nil {
		s.hydrateHistory(c)
	}

	s.hub.Register(c)
	c.Start()

	s.logger.Info("new websocket connection",
		slog.String("userID", userID),
		slog.String("username", username),
		slog.String("roomID", c.RoomID()),
		slog.String("clientID", c.ID()))
}

// authenticate resolves the connecting user's ID. When auth is enabled it
// requires a valid signed token (from the "token" query parameter or an
// "Authorization: Bearer" header) and returns false on failure. When auth is
// disabled it falls back to the userId query parameter for development.
func (s *Server) authenticate(r *http.Request) (string, bool) {
	if s.auth == nil {
		userID := r.URL.Query().Get("userId")
		if userID == "" {
			userID = "anonymous"
		}
		return userID, true
	}

	token := r.URL.Query().Get("token")
	if token == "" {
		if h := r.Header.Get("Authorization"); strings.HasPrefix(h, "Bearer ") {
			token = strings.TrimPrefix(h, "Bearer ")
		}
	}
	if token == "" {
		return "", false
	}

	userID, err := s.auth.Verify(token)
	if err != nil {
		s.logger.Warn("rejected websocket auth",
			slog.String("error", err.Error()))
		return "", false
	}
	return userID, true
}

// hydrateHistory queues the recent message history for the client's room onto
// the client's send buffer. The backlog stays within the send buffer size, so
// it is queued before the write pump starts and replayed in order ahead of any
// live messages. Storage errors degrade gracefully to an empty backlog.
func (s *Server) hydrateHistory(c *client.Client) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	msgs, err := s.storage.GetRecentMessages(ctx, c.RoomID(), defaultHistoryLimit)
	if err != nil {
		s.logger.Error("failed to load room history",
			slog.String("clientID", c.ID()),
			slog.String("roomID", c.RoomID()),
			slog.String("error", err.Error()))
		return
	}

	for _, m := range msgs {
		data, err := m.ToJSON()
		if err != nil {
			s.logger.Error("failed to marshal history message",
				slog.String("messageID", m.MessageID),
				slog.String("error", err.Error()))
			continue
		}
		c.Send(data)
	}

	s.logger.Debug("hydrated client with room history",
		slog.String("clientID", c.ID()),
		slog.String("roomID", c.RoomID()),
		slog.Int("count", len(msgs)))
}

func corsMiddleware(allowedOrigins []string, next http.Handler) http.Handler {
	allowAll := false
	allowed := make(map[string]bool, len(allowedOrigins))
	for _, o := range allowedOrigins {
		if o == "*" {
			allowAll = true
		}
		allowed[o] = true
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		switch {
		case allowAll:
			w.Header().Set("Access-Control-Allow-Origin", "*")
		case origin != "" && allowed[origin]:
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Add("Vary", "Origin")
		}
		w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
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
	mux.HandleFunc("GET /api/rooms/{id}/messages", s.handleRoomMessages)
	mux.HandleFunc("GET /api/users/{id}/messages", s.handleUserMessages)
	return corsMiddleware(s.allowedOrigins, mux)
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
	srv := NewServer(logger, repo, cfg)

	// Start hub
	go srv.hub.Run()

	// Start the persistence worker pool (nil when storage is unavailable).
	if srv.persister != nil {
		srv.persister.Start()
	}

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

	// Shutdown hub (stops clients, so no further messages are enqueued)
	srv.hub.Shutdown()

	// Drain any buffered messages before closing storage.
	if srv.persister != nil {
		srv.persister.Close()
	}

	// Release storage resources
	if srv.storage != nil {
		if err := srv.storage.Close(); err != nil {
			logger.Error("error closing storage", slog.String("error", err.Error()))
		}
	}

	logger.Info("server exited")
}
