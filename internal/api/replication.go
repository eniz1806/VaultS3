package api

import (
	"net/http"
	"strconv"

	"github.com/eniz1806/VaultS3/internal/metadata"
)

func (h *APIHandler) handleReplicationStatus(w http.ResponseWriter, r *http.Request) {
	statuses, err := h.store.GetReplicationStatuses()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if statuses == nil {
		statuses = []metadata.ReplicationStatus{}
	}
	writeJSON(w, http.StatusOK, statuses)
}

func (h *APIHandler) handleReplicationQueue(w http.ResponseWriter, r *http.Request) {
	limit := 50
	if l := r.URL.Query().Get("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 {
			limit = parsed
		}
	}

	events, err := h.store.ListReplicationQueue(limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if events == nil {
		events = []metadata.ReplicationEvent{}
	}
	writeJSON(w, http.StatusOK, events)
}
