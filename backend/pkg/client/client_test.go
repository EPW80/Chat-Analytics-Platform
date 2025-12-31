package client

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/epw80/chat-analytics-platform/pkg/message"
	"github.com/gorilla/websocket"
)

// mockHub implements the Hub interface for testing
type mockHub struct {
	mu           sync.Mutex
	registered   []*Client
	unregistered []*Client
	broadcasts   [][]byte
}

func (m *mockHub) Register(c any) {
	if client, ok := c.(*Client); ok {
		m.mu.Lock()
		m.registered = append(m.registered, client)
		m.mu.Unlock()
	}
}

func (m *mockHub) Unregister(c any) {
	if client, ok := c.(*Client); ok {
		m.mu.Lock()
		m.unregistered = append(m.unregistered, client)
		m.mu.Unlock()
	}
}

func (m *mockHub) Broadcast(data []byte) {
	m.mu.Lock()
	m.broadcasts = append(m.broadcasts, data)
	m.mu.Unlock()
}

func (m *mockHub) BroadcastCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.broadcasts)
}

func (m *mockHub) GetBroadcast(index int) []byte {
	m.mu.Lock()
	defer m.mu.Unlock()
	if index < len(m.broadcasts) {
		return m.broadcasts[index]
	}
	return nil
}

func (m *mockHub) RegisteredCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.registered)
}

func (m *mockHub) UnregisteredCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.unregistered)
}

func newMockHub() *mockHub {
	return &mockHub{
		registered:   make([]*Client, 0),
		unregistered: make([]*Client, 0),
		broadcasts:   make([][]byte, 0),
	}
}

func newTestLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelError,
	}))
}

func TestClient_SendReceive(t *testing.T) {
	hub := newMockHub()
	logger := newTestLogger()

	// Create test server
	upgrader := websocket.Upgrader{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Fatalf("upgrade error: %v", err)
		}
		defer conn.Close()

		client := New(hub, conn, "user123", "alice", logger)
		client.Start()

		// Wait for client operations
		time.Sleep(100 * time.Millisecond)
	}))
	defer server.Close()

	// Connect client
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
	ws, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("dial error: %v", err)
	}
	defer ws.Close()

	// Send message
	msg := message.NewChatMessage("user123", "alice", "Hello")
	data, _ := msg.ToJSON()
	err = ws.WriteMessage(websocket.TextMessage, data)
	if err != nil {
		t.Fatalf("write error: %v", err)
	}

	// Wait for processing
	time.Sleep(50 * time.Millisecond)

	// Verify hub received broadcast
	if hub.BroadcastCount() != 1 {
		t.Errorf("expected 1 broadcast, got %d", hub.BroadcastCount())
	}
}

func TestClient_InvalidMessage(t *testing.T) {
	hub := newMockHub()
	logger := newTestLogger()

	upgrader := websocket.Upgrader{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Fatalf("upgrade error: %v", err)
		}
		defer conn.Close()

		client := New(hub, conn, "user123", "alice", logger)
		client.Start()

		time.Sleep(100 * time.Millisecond)
	}))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
	ws, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("dial error: %v", err)
	}
	defer ws.Close()

	// Send invalid JSON
	err = ws.WriteMessage(websocket.TextMessage, []byte("invalid json"))
	if err != nil {
		t.Fatalf("write error: %v", err)
	}

	time.Sleep(50 * time.Millisecond)

	// Verify no broadcast
	if hub.BroadcastCount() != 0 {
		t.Errorf("expected 0 broadcasts for invalid message, got %d", hub.BroadcastCount())
	}
}

func TestClient_ValidationFailed(t *testing.T) {
	hub := newMockHub()
	logger := newTestLogger()

	upgrader := websocket.Upgrader{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Fatalf("upgrade error: %v", err)
		}
		defer conn.Close()

		client := New(hub, conn, "user123", "alice", logger)
		client.Start()

		time.Sleep(100 * time.Millisecond)
	}))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
	ws, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("dial error: %v", err)
	}
	defer ws.Close()

	// Send message with empty content (validation will fail)
	msg := &message.Message{
		Type:     message.TypeChat,
		Username: "alice",
		Content:  "", // Empty content for chat message
	}
	data, _ := msg.ToJSON()
	err = ws.WriteMessage(websocket.TextMessage, data)
	if err != nil {
		t.Fatalf("write error: %v", err)
	}

	time.Sleep(50 * time.Millisecond)

	// Verify no broadcast
	if hub.BroadcastCount() != 0 {
		t.Errorf("expected 0 broadcasts for invalid message, got %d", hub.BroadcastCount())
	}
}

func TestClient_SendBufferFull(t *testing.T) {
	hub := newMockHub()
	logger := newTestLogger()

	// Create a mock connection that never reads
	upgrader := websocket.Upgrader{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Fatalf("upgrade error: %v", err)
		}
		defer conn.Close()

		client := New(hub, conn, "user123", "alice", logger)

		// Fill the send buffer
		for i := 0; i < sendBufferSize+10; i++ {
			client.Send([]byte("test message"))
		}

		// Should not panic or block
		time.Sleep(100 * time.Millisecond)
	}))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
	ws, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("dial error: %v", err)
	}
	defer ws.Close()

	time.Sleep(200 * time.Millisecond)
}

func TestClient_PingPong(t *testing.T) {
	hub := newMockHub()
	logger := newTestLogger()

	upgrader := websocket.Upgrader{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Fatalf("upgrade error: %v", err)
		}
		defer conn.Close()

		client := New(hub, conn, "user123", "alice", logger)
		client.Start()

		// Keep connection alive for multiple ping cycles
		time.Sleep(pingPeriod * 3)
	}))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
	ws, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("dial error: %v", err)
	}
	defer ws.Close()

	// Set up ping handler (client sends pings, we expect to receive them)
	pingReceived := 0
	ws.SetPingHandler(func(string) error {
		pingReceived++
		// Send pong in response
		ws.WriteControl(websocket.PongMessage, []byte{}, time.Now().Add(writeWait))
		return nil
	})

	// Read messages (including pings)
	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			_, _, err := ws.ReadMessage()
			if err != nil {
				return
			}
		}
	}()

	time.Sleep(pingPeriod * 3)

	// Should have received at least 2 pings
	if pingReceived < 2 {
		t.Errorf("expected at least 2 pings, got %d", pingReceived)
	}

	ws.Close()
	<-done
}

func TestClient_Disconnect(t *testing.T) {
	hub := newMockHub()
	logger := newTestLogger()

	upgrader := websocket.Upgrader{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Fatalf("upgrade error: %v", err)
		}
		defer conn.Close()

		client := New(hub, conn, "user123", "alice", logger)
		hub.Register(client)
		client.Start()

		// Wait for disconnect
		time.Sleep(200 * time.Millisecond)
	}))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
	ws, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("dial error: %v", err)
	}

	time.Sleep(50 * time.Millisecond)

	// Verify client was registered
	if hub.RegisteredCount() != 1 {
		t.Errorf("expected 1 registered client, got %d", hub.RegisteredCount())
	}

	// Close connection
	ws.Close()
	time.Sleep(100 * time.Millisecond)

	// Verify client was unregistered
	if hub.UnregisteredCount() != 1 {
		t.Errorf("expected 1 unregistered client, got %d", hub.UnregisteredCount())
	}
}

func TestClient_ID(t *testing.T) {
	hub := newMockHub()
	logger := newTestLogger()

	upgrader := websocket.Upgrader{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Fatalf("upgrade error: %v", err)
		}
		defer conn.Close()

		client := New(hub, conn, "user123", "alice", logger)

		// Verify ID is generated
		if client.ID() == "" {
			t.Error("client ID should not be empty")
		}

		// Verify ID is unique
		client2 := New(hub, conn, "user456", "bob", logger)
		if client.ID() == client2.ID() {
			t.Error("client IDs should be unique")
		}

		time.Sleep(50 * time.Millisecond)
	}))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
	ws, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("dial error: %v", err)
	}
	defer ws.Close()

	time.Sleep(100 * time.Millisecond)
}

func TestClient_MetadataOverride(t *testing.T) {
	hub := newMockHub()
	logger := newTestLogger()

	upgrader := websocket.Upgrader{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Fatalf("upgrade error: %v", err)
		}
		defer conn.Close()

		client := New(hub, conn, "actualUser", "ActualName", logger)
		client.Start()

		time.Sleep(100 * time.Millisecond)
	}))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
	ws, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("dial error: %v", err)
	}
	defer ws.Close()

	// Send message with different userID/username (should be overridden)
	msg := &message.Message{
		Type:     message.TypeChat,
		UserID:   "fakeUser",
		Username: "FakeName",
		Content:  "Hello",
	}
	data, _ := msg.ToJSON()
	err = ws.WriteMessage(websocket.TextMessage, data)
	if err != nil {
		t.Fatalf("write error: %v", err)
	}

	time.Sleep(50 * time.Millisecond)

	// Verify broadcast happened
	if hub.BroadcastCount() != 1 {
		t.Fatalf("expected 1 broadcast, got %d", hub.BroadcastCount())
	}

	// Parse broadcast message
	broadcastMsg, err := message.FromJSON(hub.GetBroadcast(0))
	if err != nil {
		t.Fatalf("failed to parse broadcast: %v", err)
	}

	// Verify metadata was overridden with actual client info
	if broadcastMsg.UserID != "actualUser" {
		t.Errorf("expected userID 'actualUser', got '%s'", broadcastMsg.UserID)
	}
	if broadcastMsg.Username != "ActualName" {
		t.Errorf("expected username 'ActualName', got '%s'", broadcastMsg.Username)
	}
}
