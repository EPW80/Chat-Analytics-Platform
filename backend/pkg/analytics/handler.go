package analytics

import (
	"encoding/json"
	"net/http"
)

// NewHandler returns an HTTP handler that serves the current metrics snapshot.
func NewHandler(t *Tracker) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")

		metrics := t.GetMetrics()
		if err := json.NewEncoder(w).Encode(metrics); err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
		}
	}
}
