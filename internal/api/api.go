package api

import (
	"net/http"
	"strings"

	"github.com/eniz1806/VaultS3/internal/config"
	"github.com/eniz1806/VaultS3/internal/metadata"
	"github.com/eniz1806/VaultS3/internal/metrics"
	"github.com/eniz1806/VaultS3/internal/storage"
)

// APIHandler serves the dashboard REST API at /api/v1/.
type APIHandler struct {
	store    *metadata.Store
	engine   storage.Engine
	metrics  *metrics.Collector
	cfg      *config.Config
	jwt      *JWTService
	activity *ActivityLog
}

func NewAPIHandler(store *metadata.Store, engine storage.Engine, mc *metrics.Collector, cfg *config.Config, activity *ActivityLog) *APIHandler {
	return &APIHandler{
		store:    store,
		engine:   engine,
		metrics:  mc,
		cfg:      cfg,
		jwt:      NewJWTService(cfg.Auth.AdminSecretKey),
		activity: activity,
	}
}

func (h *APIHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// CORS headers for dev mode (Vite proxy)
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	path := strings.TrimPrefix(r.URL.Path, "/api/v1")
	path = strings.TrimSuffix(path, "/")

	// Login does not require auth
	if path == "/auth/login" && r.Method == http.MethodPost {
		h.handleLogin(w, r)
		return
	}

	// All other routes require JWT
	if err := h.authenticate(r); err != nil {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	switch {
	case path == "/auth/me" && r.Method == http.MethodGet:
		h.handleMe(w, r)

	// Bucket routes
	case path == "/buckets" && r.Method == http.MethodGet:
		h.handleListBuckets(w, r)

	case path == "/buckets" && r.Method == http.MethodPost:
		h.handleCreateBucket(w, r)

	case strings.HasPrefix(path, "/buckets/"):
		h.routeBucket(w, r, strings.TrimPrefix(path, "/buckets/"))

	// Key management routes
	case path == "/keys" && r.Method == http.MethodGet:
		h.handleListKeys(w, r)

	case path == "/keys" && r.Method == http.MethodPost:
		h.handleCreateKey(w, r)

	case strings.HasPrefix(path, "/keys/") && r.Method == http.MethodDelete:
		accessKey := strings.TrimPrefix(path, "/keys/")
		h.handleDeleteKey(w, r, accessKey)

	// STS routes
	case path == "/sts/session-token" && r.Method == http.MethodPost:
		h.handleCreateSessionToken(w, r)

	// Audit trail route
	case path == "/audit" && r.Method == http.MethodGet:
		h.handleListAudit(w, r)

	// IAM User routes
	case path == "/iam/users" && r.Method == http.MethodGet:
		h.handleListIAMUsers(w, r)
	case path == "/iam/users" && r.Method == http.MethodPost:
		h.handleCreateIAMUser(w, r)
	case strings.HasPrefix(path, "/iam/users/"):
		h.routeIAMUser(w, r, strings.TrimPrefix(path, "/iam/users/"))

	// IAM Group routes
	case path == "/iam/groups" && r.Method == http.MethodGet:
		h.handleListIAMGroups(w, r)
	case path == "/iam/groups" && r.Method == http.MethodPost:
		h.handleCreateIAMGroup(w, r)
	case strings.HasPrefix(path, "/iam/groups/"):
		h.routeIAMGroup(w, r, strings.TrimPrefix(path, "/iam/groups/"))

	// IAM Policy routes
	case path == "/iam/policies" && r.Method == http.MethodGet:
		h.handleListIAMPolicies(w, r)
	case path == "/iam/policies" && r.Method == http.MethodPost:
		h.handleCreateIAMPolicy(w, r)
	case strings.HasPrefix(path, "/iam/policies/"):
		policyName := strings.TrimPrefix(path, "/iam/policies/")
		if r.Method == http.MethodDelete {
			h.handleDeleteIAMPolicy(w, r, policyName)
		} else {
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		}

	// Notification configs
	case path == "/notifications" && r.Method == http.MethodGet:
		h.handleListNotifications(w, r)

	// Replication routes
	case path == "/replication/status" && r.Method == http.MethodGet:
		h.handleReplicationStatus(w, r)
	case path == "/replication/queue" && r.Method == http.MethodGet:
		h.handleReplicationQueue(w, r)

	// Stats route
	case path == "/stats" && r.Method == http.MethodGet:
		h.handleStats(w, r)

	// Activity log route
	case path == "/activity" && r.Method == http.MethodGet:
		h.handleActivity(w, r)

	default:
		writeError(w, http.StatusNotFound, "not found")
	}
}

func (h *APIHandler) routeIAMUser(w http.ResponseWriter, r *http.Request, rest string) {
	parts := strings.SplitN(rest, "/", 2)
	userName := parts[0]

	if len(parts) == 1 {
		switch r.Method {
		case http.MethodGet:
			h.handleGetIAMUser(w, r, userName)
		case http.MethodDelete:
			h.handleDeleteIAMUser(w, r, userName)
		default:
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		}
		return
	}

	sub := parts[1]
	switch {
	case sub == "policies" && r.Method == http.MethodPost:
		h.handleAttachUserPolicy(w, r, userName)
	case strings.HasPrefix(sub, "policies/") && r.Method == http.MethodDelete:
		policyName := strings.TrimPrefix(sub, "policies/")
		h.handleDetachUserPolicy(w, r, userName, policyName)
	case sub == "groups" && r.Method == http.MethodPost:
		h.handleAddUserToGroup(w, r, userName)
	case strings.HasPrefix(sub, "groups/") && r.Method == http.MethodDelete:
		groupName := strings.TrimPrefix(sub, "groups/")
		h.handleRemoveUserFromGroup(w, r, userName, groupName)
	case sub == "ip-restrictions" && r.Method == http.MethodPut:
		h.handleSetIPRestrictions(w, r, userName)
	default:
		writeError(w, http.StatusNotFound, "not found")
	}
}

func (h *APIHandler) routeIAMGroup(w http.ResponseWriter, r *http.Request, rest string) {
	parts := strings.SplitN(rest, "/", 2)
	groupName := parts[0]

	if len(parts) == 1 {
		if r.Method == http.MethodDelete {
			h.handleDeleteIAMGroup(w, r, groupName)
		} else {
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		}
		return
	}

	sub := parts[1]
	switch {
	case sub == "policies" && r.Method == http.MethodPost:
		h.handleAttachGroupPolicy(w, r, groupName)
	case strings.HasPrefix(sub, "policies/") && r.Method == http.MethodDelete:
		policyName := strings.TrimPrefix(sub, "policies/")
		h.handleDetachGroupPolicy(w, r, groupName, policyName)
	default:
		writeError(w, http.StatusNotFound, "not found")
	}
}

func (h *APIHandler) routeBucket(w http.ResponseWriter, r *http.Request, rest string) {
	parts := strings.SplitN(rest, "/", 3)
	name := parts[0]

	if len(parts) == 1 {
		// /buckets/{name}
		switch r.Method {
		case http.MethodGet:
			h.handleGetBucket(w, r, name)
		case http.MethodDelete:
			h.handleDeleteBucket(w, r, name)
		default:
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		}
		return
	}

	sub := parts[1]
	keyRest := ""
	if len(parts) == 3 {
		keyRest = parts[2]
	}

	switch sub {
	case "policy":
		if r.Method == http.MethodPut {
			h.handlePutBucketPolicy(w, r, name)
		} else {
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		}
	case "quota":
		if r.Method == http.MethodPut {
			h.handlePutBucketQuota(w, r, name)
		} else {
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		}
	case "objects":
		if keyRest == "" {
			// /buckets/{name}/objects — list
			if r.Method == http.MethodGet {
				h.handleListObjects(w, r, name)
			} else {
				writeError(w, http.StatusMethodNotAllowed, "method not allowed")
			}
		} else {
			// /buckets/{name}/objects/{key...} — delete
			if r.Method == http.MethodDelete {
				h.handleDeleteObject(w, r, name, keyRest)
			} else {
				writeError(w, http.StatusMethodNotAllowed, "method not allowed")
			}
		}
	case "download":
		if keyRest != "" && r.Method == http.MethodGet {
			h.handleDownload(w, r, name, keyRest)
		} else {
			writeError(w, http.StatusNotFound, "not found")
		}
	case "upload":
		if r.Method == http.MethodPost {
			h.handleUpload(w, r, name)
		} else {
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		}
	default:
		writeError(w, http.StatusNotFound, "not found")
	}
}
