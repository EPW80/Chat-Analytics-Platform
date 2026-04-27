package hub

import (
	"log/slog"
	"sync"
	"time"

	"github.com/epw80/chat-analytics-platform/pkg/analytics"
)

// Client represents a connected WebSocket client
// This is an interface to avoid circular dependencies between hub and client packages
type Client interface {
	Send([]byte)
	Close()
	ID() string
	Username() string
}

// Hub maintains active clients and broadcasts messages
type Hub struct {
	// Registered clients
	clients map[Client]bool

	// Inbound messages from clients
	broadcast chan []byte

	// Register requests from clients
	register chan Client

	// Unregister requests from clients
	unregister chan Client

	// Mutex for thread-safe client map access
	mu sync.RWMutex

	// Logger
	logger *slog.Logger

	// Shutdown signal
	done chan struct{}

	// Optional analytics tracker (nil-safe)
	analytics *analytics.Tracker
}

// SetAnalytics attaches an analytics tracker to the hub (optional).
func (h *Hub) SetAnalytics(t *analytics.Tracker) {
	h.analytics = t
}

// New creates a new Hub instance
func New(logger *slog.Logger) *Hub {
	return &Hub{
		broadcast:  make(chan []byte, 256),
		register:   make(chan Client),
		unregister: make(chan Client),
		clients:    make(map[Client]bool),
		logger:     logger,
		done:       make(chan struct{}),
	}
}

// Run starts the hub's main event loop
// This should be called in a goroutine
func (h *Hub) Run() {
	h.logger.Info("hub started")

	for {
		select {
		case client := <-h.register:
			h.mu.Lock()
			h.clients[client] = true
			h.mu.Unlock()

			if h.analytics != nil {
				h.analytics.TrackUserJoin(client.ID(), client.Username())
			}

			count := h.ClientCount()
			h.logger.Info("client registered",
				slog.String("clientID", client.ID()),
				slog.Int("totalClients", count))

		case client := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				client.Close()
			}
			h.mu.Unlock()

			if h.analytics != nil {
				h.analytics.TrackUserLeave(client.ID())
			}

			count := h.ClientCount()
			h.logger.Info("client unregistered",
				slog.String("clientID", client.ID()),
				slog.Int("totalClients", count))

		case message := <-h.broadcast:
			start := time.Now()
			h.mu.RLock()
			for client := range h.clients {
				// Non-blocking send
				// If client's send buffer is full, skip it
				client.Send(message)
			}
			h.mu.RUnlock()
			if h.analytics != nil {
				h.analytics.TrackBroadcastLatency(time.Since(start))
			}

		case <-h.done:
			h.logger.Info("hub shutting down")
			h.mu.Lock()
			for client := range h.clients {
				client.Close()
			}
			h.clients = make(map[Client]bool)
			h.mu.Unlock()
			return
		}
	}
}

// Register adds a client to the hub
func (h *Hub) Register(client any) {
	if c, ok := client.(Client); ok {
		h.register <- c
	}
}

// Unregister removes a client from the hub
func (h *Hub) Unregister(client any) {
	if c, ok := client.(Client); ok {
		h.unregister <- c
	}
}

// Broadcast sends a message to all clients
func (h *Hub) Broadcast(message []byte) {
	h.broadcast <- message
}

// ClientCount returns the number of connected clients
func (h *Hub) ClientCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}

// Shutdown gracefully shuts down the hub
func (h *Hub) Shutdown() {
	close(h.done)
}
