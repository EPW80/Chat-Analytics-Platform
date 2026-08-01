# Briefing Frame & API Contract

Pins the interface between Phase 3 (backend delivery) and Phase 4 (frontend audio).
Reconcile the `type` field against how existing frames are discriminated (Phase 0 question 2)
before implementing — this is the *proposed* shape, not a discovered one.

## WebSocket frame: `briefing.ready`

Broadcast through the existing hub fan-out when a briefing's audio is ready. Text-only
briefings (TTS failed or over budget) still emit the frame with `audioUrl` empty, so the
transcript shows even without sound.

```json
{
  "type": "briefing.ready",
  "briefing": {
    "hash": "3c556b79...",
    "kind": "rate_spike",
    "scope": "global",
    "severity": "yellow",
    "script": "Ops, analytics. Message-rate spike in the global room ...",
    "audioUrl": "/api/briefings/3c556b79.../audio",
    "createdAt": "2026-07-27T18:04:11Z"
  }
}
```

`kind` ∈ `rate_spike | latency_breach | connection_surge | traffic_dropout`
`severity` ∈ `yellow | red`
`audioUrl` — empty string for text-only briefings.

## REST

### `GET /api/briefings?limit=N`
Recent briefings, newest first, from the in-memory ring buffer. `limit` defaults to store cap.

```json
{ "briefings": [ /* Record objects, same shape as .briefing above */ ] }
```

### `GET /api/briefings/{hash}/audio`
Serves the mp3 from the volume. `Content-Type: audio/mpeg`. 404 if hash unknown or text-only.

## Frontend handling (Phase 4)

- `useWebSocket.ts`: on `briefing.ready`, prepend to a recent-briefings list.
- Play audio only if the "voice enabled" toggle is on (localStorage, default off).
- **Autoplay:** browsers reject `play()` before a user gesture, silently. Gate autoplay behind
  the toggle click (the gesture) and render a rejected `play()` promise as a visible
  "click to enable audio" state — never swallow it.
