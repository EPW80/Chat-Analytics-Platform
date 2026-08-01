package briefing

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/epw80/chat-analytics-platform/pkg/analytics"
)

func TestListHandler(t *testing.T) {
	store := NewStore(10)
	for _, hash := range []string{"aaa", "bbb", "ccc"} {
		store.Add(Record{Hash: hash, Kind: analytics.RateSpike, Script: "script", AudioURL: audioURLFor(hash)})
	}

	t.Run("newest first", func(t *testing.T) {
		rec := httptest.NewRecorder()
		NewListHandler(store)(rec, httptest.NewRequest(http.MethodGet, "/api/briefings", nil))

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d", rec.Code)
		}
		if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
			t.Errorf("Content-Type = %q", ct)
		}

		var body listResponse
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			t.Fatal(err)
		}
		if len(body.Briefings) != 3 || body.Briefings[0].Hash != "ccc" {
			t.Errorf("got %v, want three briefings newest-first starting at ccc", body.Briefings)
		}
	})

	t.Run("respects limit", func(t *testing.T) {
		rec := httptest.NewRecorder()
		NewListHandler(store)(rec, httptest.NewRequest(http.MethodGet, "/api/briefings?limit=2", nil))

		var body listResponse
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			t.Fatal(err)
		}
		if len(body.Briefings) != 2 {
			t.Errorf("limit=2 returned %d briefings", len(body.Briefings))
		}
	})

	t.Run("empty store encodes an array not null", func(t *testing.T) {
		rec := httptest.NewRecorder()
		NewListHandler(NewStore(10))(rec, httptest.NewRequest(http.MethodGet, "/api/briefings", nil))

		// `null` would make the frontend's .map() throw.
		var body map[string]json.RawMessage
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			t.Fatal(err)
		}
		if got := string(body["briefings"]); got != "[]" {
			t.Errorf("briefings = %s, want []", got)
		}
	})
}

// audioRequest builds a request with the {hash} path value the mux would set.
func audioRequest(hash string) *http.Request {
	r := httptest.NewRequest(http.MethodGet, "/api/briefings/"+hash+"/audio", nil)
	r.SetPathValue("hash", hash)
	return r
}

func TestAudioHandler(t *testing.T) {
	dir := t.TempDir()
	hash := "abc123def456"
	if err := os.WriteFile(filepath.Join(dir, hash+".mp3"), []byte("ID3-FAKE-MP3"), 0o644); err != nil {
		t.Fatal(err)
	}

	store := NewStore(10)
	store.Add(Record{Hash: hash, AudioURL: audioURLFor(hash)})
	store.Add(Record{Hash: "textonly1", AudioURL: ""}) // TTS failed for this one

	t.Run("serves the mp3", func(t *testing.T) {
		rec := httptest.NewRecorder()
		NewAudioHandler(store, dir)(rec, audioRequest(hash))

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, body = %s", rec.Code, rec.Body)
		}
		if ct := rec.Header().Get("Content-Type"); ct != "audio/mpeg" {
			t.Errorf("Content-Type = %q, want audio/mpeg", ct)
		}
		if rec.Body.String() != "ID3-FAKE-MP3" {
			t.Errorf("body = %q", rec.Body)
		}
	})

	t.Run("404s", func(t *testing.T) {
		cases := map[string]string{
			"unknown hash":       "neverissued",
			"text-only briefing": "textonly1",
			"traversal attempt":  "../../etc/passwd",
			"absolute path":      "/etc/passwd",
		}
		for name, hash := range cases {
			t.Run(name, func(t *testing.T) {
				rec := httptest.NewRecorder()
				NewAudioHandler(store, dir)(rec, audioRequest(hash))
				if rec.Code != http.StatusNotFound {
					t.Errorf("status = %d, want 404", rec.Code)
				}
			})
		}
	})

	t.Run("missing hash is a 400", func(t *testing.T) {
		rec := httptest.NewRecorder()
		NewAudioHandler(store, dir)(rec, httptest.NewRequest(http.MethodGet, "/api/briefings//audio", nil))
		if rec.Code != http.StatusBadRequest {
			t.Errorf("status = %d, want 400", rec.Code)
		}
	})
}

// A hash that the store issued but whose file was lost (volume wiped, cache
// evicted) must 404 rather than serve an empty body as audio.
func TestAudioHandler_MissingFile(t *testing.T) {
	store := NewStore(10)
	store.Add(Record{Hash: "deadbeef", AudioURL: audioURLFor("deadbeef")})

	rec := httptest.NewRecorder()
	NewAudioHandler(store, t.TempDir())(rec, audioRequest("deadbeef"))

	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404 for a hash with no file on disk", rec.Code)
	}
}

func TestIsHexHash(t *testing.T) {
	valid := []string{"abc123", "0123456789abcdef", "deadbeef"}
	invalid := []string{"", "../etc", "ABC123", "abc-123", "abc.mp3", "g00d"}

	for _, s := range valid {
		if !isHexHash(s) {
			t.Errorf("isHexHash(%q) = false, want true", s)
		}
	}
	for _, s := range invalid {
		if isHexHash(s) {
			t.Errorf("isHexHash(%q) = true, want false", s)
		}
	}
}
