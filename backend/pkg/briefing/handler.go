package briefing

import (
	"encoding/json"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
)

// listResponse is the GET /api/briefings body. A named type rather than a map so
// the wire shape matches docs/briefing-frame-contract.md exactly.
type listResponse struct {
	Briefings []Record `json:"briefings"`
}

// NewListHandler serves recent briefings, newest first.
// Route: GET /api/briefings?limit=N   (registered in main.go setupRoutes).
func NewListHandler(store *Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		n := 0
		if v := r.URL.Query().Get("limit"); v != "" {
			if parsed, err := strconv.Atoi(v); err == nil && parsed > 0 {
				n = parsed
			}
		}

		// Recent returns nil when empty; the contract promises an array.
		recent := store.Recent(n)
		if recent == nil {
			recent = []Record{}
		}

		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
		_ = json.NewEncoder(w).Encode(listResponse{Briefings: recent})
	}
}

// NewAudioHandler serves a briefing's mp3 from the audio dir.
// Route: GET /api/briefings/{hash}/audio
func NewAudioHandler(store *Store, audioDir string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		hash := r.PathValue("hash")
		if hash == "" {
			http.Error(w, "hash required", http.StatusBadRequest)
			return
		}

		// The hash must exist in the store before it names a file. This is the
		// real traversal guard: a hash the store never issued is a 404, so an
		// attacker cannot steer the path at all.
		rec, ok := store.ByHash(hash)
		if !ok || rec.AudioURL == "" {
			http.NotFound(w, r) // unknown or text-only briefing
			return
		}
		if !isHexHash(hash) {
			http.NotFound(w, r)
			return
		}

		// Belt and braces: resolve and confirm the result is still under audioDir.
		path := filepath.Join(audioDir, hash+".mp3")
		if !strings.HasPrefix(path, filepath.Clean(audioDir)+string(filepath.Separator)) {
			http.NotFound(w, r)
			return
		}

		w.Header().Set("Content-Type", "audio/mpeg")
		http.ServeFile(w, r, path)
	}
}

// isHexHash reports whether s is a plausible content hash — hex only, so it can
// never contain a separator or a dot segment.
func isHexHash(s string) bool {
	if len(s) == 0 || len(s) > 64 {
		return false
	}
	for _, r := range s {
		isDigit := r >= '0' && r <= '9'
		isLowerHex := r >= 'a' && r <= 'f'
		if !isDigit && !isLowerHex {
			return false
		}
	}
	return true
}
