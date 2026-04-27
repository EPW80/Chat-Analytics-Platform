package client

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/epw80/chat-analytics-platform/pkg/analytics"
	"github.com/epw80/chat-analytics-platform/pkg/message"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

const (
	// Time allowed to write a message to the peer
	writeWait = 10 * time.Second

	// Time allowed to read the next pong message from the peer
	pongWait = 60 * time.Second

	// Send pings to peer with this period (must be less than pongWait)
	pingPeriod = (pongWait * 9) / 10

	// Maximum message size allowed from peer
	maxMessageSize = 8192

	// Buffer size for the send channel
	sendBufferSize = 256
)

// Hub interface to avoid circular dependencies
type Hub interface {
	Register(any)
	Unregister(any)
	Broadcast([]byte)
}

// MessageRepository interface for message persistence
type MessageRepository interface {
	SaveMessage(ctx context.Context, msg *message.Message) error
}

// Client represents a WebSocket client connection
type Client struct {
	hub Hub

	// The websocket connection
	conn *websocket.Conn

	// Buffered channel of outbound messages
	send chan []byte

	// Client metadata
	id       string
	username string
	userID   string

	// Optional message storage (nil-safe)
	storage MessageRepository

	// Optional analytics tracker (nil-safe)
	analytics *analytics.Tracker

	// Configurable timing (defaults to package constants)
	pongWait   time.Duration
	pingPeriod time.Duration

	// Logger
	logger *slog.Logger
}

// New creates a new Client instance
func New(hub Hub, conn *websocket.Conn, userID, username string, logger *slog.Logger) *Client {
	return &Client{
		hub:        hub,
		conn:       conn,
		send:       make(chan []byte, sendBufferSize),
		id:         generateID(),
		username:   username,
		userID:     userID,
		storage:    nil, // Can be set later with SetStorage
		pongWait:   pongWait,
		pingPeriod: pingPeriod,
		logger:     logger,
	}
}

// SetStorage sets the message repository for this client (optional)
func (c *Client) SetStorage(storage MessageRepository) {
	c.storage = storage
}

// SetAnalytics attaches an analytics tracker to this client (optional)
func (c *Client) SetAnalytics(t *analytics.Tracker) {
	c.analytics = t
}

// Username returns the display name of the connected user.
// Implements the hub.Client interface.
func (c *Client) Username() string {
	return c.username
}

// readPump pumps messages from the WebSocket connection to the hub
//
// The application runs readPump in a per-connection goroutine. The application
// ensures that there is at most one reader on a connection by executing all
// reads from this goroutine.
func (c *Client) readPump() {
	defer func() {
		c.hub.Unregister(c)
		c.conn.Close()
	}()

	c.conn.SetReadLimit(maxMessageSize)
	c.conn.SetReadDeadline(time.Now().Add(c.pongWait))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(c.pongWait))
		return nil
	})

	for {
		_, data, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				c.logger.Error("websocket read error",
					slog.String("clientID", c.id),
					slog.String("error", err.Error()))
			}
			break
		}

		// Parse and validate message
		msg, err := message.FromJSON(data)
		if err != nil {
			c.logger.Warn("invalid message format",
				slog.String("clientID", c.id),
				slog.String("error", err.Error()))
			continue
		}

		// Set client metadata (server overrides any client-supplied values)
		msg.UserID = c.userID
		msg.Username = c.username

		// Enrich with server-side fields the client doesn't set
		if msg.MessageID == "" {
			msg.MessageID = uuid.New().String()
		}
		if msg.RoomID == "" {
			msg.RoomID = "global"
		}
		if msg.Timestamp.IsZero() {
			msg.Timestamp = time.Now().UTC()
		}

		// Validate
		if err := msg.Validate(); err != nil {
			c.logger.Warn("message validation failed",
				slog.String("clientID", c.id),
				slog.String("error", err.Error()))
			continue
		}

		// Non-blocking persistence (if storage is configured)
		if c.storage != nil {
			go func(msgCopy *message.Message) {
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()

				if err := c.storage.SaveMessage(ctx, msgCopy); err != nil {
					c.logger.Error("failed to persist message",
						slog.String("clientID", c.id),
						slog.String("messageId", msgCopy.MessageID),
						slog.String("error", err.Error()))
					// Don't block message delivery on storage failure
				}
			}(msg)
		}

		if c.analytics != nil {
			c.analytics.TrackMessage(msg)
		}

		// Convert back to JSON and broadcast
		jsonData, err := msg.ToJSON()
		if err != nil {
			c.logger.Error("failed to marshal message",
				slog.String("clientID", c.id),
				slog.String("error", err.Error()))
			continue
		}

		c.hub.Broadcast(jsonData)
	}
}

// writePump pumps messages from the hub to the WebSocket connection
//
// A goroutine running writePump is started for each connection. The
// application ensures that there is at most one writer to a connection by
// executing all writes from this goroutine.
func (c *Client) writePump() {
	ticker := time.NewTicker(c.pingPeriod)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				// Hub closed the channel
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			w, err := c.conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}
			w.Write(message)

			// Add queued messages to the current websocket message
			n := len(c.send)
			for i := 0; i < n; i++ {
				w.Write([]byte{'\n'})
				w.Write(<-c.send)
			}

			if err := w.Close(); err != nil {
				return
			}

		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// Start begins the client's read and write pumps
func (c *Client) Start() {
	go c.writePump()
	go c.readPump()
}

// Send queues a message to be sent to the client
// Implements the hub.Client interface
func (c *Client) Send(data []byte) {
	select {
	case c.send <- data:
	default:
		// Channel is full, log and skip
		c.logger.Warn("client send buffer full, dropping message",
			slog.String("clientID", c.id))
	}
}

// Close closes the client's send channel
// Implements the hub.Client interface
func (c *Client) Close() {
	close(c.send)
}

// ID returns the client's unique identifier
// Implements the hub.Client interface
func (c *Client) ID() string {
	return c.id
}

// generateID generates a unique client ID
func generateID() string {
	return fmt.Sprintf("client-%d", time.Now().UnixNano())
}
