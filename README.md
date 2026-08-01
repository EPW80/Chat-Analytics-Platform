# Real-Time Chat Analytics Platform

A real-time, Twitch-style chat platform with a live analytics dashboard. A Go WebSocket backend (hub-and-spoke) handles rooms, message persistence, and metrics; a React + TypeScript frontend renders the chat and analytics in a polished dark UI.

## Overview

The platform provides real-time room-based chat with live analytics: message throughput, active users vs. connections, peak connections, and broadcast-latency percentiles. Messages are persisted to DynamoDB through a bounded worker pool and exposed via a history API, while high-frequency metrics are tracked in memory with atomic counters and a sliding window.

## Architecture

```
┌──────────────────────────────────────────────┐
│   Frontend — React + Vite + TypeScript        │
│   • Virtualized chat (react-window)           │
│   • Analytics dashboard (live polling)        │
│   • useWebSocket: exponential-backoff reconnect│
└───────────────┬──────────────────────────────┘
        wss /ws │  https /api/*
                ▼
┌──────────────────────────────────────────────┐
│   Hub (central manager)                        │
│   • Clients grouped by room (map per RoomID)   │
│   • Per-room broadcast fan-out                 │
│   • Channel-based register/unregister/broadcast│
└───────┬───────────────────────────┬───────────┘
        │                           │
        ▼                           ▼
┌───────────────────┐     ┌──────────────────────┐
│ Client connections│     │ Analytics tracker     │
│ • read/write pumps│     │ • atomic counters     │
│ • validation      │     │ • 15-min sliding window│
│ • rate limiting   │     │ • latency percentiles │
│ • ping/pong       │     │ • GET /api/analytics  │
└───────┬───────────┘     └──────────────────────┘
        │ enqueue (non-blocking)
        ▼
┌──────────────────────────────────────────────┐
│   Persistence worker pool                      │
│   • bounded, batching (BatchWriteItem)         │
│   • drops-when-full, drains on shutdown        │
└───────────────┬──────────────────────────────┘
                ▼
┌──────────────────────────────────────────────┐
│   DynamoDB (chat-messages)                     │
│   • PK RoomID / SK MessageID                   │
│   • GSI: UserID-Timestamp, RoomID-Timestamp    │
└──────────────────────────────────────────────┘
```

## Features

- **Room-based chat** — clients join a room (`?room=`, default `global`); broadcasts are scoped per room.
- **Live analytics** — total messages, active connections vs. unique users, peak connections, messages/minute (15-min window), and p50/p95/p99 broadcast latency, served at `/api/analytics`.
- **Message history API** — recent room history and per-user history, with join-time hydration so a connecting client replays recent messages.
- **Bounded persistence pool** — messages are enqueued non-blocking and written to DynamoDB in batches by a fixed worker pool, keeping the broadcast path off storage latency.
- **Per-connection rate limiting** — token-bucket throttle on inbound messages.
- **Token auth** — optional HMAC-signed bearer tokens (enabled when `AUTH_SECRET` is set); falls back to a `userId` query param for local development.
- **Configurable CORS / WebSocket origin allowlist.**
- **Graceful degradation** — runs without DynamoDB (chat + live analytics still work, no persistence).
- **Voice Ops Briefings** — an anomaly detector watches the live metrics and narrates traffic events aloud in a control-room register via ElevenLabs TTS. Off by default; see below.
- **Polished React frontend** — refined dark theme, design tokens, reusable UI primitives, avatars, virtualized message list, and a live metrics dashboard.
- **Comprehensive tests** — every backend package has a `_test.go`; the suite runs clean under `-race`.

## Quick Start

### Full stack via Docker Compose

**Prerequisites:** Docker + Docker Compose, Git.

```bash
git clone https://github.com/EPW80/Chat-Analytics-Platform.git
cd Chat-Analytics-Platform

# Start DynamoDB Local + backend + frontend
docker-compose up -d

# Create DynamoDB tables (enables persistence + history)
./scripts/init-tables.sh

# Health check
curl http://localhost:8080/health
# {"status":"ok","clients":0,"storage":"ok"}
```

Services:
- Frontend: `http://localhost:3000`
- Backend: `http://localhost:8080` (WebSocket at `ws://localhost:8080/ws`)
- DynamoDB Local: `http://localhost:8000`

`./start.sh` orchestrates the same flow (build, wait for health, init tables).

### Backend only (local Go)

**Prerequisites:** Go 1.23+.

```bash
cd backend
go mod download
go run ./cmd/server
# Runs on :8080. Without DynamoDB reachable it logs a warning and
# continues with storage disabled (chat + analytics still work).
```

### Frontend only (Vite dev server)

**Prerequisites:** Node 20+.

```bash
cd frontend
npm install
npm run dev          # http://localhost:5173
```

The frontend reads `VITE_WS_URL` and `VITE_API_URL` (defaults point at `localhost:8080`). For a production build these are baked in at build time.

## API

### `GET /health`
Liveness + readiness. `storage` is `ok`, `unavailable`, or `disabled`.
```json
{ "status": "ok", "clients": 5, "storage": "ok" }
```

### `WS /ws`
WebSocket chat endpoint.

| Query param | Default | Notes |
|-------------|---------|-------|
| `userId`    | `anonymous` | ignored when token auth is enabled |
| `username`  | `Anonymous` | display name |
| `room`      | `global` | room to join |
| `token`     | — | required when `AUTH_SECRET` is set (or `Authorization: Bearer`) |

```
ws://localhost:8080/ws?userId=user123&username=Alice&room=global
```

### `GET /api/analytics`
Point-in-time metrics snapshot.
```json
{
  "totalMessages": 1280,
  "activeConnections": 4,
  "activeUsers": 3,
  "peakConnections": 17,
  "messagesPerMinute": [/* last 15 minutes */],
  "latencyP50Ms": 0.4, "latencyP95Ms": 1.2, "latencyP99Ms": 2.1,
  "activeUserDetails": [{ "clientId": "...", "userId": "...", "username": "Alice", "joinedAt": "..." }],
  "uptimeSeconds": 3600, "serverStartTime": "..."
}
```

### `GET /api/rooms/{id}/messages` · `GET /api/users/{id}/messages`
Recent message history for a room or a user. Optional `?limit=` (default 50, max 200). Returns `503` when storage is unavailable.

### Message format
```json
{
  "messageId": "550e8400-e29b-41d4-a716-446655440000",
  "roomId": "global",
  "type": "chat",
  "userId": "user123",
  "username": "Alice",
  "content": "Hello world",
  "timestamp": "2026-06-16T10:30:00Z"
}
```
**Types:** `chat`, `system`, `join`, `leave`. **Validation:** username required (≤50 chars); chat content required (≤1000 chars); `messageId`/`timestamp`/`roomId` are server-authoritative.

## Voice Ops Briefings

An anomaly detector polls the live metrics every 5 seconds and narrates traffic events aloud, so an operator watching the dashboard hears about a spike instead of having to notice it.

```
 ticker (5s, own goroutine)  — never on the message hot path
        ▼
 analytics.Detector.Check(tracker.GetMetrics(), now) ──► []Anomaly
        ▼                      (stateful: prev snapshot + per-(kind,scope) debounce)
 briefing.Compose(anomaly) ──► script            [pure templates, no LLM]
        ▼
 briefing.ElevenLabs.Synthesize(ctx, script) ──► mp3   [SHA-256 content cache]
        ├──► briefing.Store.Add(record)          (in-memory ring buffer)
        ├──► write {BRIEFING_AUDIO_DIR}/{hash}.mp3
        └──► hub.BroadcastAll({"type":"briefing.ready", ...})
```

**Detected conditions:** message-rate spike, p99 latency breach, connection surge, traffic dropout — each in a Yellow or Red severity band, debounced to at most one briefing per kind per 5 minutes.

A sample script, as spoken:

> Ops, analytics. Message-rate spike in the global room — four hundred messages per minute against a baseline of ninety. Monitoring.

**Design notes.** Detection reads a metrics snapshot on a ticker and never touches the broadcast loop. Synthesis takes 1–3s, so each anomaly is dispatched to its own goroutine — awaiting it inline would silently back up metric polling. Audio is cached by SHA-256 of (script + voice + model), so identical input is never billed twice. Scripts are deterministic templates, not LLM output, and the script doubles as the fallback artifact: if ElevenLabs is down or the character budget is spent, the briefing still ships as text.

The whole feature is an optional dependency. With `BRIEFING_ENABLED=false` the detector goroutine never starts and the routes are not registered — the same binary, feature dark.

### `GET /api/briefings?limit=N`
Recent briefings, newest first, from the in-memory ring buffer. `404` when the feature is disabled.

```json
{ "briefings": [
  {
    "hash": "3c556b79...",
    "kind": "rate_spike",
    "scope": "global",
    "severity": "yellow",
    "script": "Ops, analytics. Message-rate spike in the global room ...",
    "audioUrl": "/api/briefings/3c556b79.../audio",
    "createdAt": "2026-07-31T18:04:11Z"
  }
] }
```

`kind` ∈ `rate_spike | latency_breach | connection_surge | traffic_dropout`; `severity` ∈ `yellow | red`; `audioUrl` is `""` for a text-only briefing.

### `GET /api/briefings/{hash}/audio`
Serves the mp3 (`Content-Type: audio/mpeg`). `404` if the hash is unknown or the briefing is text-only.

The same payload arrives over the WebSocket as a `{"type": "briefing.ready", "briefing": {...}}` frame — no separate socket. The frontend plays it only when the **Voice on** toggle is set (persisted to localStorage, default off), since browsers reject audio autoplay before a user gesture.

## Project Structure

```
RealTimeChatAnalyticsPlatform/
├── backend/                     # Go WebSocket server
│   ├── cmd/server/              # Entry point, HTTP routes, wiring
│   └── pkg/
│       ├── analytics/           # Atomic counters, sliding window, anomaly detector
│       ├── auth/                # HMAC-signed token authenticator
│       ├── briefing/            # Voice briefings: compose, TTS, store, runner
│       ├── client/              # Per-connection read/write pumps
│       ├── config/              # Environment configuration
│       ├── hub/                 # Room-based connection manager
│       ├── message/             # Message types + validation
│       ├── persist/             # Bounded batching persistence worker pool
│       ├── ratelimit/           # Per-connection token bucket
│       └── storage/             # DynamoDB repository (interface-based)
├── frontend/                    # React + Vite + TypeScript + Tailwind
│   └── src/
│       ├── components/          # Chat, message list, input, user list, dashboard
│       │   └── ui/              # Reusable primitives (Button, Card, Badge, …)
│       ├── hooks/               # useWebSocket, useAnalytics, useBriefings, useElementSize
│       ├── lib/                 # cn, userColor helpers
│       └── types/               # Shared TypeScript types
├── scripts/                     # init-tables.sh, wait-for-dynamodb.sh, …
├── docker-compose.yml           # Local stack (DynamoDB + backend + frontend)
├── start.sh                     # One-command local startup
└── docs/BUILD_PLAN.md           # Full architecture & phase plan
```

## Configuration

Backend (environment variables):

| Variable | Default | Purpose |
|----------|---------|---------|
| `PORT` | `8080` | HTTP/WS port |
| `LOG_LEVEL` | `info` | `debug` / `info` / `warn` / `error` |
| `DYNAMODB_ENDPOINT` | `http://localhost:8000` | local endpoint; empty/AWS for production |
| `DYNAMODB_REGION` | `us-east-1` | DynamoDB region |
| `AWS_ACCESS_KEY_ID` / `AWS_SECRET_ACCESS_KEY` | `dummy` | local creds; use an IAM role in production |
| `ALLOWED_ORIGINS` | `*` | CORS + WebSocket origin allowlist (comma-separated) |
| `AUTH_SECRET` | — | HMAC secret; empty disables token auth |
| `RATE_LIMIT_PER_SEC` / `RATE_LIMIT_BURST` | `5` / `10` | per-connection token bucket (`<=0` disables) |
| `PERSIST_WORKERS` / `PERSIST_BATCH_SIZE` / `PERSIST_QUEUE_SIZE` | `4` / `25` / `1024` | persistence pool tuning |
| `BRIEFING_ENABLED` | `false` | master switch for Voice Ops Briefings |
| `ELEVENLABS_API_KEY` / `ELEVENLABS_VOICE_ID` | — | ElevenLabs credentials; required when briefings are on |
| `ELEVENLABS_MODEL_ID` | `eleven_flash_v2_5` | TTS model; flash is fast and ~half the credit cost |
| `BRIEFING_MONTHLY_CHAR_BUDGET` | `10000` | characters/month before synthesis is refused (`<=0` disables the guard) |
| `BRIEFING_AUDIO_DIR` | `/data/briefings` | mp3 cache directory; must be on a persisted volume |

Frontend: `VITE_WS_URL` (WebSocket URL) and `VITE_API_URL` (REST base), baked in at build time.

## Development

```bash
cd backend
go test ./... -race      # all packages, race detector
go test ./... -cover     # coverage
go build ./cmd/server    # build the binary

cd ../frontend
npm run build            # tsc type-check + vite build
npm run lint             # eslint
```

> Note: `TestClient_PingPong` has a documented pre-existing race in the test harness — left as-is intentionally.

## Deployment

Production target: **AWS ECS Fargate** for the backend behind an ALB, **DynamoDB** (on-demand) for persistence, and the static frontend on **S3 + CloudFront**. The DynamoDB table and IAM policies are authored by hand. The backend uses the AWS default credential chain (task IAM role) when `DYNAMODB_ENDPOINT` is empty; `ALLOWED_ORIGINS` must list the public frontend origin and TLS (`wss`) is terminated at the edge.

## Technology Stack

**Backend:** Go 1.23, gorilla/websocket, aws-sdk-go-v2 (DynamoDB), google/uuid, `slog` structured logging, goroutines + channels.

**Frontend:** React 18, Vite 5, TypeScript 5, Tailwind CSS 3, react-window (virtualization), lucide-react (icons), Inter (font).

**Infrastructure:** Docker + Docker Compose, DynamoDB Local (dev) / AWS DynamoDB (prod).

## Security Notes

Implemented: token (HMAC) auth, configurable CORS/origin allowlist, per-connection rate limiting, server-authoritative message fields. For production also ensure: TLS/`wss` at the edge, a strong `AUTH_SECRET` via a secrets manager, an IAM task role (no static keys), and a restrictive `ALLOWED_ORIGINS`.

## License

MIT

## Author

Erik Williams ([@EPW80](https://github.com/EPW80))
