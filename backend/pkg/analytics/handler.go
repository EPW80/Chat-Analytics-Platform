package analytics

import (
	"encoding/json"
	"net/http"
)

// NewHandler returns an HTTP handler that serves the current metrics snapshot.
func NewHandler(t *Tracker) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		metrics := t.GetMetrics()
		payload, err := json.Marshal(metrics)
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(payload)
	}
}
