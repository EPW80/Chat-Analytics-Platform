package hub

import (
	"log/slog"
	"os"
	"sync"
	"testing"
	"time"
)

// mockClient implements the Client interface for testing
type mockClient struct {
	id       string
	messages [][]byte
	closed   bool
	mu       sync.Mutex
}

func (m *mockClient) Send(data []byte) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.closed {
		m.messages = append(m.messages, data)
	}
}

func (m *mockClient) Close() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.closed = true
}

func (m *mockClient) ID() string {
	return m.id
}

func (m *mockClient) MessageCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.messages)
}

func (m *mockClient) IsClosed() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.closed
}

func newMockClient(id string) *mockClient {
	return &mockClient{
		id:       id,
		messages: make([][]byte, 0),
		closed:   false,
	}
}

func newTestHub() *Hub {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelError, // Reduce noise in tests
	}))
	return New(logger)
}

func TestHub_RegisterUnregister(t *testing.T) {
	hub := newTestHub()
	go hub.Run()
	defer hub.Shutdown()

	client := newMockClient("test1")

	// Register
	hub.Register(client)
	time.Sleep(10 * time.Millisecond) // Allow processing

	if count := hub.ClientCount(); count != 1 {
		t.Errorf("expected 1 client, got %d", count)
	}

	// Unregister
	hub.Unregister(client)
	time.Sleep(10 * time.Millisecond)

	if count := hub.ClientCount(); count != 0 {
		t.Errorf("expected 0 clients, got %d", count)
	}

	if !client.IsClosed() {
		t.Error("client should be closed after unregister")
	}
}

func TestHub_Broadcast(t *testing.T) {
	hub := newTestHub()
	go hub.Run()
	defer hub.Shutdown()

	// Register 3 clients
	clients := []*mockClient{
		newMockClient("client1"),
		newMockClient("client2"),
		newMockClient("client3"),
	}

	for _, c := range clients {
		hub.Register(c)
	}
	time.Sleep(10 * time.Millisecond)

	// Broadcast message
	message := []byte("test message")
	hub.Broadcast(message)
	time.Sleep(10 * time.Millisecond)

	// Verify all clients received message
	for _, c := range clients {
		if count := c.MessageCount(); count != 1 {
			t.Errorf("client %s: expected 1 message, got %d", c.ID(), count)
		}
	}
}

func TestHub_MultipleBroadcasts(t *testing.T) {
	hub := newTestHub()
	go hub.Run()
	defer hub.Shutdown()

	client := newMockClient("test1")
	hub.Register(client)
	time.Sleep(10 * time.Millisecond)

	// Send multiple messages
	messages := []string{"msg1", "msg2", "msg3"}
	for _, msg := range messages {
		hub.Broadcast([]byte(msg))
	}
	time.Sleep(10 * time.Millisecond)

	if count := client.MessageCount(); count != 3 {
		t.Errorf("expected 3 messages, got %d", count)
	}
}

func TestHub_ConcurrentOperations(t *testing.T) {
	hub := newTestHub()
	go hub.Run()
	defer hub.Shutdown()

	const numClients = 50
	const numMessages = 100

	var wg sync.WaitGroup
	clients := make([]*mockClient, numClients)

	// Register clients concurrently
	wg.Add(numClients)
	for i := 0; i < numClients; i++ {
		go func(idx int) {
			defer wg.Done()
			clients[idx] = newMockClient(string(rune('A' + idx%26)))
			hub.Register(clients[idx])
		}(i)
	}
	wg.Wait()
	time.Sleep(50 * time.Millisecond)

	if count := hub.ClientCount(); count != numClients {
		t.Errorf("expected %d clients, got %d", numClients, count)
	}

	// Broadcast messages concurrently
	wg.Add(numMessages)
	for i := 0; i < numMessages; i++ {
		go func(idx int) {
			defer wg.Done()
			hub.Broadcast([]byte("message"))
		}(i)
	}
	wg.Wait()
	time.Sleep(50 * time.Millisecond)

	// Verify all clients received all messages
	for _, c := range clients {
		if count := c.MessageCount(); count != numMessages {
			t.Errorf("client %s: expected %d messages, got %d", c.ID(), numMessages, count)
		}
	}

	// Unregister clients concurrently
	wg.Add(numClients)
	for i := 0; i < numClients; i++ {
		go func(idx int) {
			defer wg.Done()
			hub.Unregister(clients[idx])
		}(i)
	}
	wg.Wait()
	time.Sleep(50 * time.Millisecond)

	if count := hub.ClientCount(); count != 0 {
		t.Errorf("expected 0 clients, got %d", count)
	}
}

func TestHub_Shutdown(t *testing.T) {
	hub := newTestHub()
	go hub.Run()

	clients := []*mockClient{
		newMockClient("client1"),
		newMockClient("client2"),
	}

	for _, c := range clients {
		hub.Register(c)
	}
	time.Sleep(10 * time.Millisecond)

	// Shutdown hub
	hub.Shutdown()
	time.Sleep(10 * time.Millisecond)

	// Verify all clients are closed
	for _, c := range clients {
		if !c.IsClosed() {
			t.Errorf("client %s should be closed after shutdown", c.ID())
		}
	}

	if count := hub.ClientCount(); count != 0 {
		t.Errorf("expected 0 clients after shutdown, got %d", count)
	}
}

func TestHub_UnregisterNonExistentClient(t *testing.T) {
	hub := newTestHub()
	go hub.Run()
	defer hub.Shutdown()

	client := newMockClient("test1")

	// Unregister without registering first
	hub.Unregister(client)
	time.Sleep(10 * time.Millisecond)

	// Should not panic
	if count := hub.ClientCount(); count != 0 {
		t.Errorf("expected 0 clients, got %d", count)
	}
}

func TestHub_DuplicateRegister(t *testing.T) {
	hub := newTestHub()
	go hub.Run()
	defer hub.Shutdown()

	client := newMockClient("test1")

	// Register twice
	hub.Register(client)
	hub.Register(client)
	time.Sleep(10 * time.Millisecond)

	// Client count should still be 1 (map deduplicates)
	if count := hub.ClientCount(); count != 1 {
		t.Errorf("expected 1 client, got %d", count)
	}
}

// Test for race conditions - run with: go test -race
func TestHub_RaceConditions(t *testing.T) {
	hub := newTestHub()
	go hub.Run()
	defer hub.Shutdown()

	var wg sync.WaitGroup
	const numGoroutines = 100

	// Concurrent register/unregister/broadcast operations
	wg.Add(numGoroutines * 3)

	for i := 0; i < numGoroutines; i++ {
		// Register and unregister
		go func(idx int) {
			defer wg.Done()
			c := newMockClient(string(rune('A' + idx%26)))
			hub.Register(c)
			time.Sleep(time.Millisecond)
			hub.Unregister(c)
		}(i)

		// Broadcast
		go func() {
			defer wg.Done()
			hub.Broadcast([]byte("test"))
		}()

		// Read count
		go func() {
			defer wg.Done()
			_ = hub.ClientCount()
		}()
	}

	wg.Wait()
}

func TestHub_BroadcastToZeroClients(t *testing.T) {
	hub := newTestHub()
	go hub.Run()
	defer hub.Shutdown()

	// Broadcast to empty hub should not panic
	hub.Broadcast([]byte("test message"))
	time.Sleep(10 * time.Millisecond)

	// Should complete without error
}

func BenchmarkHub_Broadcast(b *testing.B) {
	hub := newTestHub()
	go hub.Run()
	defer hub.Shutdown()

	// Register 100 clients
	for i := 0; i < 100; i++ {
		hub.Register(newMockClient(string(rune('A' + i%26))))
	}
	time.Sleep(50 * time.Millisecond)

	message := []byte("benchmark message")
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		hub.Broadcast(message)
	}
}

func BenchmarkHub_RegisterUnregister(b *testing.B) {
	hub := newTestHub()
	go hub.Run()
	defer hub.Shutdown()

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		client := newMockClient("bench")
		hub.Register(client)
		hub.Unregister(client)
	}
}
