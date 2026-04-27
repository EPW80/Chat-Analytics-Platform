# Chat Analytics Platform

Real-time Twitch-style chat analytics. Go WebSocket backend, React/TypeScript frontend, DynamoDB persistence.

## Status

- Phase 1 (WebSocket core): COMPLETE
- Phase 2A (Docker + DynamoDB): IN PROGRESS — table init debugging
- Phase 2B (Analytics backend): PENDING
- Phase 2C (React frontend): PENDING

For full architecture, schemas, and phase details, see `docs/BUILD_PLAN.md`.

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
backend/pkg/analytics/            # Metrics tracking (Phase 2B)
frontend/src/                     # React app (Phase 2C)
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
- `IMPORTANT:` There is a pre-existing race condition in `TestClient_PingPong`. Do not spend time fixing it unless explicitly asked.

## Key Patterns

- Two goroutines per WebSocket client: readPump + writePump
- Hub uses channels (register/unregister/broadcast) with a central select loop
- Storage layer is interface-based (`MessageRepository`) for testability and nil-safe graceful degradation
- DynamoDB: partition key = RoomID, sort key = MessageID (UUID). Two GSIs for user history and chronological queries

## When Working on Phase 2B

Read `docs/BUILD_PLAN.md` § "Phase 2B: Analytics Backend" before starting. Key constraint: analytics tracker integrates into Hub (joins/leaves/latency) and Client (message tracking), passed as optional dependency same as storage.

## When Working on Phase 2C

Read `docs/BUILD_PLAN.md` § "Phase 2C: React Frontend" before starting. Key constraint: useWebSocket hook must implement exponential backoff reconnection (max 5 attempts). Use react-window for message list virtualization.
