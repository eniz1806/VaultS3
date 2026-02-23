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
	store   *metadata.Store
	engine  storage.Engine
	metrics *metrics.Collector
	cfg     *config.Config
	jwt     *JWTService
}

func NewAPIHandler(store *metadata.Store, engine storage.Engine, mc *metrics.Collector, cfg *config.Config) *APIHandler {
	return &APIHandler{
		store:   store,
		engine:  engine,
		metrics: mc,
		cfg:     cfg,
		jwt:     NewJWTService(cfg.Auth.AdminSecretKey),
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

	// Stats route
	case path == "/stats" && r.Method == http.MethodGet:
		h.handleStats(w, r)

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
