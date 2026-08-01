// Package briefing turns detected analytics anomalies into spoken "control room"
// briefings: it composes a script (templates, deterministic), synthesizes audio
// via a Synthesizer (ElevenLabs in prod, a mock in tests), stores a bounded
// history in memory, and hands a ready record back to the caller to broadcast.
//
// The whole package is an OPTIONAL dependency of the server, wired exactly like
// storage and analytics: constructed only when BRIEFING_ENABLED=true, and every
// call site guards with `if svc != nil`.
package briefing

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/epw80/chat-analytics-platform/pkg/analytics"
)

// Record is one briefing: the script always exists; AudioURL is empty when TTS
// failed or was skipped (budget exhausted), in which case the record is text-only.
//
// AudioURL is deliberately a URL, not a filesystem path. The mp3 lives on the
// server's volume and is only ever reachable through the audio handler, so the
// on-disk location never reaches the browser.
type Record struct {
	Hash      string                `json:"hash"`
	Kind      analytics.AnomalyKind `json:"kind"`
	Scope     string                `json:"scope"`
	Severity  analytics.Severity    `json:"severity"`
	Script    string                `json:"script"`
	AudioURL  string                `json:"audioUrl"` // "" => text-only
	CreatedAt time.Time             `json:"createdAt"`
}

// audioURLFor builds the client-facing URL for a synthesized briefing.
func audioURLFor(hash string) string {
	return "/api/briefings/" + hash + "/audio"
}

// Service composes → synthesizes → stores. Construct with New.
type Service struct {
	synth Synthesizer
	store *Store
	log   *slog.Logger
}

// New wires a Service. Pass a real ElevenLabs Synthesizer in prod, a Mock in tests.
func New(synth Synthesizer, store *Store, log *slog.Logger) *Service {
	return &Service{synth: synth, store: store, log: log}
}

// Store exposes the ring buffer so main.go can hand it to the HTTP handlers
// without reaching into unexported state.
func (s *Service) Store() *Store { return s.store }

// Handle runs the full pipeline for one anomaly and returns the stored Record.
// It is safe to call from its own goroutine (see BUILD_PLAN §Phase 3: never run
// this inline on the detector ticker — synthesis takes 1–3s).
//
// A synthesis failure is not an error: the script is the fallback artifact, so
// the record is stored text-only and returned with a nil error. Only a failure
// to produce a briefing at all would be worth surfacing.
func (s *Service) Handle(ctx context.Context, a analytics.Anomaly) (Record, error) {
	script := Compose(a)

	rec := Record{
		Kind:      a.Kind,
		Scope:     a.Scope,
		Severity:  a.Severity,
		Script:    script,
		CreatedAt: a.At,
	}

	audio, err := s.synth.Synthesize(ctx, script)
	switch {
	case errors.Is(err, ErrBudgetExhausted):
		s.log.Warn("briefing: character budget exhausted, emitting text-only briefing",
			"kind", a.Kind, "scope", a.Scope)
	case err != nil:
		s.log.Error("briefing: synthesis failed, emitting text-only briefing",
			"error", err, "kind", a.Kind, "scope", a.Scope)
	default:
		rec.Hash = audio.Hash
		rec.AudioURL = audioURLFor(audio.Hash)
	}

	// A text-only record still needs an identity for the API; fall back to the
	// script hash so the frontend has a stable key either way.
	if rec.Hash == "" {
		rec.Hash = ContentHash(script, "", "")
	}

	s.store.Add(rec)
	return rec, nil
}
