package s3

import (
	"log"
	"net/http"
	"strings"

	"github.com/eniz1806/VaultS3/internal/metadata"
	"github.com/eniz1806/VaultS3/internal/storage"
)

// Handler routes incoming S3 API requests to the appropriate handler.
type Handler struct {
	store   *metadata.Store
	engine  storage.Engine
	auth    *Authenticator
	buckets *BucketHandler
	objects *ObjectHandler
}

func NewHandler(store *metadata.Store, engine storage.Engine, auth *Authenticator) *Handler {
	h := &Handler{
		store:  store,
		engine: engine,
		auth:   auth,
	}
	h.buckets = &BucketHandler{store: store, engine: engine}
	h.objects = &ObjectHandler{store: store, engine: engine}
	return h
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Authenticate
	if err := h.auth.Authenticate(r); err != nil {
		writeS3Error(w, "AccessDenied", err.Error(), http.StatusForbidden)
		return
	}

	// Parse bucket and key from path: /{bucket} or /{bucket}/{key...}
	path := strings.TrimPrefix(r.URL.Path, "/")
	bucket, key := parsePath(path)

	log.Printf("[S3] %s /%s/%s", r.Method, bucket, key)

	// Route based on path and method
	switch {
	case bucket == "":
		// Service-level operations (e.g., ListBuckets)
		if r.Method == http.MethodGet {
			h.buckets.ListBuckets(w, r)
			return
		}

	case key == "":
		// Bucket-level operations
		switch r.Method {
		case http.MethodPut:
			h.buckets.CreateBucket(w, r, bucket)
		case http.MethodDelete:
			h.buckets.DeleteBucket(w, r, bucket)
		case http.MethodHead:
			h.buckets.HeadBucket(w, r, bucket)
		case http.MethodGet:
			// GET on bucket = ListObjectsV2
			h.objects.ListObjects(w, r, bucket)
		default:
			writeS3Error(w, "MethodNotAllowed", "Method not allowed", http.StatusMethodNotAllowed)
		}

	default:
		// Object-level operations
		switch r.Method {
		case http.MethodPut:
			h.objects.PutObject(w, r, bucket, key)
		case http.MethodGet:
			h.objects.GetObject(w, r, bucket, key)
		case http.MethodDelete:
			h.objects.DeleteObject(w, r, bucket, key)
		case http.MethodHead:
			h.objects.HeadObject(w, r, bucket, key)
		default:
			writeS3Error(w, "MethodNotAllowed", "Method not allowed", http.StatusMethodNotAllowed)
		}
	}
}

func parsePath(path string) (bucket, key string) {
	if path == "" {
		return "", ""
	}
	parts := strings.SplitN(path, "/", 2)
	bucket = parts[0]
	if len(parts) > 1 {
		key = parts[1]
	}
	return
}
