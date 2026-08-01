package briefing

import (
	"encoding/json"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/epw80/chat-analytics-platform/pkg/analytics"
)

// errUnavailable stands in for a dead ElevenLabs API.
var errUnavailable = errors.New("elevenlabs unavailable")

// fakeMetrics is a MetricsSource that hands back a fixed snapshot.
type fakeMetrics struct {
	mu sync.Mutex
	m  analytics.Metrics
}

func (f *fakeMetrics) GetMetrics() analytics.Metrics {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.m
}

// capture collects broadcast frames.
type capture struct {
	mu     sync.Mutex
	frames [][]byte
}

func (c *capture) broadcast(b []byte) {
	c.mu.Lock()
	c.frames = append(c.frames, b)
	c.mu.Unlock()
}

func (c *capture) count() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.frames)
}

func (c *capture) first() []byte {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.frames) == 0 {
		return nil
	}
	return c.frames[0]
}

// spikeMetrics is a snapshot that trips the rate-spike rule.
func spikeMetrics() analytics.Metrics {
	perMin := make([]int64, 15)
	for i := 0; i < 14; i++ {
		perMin[i] = 90
	}
	perMin[14] = 400
	return analytics.Metrics{MessagesPerMinute: perMin}
}

func waitFor(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s", what)
}

func TestRunner_BroadcastsReadyFrame(t *testing.T) {
	metrics := &fakeMetrics{m: spikeMetrics()}
	cap := &capture{}
	svc := New(&Mock{}, NewStore(10), quietLogger())
	r := NewRunner(metrics, analytics.NewDetector(analytics.DefaultThresholds()), svc, cap.broadcast, 10*time.Millisecond, quietLogger())

	go r.Run()
	defer r.Stop()

	waitFor(t, "a briefing frame", func() bool { return cap.count() > 0 })

	frame := cap.first()
	if frame[len(frame)-1] != '\n' {
		t.Error("frame must be newline-terminated; writePump batches newline-delimited JSON")
	}

	var got readyFrame
	if err := json.Unmarshal(frame, &got); err != nil {
		t.Fatalf("frame is not valid JSON: %v\n%s", err, frame)
	}
	if got.Type != "briefing.ready" {
		t.Errorf("frame type = %q, want briefing.ready", got.Type)
	}
	if got.Briefing.Kind != analytics.RateSpike {
		t.Errorf("briefing kind = %q, want rate_spike", got.Briefing.Kind)
	}
	if got.Briefing.Script == "" {
		t.Error("briefing carries no script")
	}
	if got.Briefing.AudioURL == "" {
		t.Error("briefing carries no audio URL despite a working synthesizer")
	}
}

// The frame the frontend consumes must match docs/briefing-frame-contract.md
// key for key — this is the seam between the two phases.
func TestRunner_FrameMatchesContract(t *testing.T) {
	frame, err := marshalFrame(Record{
		Hash:      "3c556b79",
		Kind:      analytics.RateSpike,
		Scope:     "global",
		Severity:  analytics.Yellow,
		Script:    "Ops, analytics.",
		AudioURL:  "/api/briefings/3c556b79/audio",
		CreatedAt: time.Now(),
	})
	if err != nil {
		t.Fatal(err)
	}

	var envelope map[string]json.RawMessage
	if err := json.Unmarshal(frame, &envelope); err != nil {
		t.Fatal(err)
	}
	if _, ok := envelope["briefing"]; !ok {
		t.Fatal(`frame is missing the "briefing" key`)
	}

	var body map[string]json.RawMessage
	if err := json.Unmarshal(envelope["briefing"], &body); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"hash", "kind", "scope", "severity", "script", "audioUrl", "createdAt"} {
		if _, ok := body[key]; !ok {
			t.Errorf("briefing body is missing %q", key)
		}
	}
	// The disk path must never reach the browser.
	if _, ok := body["audioPath"]; ok {
		t.Error(`briefing body leaks "audioPath"; only audioUrl belongs on the wire`)
	}
}

// Debounce lives in the detector, but the runner is where it pays off: a metric
// stuck in an anomalous state must not narrate on every tick.
func TestRunner_DebouncesRepeatAnomalies(t *testing.T) {
	metrics := &fakeMetrics{m: spikeMetrics()}
	cap := &capture{}
	mock := &Mock{}
	svc := New(mock, NewStore(10), quietLogger())
	r := NewRunner(metrics, analytics.NewDetector(analytics.DefaultThresholds()), svc, cap.broadcast, 5*time.Millisecond, quietLogger())

	go r.Run()
	defer r.Stop()

	waitFor(t, "the first briefing", func() bool { return cap.count() > 0 })
	time.Sleep(100 * time.Millisecond) // ~20 more ticks, all inside the debounce window

	if n := cap.count(); n != 1 {
		t.Errorf("broadcast %d briefings for a persistent anomaly, want 1", n)
	}
	if n := mock.CallCount(); n != 1 {
		t.Errorf("synthesized %d times, want 1 — every extra call bills credits", n)
	}
}

func TestRunner_StopEndsTheLoop(t *testing.T) {
	metrics := &fakeMetrics{m: spikeMetrics()}
	cap := &capture{}
	svc := New(&Mock{}, NewStore(10), quietLogger())
	r := NewRunner(metrics, analytics.NewDetector(analytics.DefaultThresholds()), svc, cap.broadcast, 5*time.Millisecond, quietLogger())

	done := make(chan struct{})
	go func() {
		r.Run()
		close(done)
	}()

	r.Stop()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Run did not return after Stop")
	}
}

// A dead synthesizer must still produce a briefing — text-only, but broadcast.
func TestRunner_BroadcastsTextOnlyOnTTSFailure(t *testing.T) {
	metrics := &fakeMetrics{m: spikeMetrics()}
	cap := &capture{}
	svc := New(&Mock{Err: errUnavailable}, NewStore(10), quietLogger())
	r := NewRunner(metrics, analytics.NewDetector(analytics.DefaultThresholds()), svc, cap.broadcast, 10*time.Millisecond, quietLogger())

	go r.Run()
	defer r.Stop()

	waitFor(t, "a text-only briefing frame", func() bool { return cap.count() > 0 })

	var got readyFrame
	if err := json.Unmarshal(cap.first(), &got); err != nil {
		t.Fatal(err)
	}
	if got.Briefing.AudioURL != "" {
		t.Errorf("AudioURL = %q, want empty when synthesis failed", got.Briefing.AudioURL)
	}
	if got.Briefing.Script == "" {
		t.Error("text-only briefing must still carry its script")
	}
}

func TestNewRunner_DefaultsInterval(t *testing.T) {
	r := NewRunner(&fakeMetrics{}, analytics.NewDetector(analytics.DefaultThresholds()),
		New(&Mock{}, NewStore(1), quietLogger()), func([]byte) {}, 0, quietLogger())
	if r.interval != 5*time.Second {
		t.Errorf("interval = %v, want the 5s default", r.interval)
	}
}
