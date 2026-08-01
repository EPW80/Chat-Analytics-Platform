# BUILD_PLAN — Voice Ops Briefings ("Control Room")

> Give the analytics dashboard a voice: detect traffic anomalies over the live metrics,
> narrate them via ElevenLabs TTS, and deliver the audio through the existing hub fan-out.
>
> Target repo: **Chat-Analytics-Platform** (`github.com/epw80/chat-analytics-platform`).
> Module path prefix for all new packages: `github.com/epw80/chat-analytics-platform/pkg/...`

**Scope:** ~2–3 evenings. Small integration on top of a finished analytics layer, high demo value.
**Non-goals:** speech-to-text, conversational agents, multi-voice dialogue, per-user narration.

**Grounding note:** `CLAUDE.md` lists Phase 2B (analytics) as PENDING, but `pkg/analytics/tracker.go`
is complete and tested. This plan treats analytics as a finished, verified dependency (trust the code,
not the stale status line) and adds the one thing the repo genuinely lacks: an anomaly layer.

---

## Model routing

| Work | Model |
| ---- | ----- |
| This plan + Phase 3 wiring review + detector threshold design | Opus |
| Phases 1–4 implementation against fixed signatures | Sonnet |
| Env plumbing, README, Compose edits, mock fixtures | Haiku |

---

## Architecture (agreed before any code)

```
 ticker (5s, own goroutine)
        │  reads a snapshot — never on the message hot path
        ▼
 analytics.Detector.Check(Tracker.GetMetrics(), prev)  ──►  []Anomaly
        │   (stateful: holds previous snapshot; debounces per (kind,scope))
        ▼
 briefing.Composer.Compose(anomaly)  ──►  script string   [templates, deterministic]
        ▼
 briefing.TTS.Synthesize(ctx, script) ──► mp3 bytes        [content-hash cached]
        │
        ├──► briefing.Store.Add(record)          (in-memory ring buffer, like latencySamples)
        ├──► write /data/briefings/{hash}.mp3     (existing Docker volume)
        └──► hub broadcast: {"type":"briefing.ready", ...}  ──► frontend audio player
```

**Seam:** `analytics/` decides *something is anomalous*; `briefing/` decides *how it sounds and how it ships*.

**Invariants:**
- Detection reads `Tracker.GetMetrics()` on a ticker. It never touches `TrackMessage` / the broadcast loop.
- TTS and file I/O run off the hot path. A dead ElevenLabs API must never slow ingest or broadcast.
- Audio is cached by SHA-256 of (script + voice_id + model_id). Same input is never billed twice.
- Briefing scripts are deterministic given an anomaly. Templates, not an LLM, in the core path.
- Briefing feature is an **optional dependency**, nil-safe, wired exactly like `storage` and `analytics`
  (guard every call site with `if b != nil`). `BRIEFING_ENABLED=false` by default.

**Conventions carried from repo `CLAUDE.md` (do not relitigate):**
`slog` everywhere; concrete types and narrow interfaces, no `any`; every new package ships a `_test.go`;
`go test ./... -race` before commit; leave the known `TestClient_PingPong` race alone.

---

## Phase 0 — Discovery, seam confirmation, smoke test

- [ ] Confirm three integration facts against the source (assumptions until verified):
      1. **Broadcast scope.** `Tracker` is global (one instance, metrics span all rooms), so anomalies
         are global — but `hub` fan-out is per-room. Confirm whether the hub has an all-clients path or
         whether a global briefing must iterate rooms. **This decides the Phase 3 delivery call.**
      2. **Frame typing.** Confirm how the frontend discriminates inbound frames (the `type` field on
         broadcast messages) so `briefing.ready` slots into the existing switch, not a new socket.
      3. **Startup wiring.** Confirm where `main.go` constructs `Tracker` + `Hub` so the detector
         goroutine and briefing service are constructed and injected the same way.
- [ ] ElevenLabs account + API key. Add `ELEVENLABS_API_KEY`, `ELEVENLABS_VOICE_ID`,
      `BRIEFING_ENABLED=false`, `BRIEFING_MONTHLY_CHAR_BUDGET` to `backend/.env.example` and Compose.
- [ ] Voice + model: default `eleven_flash_v2_5` (fast, ~half credit cost). Upgrade to
      `eleven_multilingual_v2` only if flash sounds flat in the demo.
- [ ] `backend/scripts/tts_smoke.go`: hardcoded string → mp3 on disk. Run once. Listen to it.

**Done when:** the three seam facts are written down from the actual code, and the smoke script
produces a playable mp3.

---

## Phase 1a — analytics.Detector (pure over a snapshot, tested, no I/O)

- [ ] `pkg/analytics/detector.go`: `Detector` holds the previous `Metrics` snapshot + last-fired
      timestamps for debounce. `Check(cur Metrics, now time.Time) []Anomaly`.
- [ ] `Anomaly` type: `Kind` (RateSpike, LatencyBreach, ConnectionSurge, TrafficDropout), `Scope`
      (global for v1), `Observed`, `Baseline`, `Severity` (Yellow/Red), `At`.
- [ ] Detection rules read straight off `Metrics` — all fields already exist:
      - **RateSpike** — `MessagesPerMinute` is 15 slots, oldest first. Baseline = mean of slots[0..13];
        current = slots[14]. Fire if `current > baseline*K` **and** `current > floor` (floor kills
        divide-by-noise on an idle server).
      - **LatencyBreach** — `LatencyP99Ms` crosses an absolute ceiling (Yellow/Red bands).
      - **ConnectionSurge** — `ActiveConnections` jump vs previous snapshot beyond a delta.
      - **TrafficDropout** — was non-zero last poll, now zero. A room going silent is a real ops event
        and it demos well.
- [ ] Debounce: at most one briefing per `(Kind, Scope)` per 5 min. An oscillating metric must not
      generate 40 audio files.

Don't: call ElevenLabs, hit the network, or read the clock implicitly.
Do: keep `Check` pure over (snapshot, now) so tests are millisecond-fast and CI never touches the network.

**Done when:** table-driven tests cover each Kind, both severity bands, the idle-floor guard, and the
debounce window. `go test ./pkg/analytics/ -race` green.

---

## Phase 1b — briefing.Composer (anomaly → script, pure)

- [ ] `pkg/briefing/composer.go`: `Compose(a analytics.Anomaly) string`. Control-room register,
      short declarative sentences. Example:
      `"Ops, analytics. Message-rate spike in the global room — four hundred per minute against a
      baseline of ninety. P99 latency holding at twelve milliseconds. Monitoring."`
- [ ] Numbers formatted for speech: "twelve milliseconds", not "12ms". Decide the rounding once, test it.
- [ ] Hard cap script at 600 chars (cost + attention); overflow ends with "Full detail on the dashboard."

Don't: put an LLM in this path.
Do: keep it a pure function; the script is the fallback artifact when TTS is unavailable.

**Done when:** one test per Kind; you've read 5 outputs aloud and they sound like a control room, not a changelog.

---

## Phase 2 — briefing.TTS + briefing.Store

- [ ] `pkg/briefing/tts.go`: `Synthesizer` interface (narrow, mockable) + an ElevenLabs impl over
      `net/http`. `Synthesize(ctx, script) (Audio, error)`.
- [ ] Content-hash cache: hash (script+voice+model); if `/data/briefings/{hash}.mp3` exists, skip the API.
- [ ] Retry once with backoff on 429/5xx. On final failure: `slog.Error`, keep the script as text-only
      record, return a sentinel — callers degrade, never crash.
- [ ] Monthly character budget counter (free tier ≈ 10k credits). Past budget → refuse synthesis,
      emit text-only briefing.
- [ ] `pkg/briefing/store.go`: in-memory ring buffer of the last N briefing records (mirror the
      `latencySamples` pattern in `tracker.go` — `sync.RWMutex`, cap, evict oldest). Audio on the volume.

Don't: persist briefing metadata to DynamoDB in v1, and do not have AI author any table schema/GSI —
the repo `CLAUDE.md` reserves schema work for hand-writing. `IMPORTANT:` AI-authored DynamoDB schemas
are a documented silent-failure trap in this repo; keep v1 in-memory and out of that trap entirely.
Do: serve briefings from the ring buffer; graceful-degradation parity with the existing analytics layer.

**Done when:** synthesize twice with identical input → one API call; network killed mid-call → text-only
record, no panic; `go test ./pkg/briefing/ -race` green with the Synthesizer mocked.

---

## Phase 3 — Wiring into the server

- [ ] In `main.go`, construct the briefing service + detector next to `Tracker`/`Hub`, inject as
      optional deps (nil when `BRIEFING_ENABLED=false`), same shape as `storage`/`analytics`.
- [ ] Detector goroutine: `time.Ticker` (5s). Each tick → `Detector.Check(tracker.GetMetrics(), now)`
      → for each anomaly: compose → synthesize → store → broadcast. Fire-and-forget per anomaly so a
      slow TTS call can't stall the ticker.
- [ ] Delivery: broadcast a `briefing.ready` frame through the hub per the Phase 0 scope decision
      (all-clients path vs iterate-rooms). Reuse the existing typed-frame mechanism; **no new socket.**
- [ ] `GET /api/briefings` (recent, from ring buffer) and `GET /api/briefings/{hash}/audio`
      (serve the file, `Content-Type: audio/mpeg`).

**IMPORTANT:** synthesis takes 1–3s. If a tick `await`s TTS inline, an anomaly burst backs up the
ticker and metric polling silently drifts late — no error anywhere. Dispatch each anomaly to its own
goroutine; the ticker only reads metrics and hands off.

**Done when:** a scripted message flood against a room produces a `briefing.ready` frame and a playable
audio URL within ~5s, and broadcast-latency percentiles in `/api/analytics` are unchanged under load.

---

## Phase 4 — Frontend audio surface

- [ ] Extend `useWebSocket.ts`: handle the `briefing.ready` frame type; keep a recent-briefings list.
- [ ] Briefing panel in `AnalyticsDashboard.tsx`: recent briefings with a play button and the script
      shown as a transcript (accessibility + demos read even with sound off).
- [ ] On `briefing.ready`: chime + audio **only if** a "voice enabled" toggle is on. Persist the toggle
      (localStorage), default off.

**IMPORTANT:** browsers block audio autoplay before a user gesture — `play()` rejects silently, no
console error. Gate autoplay behind the toggle click (which is the gesture) and render rejected `play()`
promises as a visible "click to enable audio" state.

**Done when:** cold-start in a fresh browser profile, one click on the toggle, and the next anomaly speaks.

---

## Phase 5 — Ops, CI, docs

- [ ] CI: detector + composer + store tests run with the Synthesizer mocked (interface swap, no httptest
      needed). `go test ./... -race`.
- [ ] **IMPORTANT:** CI must never reach `api.elevenlabs.io` — it burns credits silently and goes flaky
      on their rate limits. The mock is the default in tests; assert no real client is constructed under test.
- [ ] Compose: env vars wired, `/data/briefings` on the volume, healthcheck unaffected.
- [ ] README: feature blurb + the architecture diagram above + a 15s demo clip / GIF.
- [ ] `pkg/briefing/CLAUDE.md` (module-level, <20 lines): the invariants above, "scripts are templates,
      not an LLM," and "briefing storage is in-memory in v1 — no AI-authored DynamoDB schema."
- [ ] Update root `CLAUDE.md`: correct the stale Phase 2B status and add a Phase 3 (Voice Briefings) line.

**Done when:** CI green with no ElevenLabs egress; deployed to Hetzner via Coolify; demo clip recorded.

---

## Stretch (separate plan if pursued)

- Persist briefing metadata to DynamoDB — **hand-written** table schema + GSI per repo convention;
  the in-memory store stays as the read path / fallback.
- Per-room detection + per-room briefings once the global path is proven.
- Claude-composed briefings that correlate multiple simultaneous anomalies ("rate spike *and* latency
  breach together") — templates remain the fallback, treat LLM output as untrusted input, cap length
  before TTS.
- Scheduled "shift handover" digest briefing on a cron.

---

# As-built (2026-07-31)

Phases 0–5 are implemented. This section records how the delivered code differs from the
plan above, so the plan stays readable as the original intent rather than being rewritten.

## Where things landed

| Concern | File |
| --- | --- |
| Anomaly rules, severity, debounce | `backend/pkg/analytics/detector.go` |
| Script templates, number-to-speech | `backend/pkg/briefing/composer.go` |
| ElevenLabs client, content cache, budget | `backend/pkg/briefing/tts.go` |
| Ring buffer | `backend/pkg/briefing/store.go` |
| Pipeline (compose → synthesize → store) | `backend/pkg/briefing/briefing.go` |
| Ticker loop, `briefing.ready` frame | `backend/pkg/briefing/runner.go` |
| REST handlers | `backend/pkg/briefing/handler.go` |
| Global fan-out | `backend/pkg/hub/hub.go` (`BroadcastAll`) |
| Construction + routes + lifecycle | `backend/cmd/server/main.go` |
| Frame types, socket branch, panel | `frontend/src/{types/message.ts,hooks/useWebSocket.ts,hooks/useBriefings.ts,components/BriefingPanel.tsx}` |

The live wire contract is `docs/briefing-frame-contract.md`. Module-level invariants are in
`backend/pkg/briefing/CLAUDE.md`.

## Deviations from the plan

1. **`Record` emits `audioUrl`, not `audioPath`.** The original stub serialized a filesystem
   path under `audioPath` while the frame contract and the frontend both expected `audioUrl`.
   The record now carries a URL (`/api/briefings/{hash}/audio`); the on-disk location never
   reaches the browser. `TestRunner_FrameMatchesContract` asserts `audioPath` is absent.

2. **`setupRoutes()` kept its no-argument signature.** The Phase 3 wiring notes assumed a `cfg`
   field on `Server` and an `s.briefingStore()` helper, neither of which existed, and changing
   the signature would have rippled into six test call sites. `Server` holds `briefingStore`
   and `briefingAudioDir` instead.

3. **A Docker volume had to be created.** The plan described `/data/briefings` as living on an
   existing volume; `docker-compose.yml` declared no volumes at all. A named `briefing-audio`
   volume now backs it — without one, the content-hash cache is lost on every restart and the
   same briefings are re-billed.

4. **CI is net-new.** There was no `.github/` directory to extend. It initially had to split
   the race step around the pre-existing `TestClient_PingPong` race; that race was subsequently
   fixed (the test's ping counter written from the reader goroutine is now `atomic.Int64`),
   and CI runs a single `go test ./... -race` step.

5. **`Service.Handle` returns a nil error on synthesis failure.** A dead API or an exhausted
   budget is not a briefing failure — the script is the fallback artifact, so the record is
   stored text-only and broadcast anyway. Only an inability to produce a briefing at all would
   be worth surfacing as an error.

6. **Severity is derived, not passed in.** `Red` is used when a rate spike exceeds twice the
   multiplier, when P99 passes `LatencyRedMs`, or when a connection surge doubles the delta;
   otherwise `Yellow`. These are placeholder bands — tune them against real traffic.

7. **`speakInt` falls back to digits at one million.** A briefing that reads a nine-digit number
   aloud is worse than one that points at the dashboard.

## Verification status

- `gofmt`, `go vet` clean. All packages pass `-race`, including `pkg/client` after the
  `TestClient_PingPong` fix.
- Coverage: `pkg/briefing` 88.2%, `pkg/analytics` 91.2%.
- Frontend builds and lints clean (one pre-existing `exhaustive-deps` warning in `useWebSocket`).
- Live server checks: with the feature off, `/api/briefings` 404s and no detector goroutine
  starts; with it on, the route returns `{"briefings":[]}` and path-traversal attempts 404.

## Live verification (2026-07-31)

Run against the real ElevenLabs API on a free-tier key.

- **Phase 0 smoke test** — `tts_smoke.go` produced a 77 KB playable mp3. Endpoint, `xi-api-key`
  header and request body all confirmed against the live service.
- **Shipped client** — `ElevenLabs.Synthesize` via `Service.Handle`: first call 408 ms, second
  call on identical input 0 ms with one file on disk. The content cache genuinely prevents a
  second charge; this is the invariant that protects the credit budget.
- **End to end** — a 200-message flood produced a `briefing.ready` frame within one detector
  tick, `GET /api/briefings` returned the record, and `GET /api/briefings/{hash}/audio` served
  150 KB of `audio/mpeg`.
- **Billing** — 202 characters cost 101 credits, confirming `eleven_flash_v2_5` bills at half
  rate. Note `BRIEFING_MONTHLY_CHAR_BUDGET` counts *characters* while ElevenLabs meters
  *credits*, so on flash the guard is conservative by 2x. Intentional, but worth knowing before
  tuning it.

`RATE_LIMIT_PER_SEC` (default 5) throttles inbound messages, so a flood driven through a single
client cannot reach the `SpikeFloor` of 50 messages/minute. Any load test of the detector has to
raise or disable the limit, or spread traffic across many clients.

## Not yet done

- **Phase 4 browser gates** — the localStorage toggle and the rejected-autoplay state need a
  real browser; there is no frontend test runner.
- **Demo clip / deploy** — the Phase 5 acceptance gate.
- **CI has never executed** — `.github/` did not exist before this work. The workflow passes
  locally command-for-command, but its first real run is on the pushed commit.
