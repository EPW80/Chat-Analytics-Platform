# Chat Analytics Platform

Real-time Twitch-style chat analytics. Go WebSocket backend, React/TypeScript frontend, DynamoDB persistence.

## Status

- Phase 1 (WebSocket core): COMPLETE
- Phase 2A (Docker + DynamoDB): IN PROGRESS — table init debugging
- Phase 2B (Analytics backend): COMPLETE
- Phase 2C (React frontend): COMPLETE
- Phase 3 (Voice Ops Briefings): COMPLETE — off by default via `BRIEFING_ENABLED`

For full architecture, schemas, and phase details, see `docs/BUILD_PLAN.md`.
For the voice briefings feature, see `docs/VOICE_BRIEFINGS_PLAN.md` and
`docs/briefing-frame-contract.md`.

## Stack

- Backend: Go 1.23, gorilla/websocket, aws-sdk-go-v2, slog
- Frontend: Vite + React 18 + TypeScript 5, Tailwind, react-window
- Storage: DynamoDB (local via Docker, production via AWS)
- Infra: Docker Compose, ECS (future)

## Commands

```bash
docker-compose up -d          # Start backend + DynamoDB Local
./scripts/init-tables.sh      # Create DynamoDB tables
go test ./... -race            # Run all tests with race detection
go test ./... -cover           # Coverage report
curl localhost:8080/health     # Health check
```

## Project Structure

```
backend/cmd/server/main.go       # Entry point
backend/pkg/hub/                  # WebSocket connection manager
backend/pkg/client/               # Per-connection read/write pumps
backend/pkg/message/              # Message types + validation
backend/pkg/storage/              # DynamoDB repository (interface-based)
backend/pkg/config/               # Environment variable loading
backend/pkg/analytics/            # Metrics tracking + anomaly Detector
backend/pkg/briefing/             # Voice briefings: compose, TTS, store, runner
frontend/src/                     # React app
```

## Conventions

- Don't use `any` or untyped interfaces. Do use concrete types and narrow interfaces.
- Don't block the broadcast loop for I/O. Do persist messages in a goroutine with a 5s context timeout.
- Don't log with fmt.Println. Do use `slog` structured logging everywhere.
- Don't skip nil checks on optional deps. Do guard storage/analytics calls with `if x != nil`.
- Don't write DynamoDB table schemas or IAM policies with AI. Do write those by hand. `IMPORTANT:` Silent failures in schema definitions are hard to debug.
- Don't create DynamoDB writes for high-frequency metrics. Do use atomic counters and in-memory sliding windows for analytics.

## Testing

- Every new package gets a `_test.go` file. Run `go test ./... -race` before committing.
- Hub, Client, Message, Config, Storage packages all have existing test suites — don't break them.
- The historical `TestClient_PingPong` race was fixed (atomic ping counter in the test); the whole suite now runs under `-race` in one step, locally and in CI. Keep it that way.

## Key Patterns

- Two goroutines per WebSocket client: readPump + writePump
- Hub uses channels (register/unregister/broadcast) with a central select loop
- Storage layer is interface-based (`MessageRepository`) for testability and nil-safe graceful degradation
- DynamoDB: partition key = RoomID, sort key = MessageID (UUID). Two GSIs for user history and chronological queries

## When Working on Voice Briefings

Read `backend/pkg/briefing/CLAUDE.md` first; `docs/VOICE_BRIEFINGS_PLAN.md` has the full rationale.

- Detection runs on a 5s ticker reading `Tracker.GetMetrics()`. It never touches `TrackMessage` or the broadcast loop.
- Don't await `Service.Handle` on the ticker. Do dispatch each anomaly to its own goroutine. `IMPORTANT:` synthesis takes 1–3s; inline awaiting silently backs up metric polling with no error anywhere.
- Don't put an LLM in the compose path. Do keep `Compose` a pure template function — the script is the text-only fallback when TTS fails.
- Don't re-synthesize equal input. Do rely on the SHA-256 content cache; every miss bills ElevenLabs credits.
- Don't let tests reach `api.elevenlabs.io`. Do swap in the `Mock` Synthesizer — CI enforces this with a grep guard.
- Briefing history is an in-memory ring buffer in v1. Persisting it to DynamoDB means hand-writing the schema (see the convention above).
