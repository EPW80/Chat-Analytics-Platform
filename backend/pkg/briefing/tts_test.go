package briefing

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/epw80/chat-analytics-platform/pkg/analytics"
)

// quietLogger keeps test output readable; the Service logs on every degraded path.
func quietLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// The Mock lets us assert the cache contract without any network.
func TestMock_RecordsCalls(t *testing.T) {
	m := &Mock{}
	if _, err := m.Synthesize(context.Background(), "hello"); err != nil {
		t.Fatal(err)
	}
	if len(m.Calls) != 1 || m.Calls[0] != "hello" {
		t.Fatalf("mock did not record call: %+v", m.Calls)
	}
}

func TestContentHash_DistinguishesVoiceAndModel(t *testing.T) {
	base := ContentHash("script", "voice", "model")
	if base == "" {
		t.Fatal("empty hash")
	}
	if base != ContentHash("script", "voice", "model") {
		t.Error("hash is not deterministic")
	}
	for _, other := range []string{
		ContentHash("other", "voice", "model"),
		ContentHash("script", "other", "model"),
		ContentHash("script", "voice", "other"),
	} {
		if other == base {
			t.Error("hash collides across script/voice/model variations")
		}
	}
	// Field boundaries must not be ambiguous: ("ab","c") and ("a","bc") differ.
	if ContentHash("ab", "c", "m") == ContentHash("a", "bc", "m") {
		t.Error("hash does not delimit its fields")
	}
}

// ttsServer stands in for the ElevenLabs API. Tests that use it never touch the
// live endpoint — CI must never make outbound calls to ElevenLabs.
func ttsServer(t *testing.T, handler http.HandlerFunc) (*ElevenLabs, *int32) {
	t.Helper()
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		handler(w, r)
	}))
	t.Cleanup(srv.Close)

	return &ElevenLabs{
		APIKey:   "test-key",
		VoiceID:  "test-voice",
		ModelID:  "eleven_flash_v2_5",
		AudioDir: t.TempDir(),
		HTTP:     srv.Client(),
		BaseURL:  srv.URL + "/v1/text-to-speech/",
	}, &calls
}

func TestElevenLabs_SynthesizeAndCache(t *testing.T) {
	e, calls := ttsServer(t, func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("xi-api-key"); got != "test-key" {
			t.Errorf("xi-api-key = %q, want the configured key", got)
		}
		w.Header().Set("Content-Type", "audio/mpeg")
		w.Write([]byte("ID3-FAKE-MP3"))
	})

	audio, err := e.Synthesize(context.Background(), "hello ops")
	if err != nil {
		t.Fatalf("first synthesis: %v", err)
	}
	if string(audio.Bytes) != "ID3-FAKE-MP3" {
		t.Errorf("got audio %q", audio.Bytes)
	}

	// The mp3 must land on disk under its hash, or the cache can never hit.
	if _, err := os.Stat(filepath.Join(e.AudioDir, audio.Hash+".mp3")); err != nil {
		t.Fatalf("audio was not cached to disk: %v", err)
	}

	// Identical input must not be billed twice.
	again, err := e.Synthesize(context.Background(), "hello ops")
	if err != nil {
		t.Fatalf("second synthesis: %v", err)
	}
	if again.Hash != audio.Hash {
		t.Errorf("cache returned a different hash: %q vs %q", again.Hash, audio.Hash)
	}
	if n := atomic.LoadInt32(calls); n != 1 {
		t.Errorf("made %d API calls for identical input, want 1", n)
	}

	// Different input still calls out.
	if _, err := e.Synthesize(context.Background(), "different script"); err != nil {
		t.Fatal(err)
	}
	if n := atomic.LoadInt32(calls); n != 2 {
		t.Errorf("made %d API calls, want 2 after a new script", n)
	}
}

func TestElevenLabs_RetriesOnceOnServerError(t *testing.T) {
	var attempts int32
	e, _ := ttsServer(t, func(w http.ResponseWriter, r *http.Request) {
		if atomic.AddInt32(&attempts, 1) == 1 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.Write([]byte("ID3-RECOVERED"))
	})

	audio, err := e.Synthesize(context.Background(), "flaky")
	if err != nil {
		t.Fatalf("retry did not recover: %v", err)
	}
	if string(audio.Bytes) != "ID3-RECOVERED" {
		t.Errorf("got %q", audio.Bytes)
	}
	if n := atomic.LoadInt32(&attempts); n != 2 {
		t.Errorf("made %d attempts, want 2", n)
	}
}

func TestElevenLabs_DoesNotRetryClientError(t *testing.T) {
	// A 401 fails identically twice; retrying only burns latency.
	e, calls := ttsServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"detail":"invalid api key"}`))
	})

	if _, err := e.Synthesize(context.Background(), "unauthorized"); err == nil {
		t.Fatal("expected an error on 401")
	}
	if n := atomic.LoadInt32(calls); n != 1 {
		t.Errorf("made %d calls on a 401, want 1 (no retry)", n)
	}
}

// A dead API must leave no cache entry behind — otherwise the next request
// would serve a zero-byte mp3 forever.
func TestElevenLabs_FailureCachesNothing(t *testing.T) {
	e, _ := ttsServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	})

	if _, err := e.Synthesize(context.Background(), "doomed"); err == nil {
		t.Fatal("expected an error")
	}
	entries, err := os.ReadDir(e.AudioDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Errorf("failed synthesis left %d files behind: %v", len(entries), entries)
	}
}

func TestElevenLabs_BudgetExhausted(t *testing.T) {
	e, calls := ttsServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("ID3-FAKE-MP3"))
	})
	e.CharBudget = 10

	if _, err := e.Synthesize(context.Background(), "this script is far longer than ten characters"); !errors.Is(err, ErrBudgetExhausted) {
		t.Fatalf("err = %v, want ErrBudgetExhausted", err)
	}
	if n := atomic.LoadInt32(calls); n != 0 {
		t.Errorf("made %d API calls past budget, want 0", n)
	}
}

func TestElevenLabs_BudgetResetsMonthly(t *testing.T) {
	e := &ElevenLabs{CharBudget: 10}
	july := time.Date(2026, 7, 31, 12, 0, 0, 0, time.UTC)

	if err := e.reserve(8, july); err != nil {
		t.Fatalf("first reserve: %v", err)
	}
	if err := e.reserve(8, july); !errors.Is(err, ErrBudgetExhausted) {
		t.Fatalf("second reserve in the same month: err = %v, want ErrBudgetExhausted", err)
	}
	if err := e.reserve(8, july.AddDate(0, 1, 0)); err != nil {
		t.Errorf("budget did not reset for the new month: %v", err)
	}
}

// --- Service-level ---

func serviceAnomaly() analytics.Anomaly {
	return analytics.Anomaly{
		Kind:     analytics.RateSpike,
		Scope:    "global",
		Observed: 400,
		Baseline: 90,
		Severity: analytics.Yellow,
		At:       time.Now(),
	}
}

func TestService_CachesByContentHash(t *testing.T) {
	// The ElevenLabs client owns the cache, so drive the Service through a real
	// one pointed at a fake server. Two identical anomalies, one API call.
	e, calls := ttsServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("ID3-FAKE-MP3"))
	})
	svc := New(e, NewStore(10), quietLogger())

	a := serviceAnomaly()
	first, err := svc.Handle(context.Background(), a)
	if err != nil {
		t.Fatal(err)
	}
	second, err := svc.Handle(context.Background(), a)
	if err != nil {
		t.Fatal(err)
	}

	if n := atomic.LoadInt32(calls); n != 1 {
		t.Errorf("identical anomalies caused %d API calls, want 1", n)
	}
	if first.Hash != second.Hash {
		t.Errorf("identical anomalies produced different hashes: %q vs %q", first.Hash, second.Hash)
	}
	if first.AudioURL != "/api/briefings/"+first.Hash+"/audio" {
		t.Errorf("AudioURL = %q, want the audio route for the hash", first.AudioURL)
	}
}

func TestService_HandleStoresRecord(t *testing.T) {
	store := NewStore(10)
	svc := New(&Mock{}, store, quietLogger())

	rec, err := svc.Handle(context.Background(), serviceAnomaly())
	if err != nil {
		t.Fatal(err)
	}
	if rec.Script == "" {
		t.Error("record has no script")
	}
	if rec.Kind != analytics.RateSpike || rec.Scope != "global" || rec.Severity != analytics.Yellow {
		t.Errorf("record did not carry the anomaly through: %+v", rec)
	}
	if _, ok := store.ByHash(rec.Hash); !ok {
		t.Error("record was not added to the store")
	}
}

// A dead synthesizer must degrade to a text-only record, never panic or drop
// the briefing — the script is the fallback artifact.
func TestService_TTSFailureDegradesToTextOnly(t *testing.T) {
	store := NewStore(10)
	svc := New(&Mock{Err: errors.New("connection reset by peer")}, store, quietLogger())

	rec, err := svc.Handle(context.Background(), serviceAnomaly())
	if err != nil {
		t.Fatalf("a TTS failure must not fail the briefing: %v", err)
	}
	if rec.AudioURL != "" {
		t.Errorf("AudioURL = %q, want empty for a text-only briefing", rec.AudioURL)
	}
	if rec.Script == "" {
		t.Error("text-only record lost its script — nothing left to show")
	}
	if rec.Hash == "" {
		t.Error("text-only record has no hash; the frontend needs a stable key")
	}
	if _, ok := store.ByHash(rec.Hash); !ok {
		t.Error("text-only record was not stored")
	}
}

func TestService_BudgetExhaustedDegradesToTextOnly(t *testing.T) {
	svc := New(&Mock{Err: ErrBudgetExhausted}, NewStore(10), quietLogger())

	rec, err := svc.Handle(context.Background(), serviceAnomaly())
	if err != nil {
		t.Fatalf("budget exhaustion must not fail the briefing: %v", err)
	}
	if rec.AudioURL != "" || rec.Script == "" {
		t.Errorf("want a text-only record with a script, got %+v", rec)
	}
}
