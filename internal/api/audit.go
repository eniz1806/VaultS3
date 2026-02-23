package api

import (
	"net/http"
	"strconv"
	"time"
)

type auditResponse struct {
	Time       string `json:"time"`
	Principal  string `json:"principal"`
	UserID     string `json:"userId"`
	Action     string `json:"action"`
	Resource   string `json:"resource"`
	Effect     string `json:"effect"`
	SourceIP   string `json:"sourceIp"`
	StatusCode int    `json:"statusCode"`
}

func (h *APIHandler) handleListAudit(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	limit := 50
	if v := q.Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			limit = n
		}
	}
	if limit > 1000 {
		limit = 1000
	}

	var fromNano, toNano int64
	if v := q.Get("from"); v != "" {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			fromNano = t.UnixNano()
		}
	}
	if v := q.Get("to"); v != "" {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			toNano = t.UnixNano()
		}
	}

	user := q.Get("user")
	bucket := q.Get("bucket")

	entries, err := h.store.ListAuditEntries(limit, fromNano, toNano, user, bucket)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to query audit trail")
		return
	}

	items := make([]auditResponse, 0, len(entries))
	for _, e := range entries {
		items = append(items, auditResponse{
			Time:       time.Unix(0, e.Time).UTC().Format(time.RFC3339Nano),
			Principal:  e.Principal,
			UserID:     e.UserID,
			Action:     e.Action,
			Resource:   e.Resource,
			Effect:     e.Effect,
			SourceIP:   e.SourceIP,
			StatusCode: e.StatusCode,
		})
	}

	writeJSON(w, http.StatusOK, items)
}
