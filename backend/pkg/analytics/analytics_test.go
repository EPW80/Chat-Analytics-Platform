package analytics

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/epw80/chat-analytics-platform/pkg/message"
)

func TestTracker_TrackMessage(t *testing.T) {
	tr := New()
	msg := &message.Message{Type: message.TypeChat, Content: "hi"}

	tr.TrackMessage(msg)
	tr.TrackMessage(msg)

	m := tr.GetMetrics()
	if m.TotalMessages != 2 {
		t.Errorf("expected 2 total messages, got %d", m.TotalMessages)
	}
}

func TestTracker_TrackUserJoin(t *testing.T) {
	tr := New()

	tr.TrackUserJoin("u1", "Alice")
	tr.TrackUserJoin("u2", "Bob")

	m := tr.GetMetrics()
	if m.ActiveUsers != 2 {
		t.Errorf("expected 2 active users, got %d", m.ActiveUsers)
	}
	if m.PeakConnections != 2 {
		t.Errorf("expected peak 2, got %d", m.PeakConnections)
	}
	if len(m.ActiveUserDetails) != 2 {
		t.Errorf("expected 2 user details, got %d", len(m.ActiveUserDetails))
	}
}

func TestTracker_TrackUserLeave(t *testing.T) {
	tr := New()
	tr.TrackUserJoin("u1", "Alice")
	tr.TrackUserJoin("u2", "Bob")
	tr.TrackUserLeave("u1")

	m := tr.GetMetrics()
	if m.ActiveUsers != 1 {
		t.Errorf("expected 1 active user, got %d", m.ActiveUsers)
	}
	// Peak should remain at 2 even after a leave.
	if m.PeakConnections != 2 {
		t.Errorf("expected peak still 2, got %d", m.PeakConnections)
	}
	if len(m.ActiveUserDetails) != 1 {
		t.Errorf("expected 1 user detail, got %d", len(m.ActiveUserDetails))
	}
	if m.ActiveUserDetails[0].UserID != "u2" {
		t.Errorf("expected remaining user u2, got %s", m.ActiveUserDetails[0].UserID)
	}
}

func TestTracker_TrackBroadcastLatency(t *testing.T) {
	tr := New()
	tr.TrackBroadcastLatency(1 * time.Millisecond)
	tr.TrackBroadcastLatency(5 * time.Millisecond)
	tr.TrackBroadcastLatency(10 * time.Millisecond)

	m := tr.GetMetrics()
	if m.LatencyP50Ms <= 0 {
		t.Errorf("expected positive P50 latency, got %f", m.LatencyP50Ms)
	}
	if m.LatencyP99Ms < m.LatencyP50Ms {
		t.Errorf("P99 (%f) should be >= P50 (%f)", m.LatencyP99Ms, m.LatencyP50Ms)
	}
}

func TestTracker_GetMetrics_Uptime(t *testing.T) {
	tr := New()
	time.Sleep(10 * time.Millisecond)
	m := tr.GetMetrics()
	if m.UptimeSeconds < 0 {
		t.Error("uptime should not be negative")
	}
}

func TestTracker_MessagesPerMinute_Length(t *testing.T) {
	tr := New()
	m := tr.GetMetrics()
	if len(m.MessagesPerMinute) != 15 {
		t.Errorf("expected 15-slot window, got %d", len(m.MessagesPerMinute))
	}
}

func TestTracker_Concurrent(t *testing.T) {
	tr := New()
	var wg sync.WaitGroup
	msg := &message.Message{Type: message.TypeChat, Content: "concurrent"}

	for i := range 50 {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			userID := "user" + string(rune('A'+id%26))
			tr.TrackUserJoin(userID, userID)
			tr.TrackMessage(msg)
			tr.TrackBroadcastLatency(time.Duration(id) * time.Microsecond)
			tr.GetMetrics()
			tr.TrackUserLeave(userID)
		}(i)
	}
	wg.Wait()

	m := tr.GetMetrics()
	if m.TotalMessages != 50 {
		t.Errorf("expected 50 total messages, got %d", m.TotalMessages)
	}
}

func TestHandler_JSON(t *testing.T) {
	tr := New()
	tr.TrackUserJoin("u1", "Alice")
	tr.TrackMessage(&message.Message{Type: message.TypeChat, Content: "test"})

	h := NewHandler(tr)
	req := httptest.NewRequest(http.MethodGet, "/api/analytics", nil)
	rec := httptest.NewRecorder()
	h(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("expected application/json Content-Type, got %s", ct)
	}

	var m Metrics
	if err := json.NewDecoder(rec.Body).Decode(&m); err != nil {
		t.Fatalf("response is not valid JSON: %v", err)
	}
	if m.TotalMessages != 1 {
		t.Errorf("expected 1 total message in JSON, got %d", m.TotalMessages)
	}
}
