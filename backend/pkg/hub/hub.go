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
	UserID() string
	Username() string
	RoomID() string
}

// broadcastRequest carries a message destined for a single room.
type broadcastRequest struct {
	roomID string
	data   []byte
}

// Hub maintains active clients grouped by room and broadcasts each message
// only to the clients in its originating room.
type Hub struct {
	// Registered clients grouped by room ID
	rooms map[string]map[Client]bool

	// Inbound messages from clients, tagged with their destination room
	broadcast chan broadcastRequest

	// Register requests from clients
	register chan Client

	// Unregister requests from clients
	unregister chan Client

	// Mutex for thread-safe room map access
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
		rooms:      make(map[string]map[Client]bool),
		broadcast:  make(chan broadcastRequest, 256),
		register:   make(chan Client),
		unregister: make(chan Client),
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
			room := client.RoomID()
			h.mu.Lock()
			members := h.rooms[room]
			if members == nil {
				members = make(map[Client]bool)
				h.rooms[room] = members
			}
			_, existed := members[client]
			members[client] = true
			h.mu.Unlock()

			if !existed && h.analytics != nil {
				h.analytics.TrackConnect(client.ID(), client.UserID(), client.Username())
			}

			count := h.ClientCount()
			h.logger.Info("client registered",
				slog.String("clientID", client.ID()),
				slog.String("roomID", room),
				slog.Int("totalClients", count))

		case client := <-h.unregister:
			room := client.RoomID()
			h.mu.Lock()
			removed := false
			if members, ok := h.rooms[room]; ok {
				if _, ok := members[client]; ok {
					delete(members, client)
					client.Close()
					removed = true
					if len(members) == 0 {
						delete(h.rooms, room)
					}
				}
			}
			h.mu.Unlock()

			if removed && h.analytics != nil {
				h.analytics.TrackDisconnect(client.ID(), client.UserID())
			}

			count := h.ClientCount()
			h.logger.Info("client unregistered",
				slog.String("clientID", client.ID()),
				slog.String("roomID", room),
				slog.Int("totalClients", count))

		case req := <-h.broadcast:
			start := time.Now()
			h.mu.RLock()
			for client := range h.rooms[req.roomID] {
				// Non-blocking send
				// If client's send buffer is full, skip it
				client.Send(req.data)
			}
			h.mu.RUnlock()
			if h.analytics != nil {
				h.analytics.TrackBroadcastLatency(time.Since(start))
			}

		case <-h.done:
			h.logger.Info("hub shutting down")
			h.mu.Lock()
			for _, members := range h.rooms {
				for client := range members {
					client.Close()
				}
			}
			h.rooms = make(map[string]map[Client]bool)
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

// Broadcast sends a message to all clients in the given room.
func (h *Hub) Broadcast(roomID string, message []byte) {
	h.broadcast <- broadcastRequest{roomID: roomID, data: message}
}

// ClientCount returns the total number of connected clients across all rooms.
func (h *Hub) ClientCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	total := 0
	for _, members := range h.rooms {
		total += len(members)
	}
	return total
}

// RoomCount returns the number of rooms with at least one connected client.
func (h *Hub) RoomCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.rooms)
}

// Shutdown gracefully shuts down the hub
func (h *Hub) Shutdown() {
	close(h.done)
}
