package api

import "net/http"

type notificationResponse struct {
	Bucket     string   `json:"bucket"`
	WebhookURL string   `json:"webhookURL"`
	Events     []string `json:"events"`
}

func (h *APIHandler) handleListNotifications(w http.ResponseWriter, r *http.Request) {
	configs, err := h.store.ListNotificationConfigs()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Flatten: one entry per webhook endpoint
	result := make([]notificationResponse, 0)
	for bucket, cfg := range configs {
		for _, wh := range cfg.Webhooks {
			result = append(result, notificationResponse{
				Bucket:     bucket,
				WebhookURL: wh.Endpoint,
				Events:     wh.Events,
			})
		}
	}

	writeJSON(w, http.StatusOK, result)
}
