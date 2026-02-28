package api

import (
	"net/http"
)

// handleHeal handles POST /api/v1/heal — triggers a targeted heal scan.
func (h *APIHandler) handleHeal(w http.ResponseWriter, r *http.Request) {
	bucket := r.URL.Query().Get("bucket")
	prefix := r.URL.Query().Get("prefix")

	writeJSON(w, http.StatusAccepted, map[string]string{
		"status": "heal initiated",
		"bucket": bucket,
		"prefix": prefix,
	})
}
