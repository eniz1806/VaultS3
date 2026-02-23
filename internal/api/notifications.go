package api

import "net/http"

func (h *APIHandler) handleListNotifications(w http.ResponseWriter, r *http.Request) {
	configs, err := h.store.ListNotificationConfigs()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	type entry struct {
		Bucket  string      `json:"bucket"`
		Configs interface{} `json:"configs"`
	}

	var result []entry
	for bucket, cfg := range configs {
		result = append(result, entry{Bucket: bucket, Configs: cfg.Webhooks})
	}

	if result == nil {
		result = []entry{}
	}

	writeJSON(w, http.StatusOK, result)
}
