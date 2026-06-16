package main

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/epw80/chat-analytics-platform/pkg/config"
	"github.com/epw80/chat-analytics-platform/pkg/message"
)

// mockRepo implements storage.MessageRepository for handler tests.
type mockRepo struct {
	recent    []*message.Message
	byUser    []*message.Message
	recentErr error
	userErr   error

	lastRoomID   string
	lastUserID   string
	lastRoomLim  int
	lastUserLim  int
	healthCalled bool
}

func (m *mockRepo) SaveMessage(ctx context.Context, msg *message.Message) error { return nil }

func (m *mockRepo) BatchSaveMessages(ctx context.Context, msgs []*message.Message) error { return nil }

func (m *mockRepo) GetRecentMessages(ctx context.Context, roomID string, limit int) ([]*message.Message, error) {
	m.lastRoomID = roomID
	m.lastRoomLim = limit
	return m.recent, m.recentErr
}

func (m *mockRepo) GetMessagesByUser(ctx context.Context, userID string, limit int) ([]*message.Message, error) {
	m.lastUserID = userID
	m.lastUserLim = limit
	return m.byUser, m.userErr
}

func (m *mockRepo) HealthCheck(ctx context.Context) error {
	m.healthCalled = true
	return nil
}

func (m *mockRepo) Close() error { return nil }

func testServer(repo *mockRepo) *Server {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	cfg := &config.Config{AllowedOrigins: []string{"*"}}
	if repo == nil {
		return NewServer(logger, nil, cfg)
	}
	return NewServer(logger, repo, cfg)
}

func TestHandleRoomMessages(t *testing.T) {
	repo := &mockRepo{recent: []*message.Message{
		{MessageID: "m1", RoomID: "lobby", Content: "hello"},
		{MessageID: "m2", RoomID: "lobby", Content: "world"},
	}}
	srv := testServer(repo)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/rooms/lobby/messages?limit=10", nil)
	srv.setupRoutes().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if repo.lastRoomID != "lobby" {
		t.Errorf("expected roomID 'lobby', got %q", repo.lastRoomID)
	}
	if repo.lastRoomLim != 10 {
		t.Errorf("expected limit 10, got %d", repo.lastRoomLim)
	}

	var resp messagesResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.RoomID != "lobby" || resp.Count != 2 || len(resp.Messages) != 2 {
		t.Errorf("unexpected response: %+v", resp)
	}
}

func TestHandleUserMessages(t *testing.T) {
	repo := &mockRepo{byUser: []*message.Message{{MessageID: "m1", UserID: "u1"}}}
	srv := testServer(repo)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/users/u1/messages", nil)
	srv.setupRoutes().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if repo.lastUserID != "u1" {
		t.Errorf("expected userID 'u1', got %q", repo.lastUserID)
	}
	// No limit param: should fall back to the default.
	if repo.lastUserLim != defaultHistoryLimit {
		t.Errorf("expected default limit %d, got %d", defaultHistoryLimit, repo.lastUserLim)
	}
}

func TestHandleRoomMessages_LimitClamped(t *testing.T) {
	repo := &mockRepo{}
	srv := testServer(repo)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/rooms/lobby/messages?limit=9999", nil)
	srv.setupRoutes().ServeHTTP(rec, req)

	if repo.lastRoomLim != maxHistoryLimit {
		t.Errorf("expected limit clamped to %d, got %d", maxHistoryLimit, repo.lastRoomLim)
	}
}

func TestHandleRoomMessages_StorageError(t *testing.T) {
	repo := &mockRepo{recentErr: errors.New("dynamo down")}
	srv := testServer(repo)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/rooms/lobby/messages", nil)
	srv.setupRoutes().ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", rec.Code)
	}
}

func TestHandleRoomMessages_NoStorage(t *testing.T) {
	srv := testServer(nil)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/rooms/lobby/messages", nil)
	srv.setupRoutes().ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("expected 503 when storage is unavailable, got %d", rec.Code)
	}
}

func TestParseLimit(t *testing.T) {
	cases := []struct {
		query string
		want  int
	}{
		{"", defaultHistoryLimit},
		{"limit=25", 25},
		{"limit=0", defaultHistoryLimit},
		{"limit=-5", defaultHistoryLimit},
		{"limit=abc", defaultHistoryLimit},
		{"limit=99999", maxHistoryLimit},
	}
	for _, tc := range cases {
		req := httptest.NewRequest(http.MethodGet, "/x?"+tc.query, nil)
		if got := parseLimit(req); got != tc.want {
			t.Errorf("parseLimit(%q) = %d, want %d", tc.query, got, tc.want)
		}
	}
}
