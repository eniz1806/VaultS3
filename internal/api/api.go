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

	case path == "/buckets" && r.Method == http.MethodGet:
		h.handleListBuckets(w, r)

	case path == "/buckets" && r.Method == http.MethodPost:
		h.handleCreateBucket(w, r)

	case strings.HasPrefix(path, "/buckets/"):
		rest := strings.TrimPrefix(path, "/buckets/")
		parts := strings.SplitN(rest, "/", 2)
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
		} else {
			// /buckets/{name}/policy or /buckets/{name}/quota
			switch parts[1] {
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
			default:
				writeError(w, http.StatusNotFound, "not found")
			}
		}

	default:
		writeError(w, http.StatusNotFound, "not found")
	}
}
