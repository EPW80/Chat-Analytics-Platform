package main

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
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
	healthErr error

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
	return m.healthErr
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

// Voice briefings are an optional dependency: with BRIEFING_ENABLED unset, the
// same binary must have no service, no detector goroutine, and no routes.
func TestLivezAlwaysOK(t *testing.T) {
	repo := &mockRepo{healthErr: errors.New("dynamo down")}
	srv := testServer(repo)

	rec := httptest.NewRecorder()
	srv.setupRoutes().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/livez", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("livez expected 200 even when storage is down, got %d", rec.Code)
	}
	if !repo.healthCalled {
		t.Error("livez should still report storage status")
	}
}

func TestReadyzFailsWhenStorageUnavailable(t *testing.T) {
	repo := &mockRepo{healthErr: errors.New("dynamo down")}
	srv := testServer(repo)

	rec := httptest.NewRecorder()
	srv.setupRoutes().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/readyz", nil))
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("readyz expected 503 when storage is down, got %d", rec.Code)
	}

	var body healthResponse
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.Status != "not_ready" || body.Storage != "unavailable" {
		t.Errorf("unexpected body: %+v", body)
	}
}

func TestReadyzOKWhenStorageDisabled(t *testing.T) {
	srv := testServer(nil)

	rec := httptest.NewRecorder()
	srv.setupRoutes().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/readyz", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("readyz expected 200 when storage is disabled, got %d", rec.Code)
	}
}

func TestBriefingsDisabledByDefault(t *testing.T) {
	srv := testServer(nil)

	if srv.briefing != nil || srv.runner != nil || srv.briefingStore != nil {
		t.Errorf("briefing deps constructed while disabled: service=%v runner=%v store=%v",
			srv.briefing != nil, srv.runner != nil, srv.briefingStore != nil)
	}

	routes := srv.setupRoutes()
	for _, path := range []string{"/api/briefings", "/api/briefings/abc123/audio"} {
		rec := httptest.NewRecorder()
		routes.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		if rec.Code != http.StatusNotFound {
			t.Errorf("GET %s returned %d while briefings are disabled, want 404", path, rec.Code)
		}
	}
}

func TestBriefingsEnabledRegistersRoutes(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	srv := NewServer(logger, nil, &config.Config{
		AllowedOrigins:   []string{"*"},
		BriefingEnabled:  true,
		ElevenLabsKey:    "test-key",
		ElevenLabsVoice:  "test-voice",
		ElevenLabsModel:  "eleven_flash_v2_5",
		BriefingAudioDir: t.TempDir(),
	})

	if srv.briefing == nil || srv.runner == nil || srv.briefingStore == nil {
		t.Fatal("briefing deps not constructed while enabled")
	}

	rec := httptest.NewRecorder()
	srv.setupRoutes().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/briefings", nil))
	if rec.Code != http.StatusOK {
		t.Errorf("GET /api/briefings returned %d, want 200", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), `"briefings"`) {
		t.Errorf("unexpected body: %s", rec.Body)
	}
}
