package briefing

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/epw80/chat-analytics-platform/pkg/analytics"
)

// MetricsSource is the narrow slice of *analytics.Tracker the runner needs.
// *analytics.Tracker satisfies it; tests pass a fake.
type MetricsSource interface {
	GetMetrics() analytics.Metrics
}

// Runner is the ONE place the detect→brief→broadcast loop lives. Construct it in
// NewServer (only when BRIEFING_ENABLED) and start Run() in a goroutine in main(),
// next to `go srv.hub.Run()`.
type Runner struct {
	metrics   MetricsSource
	detector  *analytics.Detector
	svc       *Service
	broadcast func([]byte) // wired to hub.BroadcastAll
	interval  time.Duration
	log       *slog.Logger
	done      chan struct{}
}

// NewRunner wires the loop. interval<=0 defaults to 5s.
func NewRunner(m MetricsSource, d *analytics.Detector, svc *Service, broadcast func([]byte), interval time.Duration, log *slog.Logger) *Runner {
	if interval <= 0 {
		interval = 5 * time.Second
	}
	return &Runner{metrics: m, detector: d, svc: svc, broadcast: broadcast, interval: interval, log: log, done: make(chan struct{})}
}

// Run drives the loop until Stop. Call in a goroutine.
//
// IMPORTANT: each anomaly is dispatched to its own goroutine. Never await
// Service.Handle inline on this ticker — synthesis takes 1–3s and would silently
// back up metric polling under an anomaly burst, with no error anywhere.
func (r *Runner) Run() {
	t := time.NewTicker(r.interval)
	defer t.Stop()

	for {
		select {
		case <-r.done:
			return
		case <-t.C:
			for _, a := range r.detector.Check(r.metrics.GetMetrics(), time.Now()) {
				go r.handle(a)
			}
		}
	}
}

// Stop signals Run to exit. Safe to call once; a second call panics on the
// closed channel, matching hub.Shutdown's contract.
func (r *Runner) Stop() { close(r.done) }

// handle runs one anomaly end to end and broadcasts the ready frame.
func (r *Runner) handle(a analytics.Anomaly) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	rec, err := r.svc.Handle(ctx, a)
	if err != nil {
		r.log.Error("briefing: handle failed", slog.Any("err", err), "kind", a.Kind, "scope", a.Scope)
		return
	}

	frame, err := marshalFrame(rec)
	if err != nil {
		r.log.Error("briefing: marshal frame failed", slog.Any("err", err), "hash", rec.Hash)
		return
	}

	r.broadcast(frame)
	r.log.Info("briefing ready",
		"kind", rec.Kind,
		"scope", rec.Scope,
		"severity", rec.Severity,
		"hash", rec.Hash,
		"audio", rec.AudioURL != "",
	)
}

// readyFrame is the WS envelope broadcast to clients. Matches
// docs/briefing-frame-contract.md; the frontend discriminates on Type.
type readyFrame struct {
	Type     string `json:"type"` // always "briefing.ready"
	Briefing Record `json:"briefing"`
}

// marshalFrame builds the newline-terminated JSON the hub broadcasts. Provided
// ready-to-use so Phase 3 only wires it in.
func marshalFrame(rec Record) ([]byte, error) {
	b, err := json.Marshal(readyFrame{Type: "briefing.ready", Briefing: rec})
	if err != nil {
		return nil, err
	}
	return append(b, '\n'), nil // writePump batches newline-delimited frames
}
