# Package briefing

Turns `analytics.Anomaly` events into spoken "control room" briefings: compose script →
synthesize audio (ElevenLabs) → store bounded history → hand back a `Record` to broadcast.

## Contract

- Optional dependency: constructed only when `BRIEFING_ENABLED=true`; every call site guards `if svc != nil` (same as `storage`/`analytics`).
- `Compose` is pure and template-only. Don't put an LLM in the core path; Do keep the script as the text-only fallback.
- Audio is content-hash cached (SHA-256 of script+voice+model). Don't re-synthesize equal input; Do check the on-disk cache first.
- `Store` is an in-memory ring buffer. Don't persist briefings to DynamoDB in v1; Do mirror the `latencySamples` pattern in `analytics/tracker.go`. `IMPORTANT:` AI-authored DynamoDB schemas are a repo silent-failure trap — schema work is hand-written only.

## Wiring

- Never run `Service.Handle` inline on the detector ticker. `IMPORTANT:` synthesis takes 1–3s; inline `await` silently backs up metric polling. Dispatch each anomaly to its own goroutine.
- Delivery reuses the existing hub typed-frame broadcast (`type: "briefing.ready"`). No new socket.

## Testing

- `Synthesizer` is a narrow interface; tests swap in `Mock`, never the network. `IMPORTANT:` CI must not reach `api.elevenlabs.io` — it burns credits and goes flaky on rate limits.
