package briefing

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Audio is synthesized speech plus the content hash used for caching/serving.
type Audio struct {
	Hash  string // SHA-256 of (script + voiceID + modelID)
	Bytes []byte // mp3
}

// ErrBudgetExhausted is returned when the monthly character budget is spent;
// callers degrade to a text-only Record rather than failing.
var ErrBudgetExhausted = errors.New("briefing: monthly character budget exhausted")

// apiBase is the ElevenLabs text-to-speech endpoint. Overridable per-client so
// tests can point at an httptest server and never reach the real API.
const apiBase = "https://api.elevenlabs.io/v1/text-to-speech/"

// Synthesizer is intentionally narrow so tests swap in Mock with no httptest.
type Synthesizer interface {
	Synthesize(ctx context.Context, script string) (Audio, error)
}

// ContentHash is the cache key: identical script, voice and model must never be
// billed twice. Exported so callers can derive a stable ID for a text-only
// briefing, where no synthesis happened.
func ContentHash(script, voiceID, modelID string) string {
	sum := sha256.Sum256([]byte(script + "\x00" + voiceID + "\x00" + modelID))
	return hex.EncodeToString(sum[:])
}

// ElevenLabs is the production Synthesizer (net/http). Content-hash cache lives
// here: if {AudioDir}/{hash}.mp3 exists, skip the API call.
type ElevenLabs struct {
	APIKey   string
	VoiceID  string
	ModelID  string // default "eleven_flash_v2_5"
	AudioDir string // "/data/briefings"
	HTTP     *http.Client
	BaseURL  string // defaults to apiBase; overridden in tests

	// CharBudget caps characters synthesized per billing month. <= 0 disables
	// the guard. The counter resets when the month rolls over.
	CharBudget int

	mu        sync.Mutex
	charsUsed int
	budgetFor string // "2026-07": the month charsUsed belongs to
}

// NewElevenLabs constructs the prod client with sane defaults.
func NewElevenLabs(apiKey, voiceID, audioDir string) *ElevenLabs {
	return &ElevenLabs{
		APIKey:   apiKey,
		VoiceID:  voiceID,
		ModelID:  "eleven_flash_v2_5",
		AudioDir: audioDir,
		HTTP:     &http.Client{Timeout: 15 * time.Second},
		BaseURL:  apiBase,
	}
}

// Synthesize hashes the input, checks the on-disk cache, and only then calls the
// API. Retries once with backoff on 429/5xx; returns the error on final failure
// so the caller keeps a text-only record.
func (e *ElevenLabs) Synthesize(ctx context.Context, script string) (Audio, error) {
	hash := ContentHash(script, e.VoiceID, e.ModelID)
	path := e.audioPath(hash)

	// Cache hit: the whole point of hashing. Never bill twice for equal input.
	if b, err := os.ReadFile(path); err == nil && len(b) > 0 {
		return Audio{Hash: hash, Bytes: b}, nil
	}

	// Reserve budget before the call, not after: a response that fails on the
	// way back still consumed credits upstream.
	if err := e.reserve(len(script), time.Now()); err != nil {
		return Audio{}, err
	}

	audio, err := e.post(ctx, script)
	if err != nil {
		return Audio{}, err
	}

	// A cache-write failure is not a synthesis failure — serving this briefing
	// matters more than caching the next one, so log-and-continue is the caller's
	// job, not a reason to discard good audio.
	_ = e.writeCache(path, audio)

	return Audio{Hash: hash, Bytes: audio}, nil
}

// post calls the API, retrying once on 429/5xx. Retries only cover transient
// classes: a 401 or 422 fails the same way twice and burns latency doing it.
func (e *ElevenLabs) post(ctx context.Context, script string) ([]byte, error) {
	const maxAttempts = 2
	var lastErr error

	for attempt := 0; attempt < maxAttempts; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(time.Second):
			}
		}

		audio, retryable, err := e.postOnce(ctx, script)
		if err == nil {
			return audio, nil
		}
		lastErr = err
		if !retryable {
			return nil, err
		}
	}
	return nil, lastErr
}

// ttsRequest is the ElevenLabs request body — a struct, not a map, so the wire
// shape stays typed per the repo's no-`any` convention.
type ttsRequest struct {
	Text    string `json:"text"`
	ModelID string `json:"model_id"`
}

func (e *ElevenLabs) postOnce(ctx context.Context, script string) (audio []byte, retryable bool, err error) {
	body, err := json.Marshal(ttsRequest{Text: script, ModelID: e.ModelID})
	if err != nil {
		return nil, false, fmt.Errorf("briefing: marshal request: %w", err)
	}

	base := e.BaseURL
	if base == "" {
		base = apiBase
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, base+e.VoiceID, bytes.NewReader(body))
	if err != nil {
		return nil, false, fmt.Errorf("briefing: build request: %w", err)
	}
	req.Header.Set("xi-api-key", e.APIKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "audio/mpeg")

	client := e.HTTP
	if client == nil {
		client = &http.Client{Timeout: 15 * time.Second}
	}
	resp, err := client.Do(req)
	if err != nil {
		// Transport failures (DNS, timeout, connection reset) earn one retry.
		return nil, true, fmt.Errorf("briefing: request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		msg, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		retry := resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500
		return nil, retry, fmt.Errorf("briefing: elevenlabs returned %d: %s", resp.StatusCode, bytes.TrimSpace(msg))
	}

	audio, err = io.ReadAll(resp.Body)
	if err != nil {
		return nil, true, fmt.Errorf("briefing: read audio: %w", err)
	}
	if len(audio) == 0 {
		return nil, true, errors.New("briefing: elevenlabs returned empty audio")
	}
	return audio, false, nil
}

// audioPath is where a hash's mp3 lives on disk.
func (e *ElevenLabs) audioPath(hash string) string {
	return filepath.Join(e.AudioDir, hash+".mp3")
}

// writeCache writes the mp3 atomically. A reader that caught a half-written file
// would cache a truncated briefing permanently — the hash would still match.
func (e *ElevenLabs) writeCache(path string, audio []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".briefing-*.mp3")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())

	if _, err := tmp.Write(audio); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}

// reserve claims `chars` against the monthly budget, rolling the counter over on
// a new month. Returns ErrBudgetExhausted when the claim would exceed it.
func (e *ElevenLabs) reserve(chars int, now time.Time) error {
	if e.CharBudget <= 0 {
		return nil
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	month := now.UTC().Format("2006-01")
	if e.budgetFor != month {
		e.budgetFor = month
		e.charsUsed = 0
	}
	if e.charsUsed+chars > e.CharBudget {
		return ErrBudgetExhausted
	}
	e.charsUsed += chars
	return nil
}

// Mock is the test Synthesizer: deterministic, offline, records calls so tests
// can assert the content-hash cache prevents a second synthesis of equal input.
type Mock struct {
	mu    sync.Mutex
	Calls []string // scripts seen, in order
	Err   error    // if set, returned instead of audio
}

func (m *Mock) Synthesize(_ context.Context, script string) (Audio, error) {
	m.mu.Lock()
	m.Calls = append(m.Calls, script)
	err := m.Err
	m.mu.Unlock()

	if err != nil {
		return Audio{}, err
	}
	return Audio{Hash: ContentHash(script, "mock", "mock"), Bytes: []byte("MOCKAUDIO")}, nil
}

// CallCount reports how many times Synthesize was called. Safe to read while the
// Service handles anomalies on their own goroutines.
func (m *Mock) CallCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.Calls)
}
