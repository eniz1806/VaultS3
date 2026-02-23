package s3

import (
	"log"
	"net/http"
	"strings"

	"github.com/eniz1806/VaultS3/internal/metadata"
	"github.com/eniz1806/VaultS3/internal/metrics"
	"github.com/eniz1806/VaultS3/internal/storage"
)

// ActivityFunc is a callback for recording S3 activity.
type ActivityFunc func(method, bucket, key string, status int, size int64, clientIP string)

// Handler routes incoming S3 API requests to the appropriate handler.
type Handler struct {
	store             *metadata.Store
	engine            storage.Engine
	auth              *Authenticator
	buckets           *BucketHandler
	objects           *ObjectHandler
	encryptionEnabled bool
	domain            string // base domain for virtual-hosted style URLs
	metrics           *metrics.Collector
	onActivity        ActivityFunc
}

func NewHandler(store *metadata.Store, engine storage.Engine, auth *Authenticator, encryptionEnabled bool, domain string, mc *metrics.Collector) *Handler {
	h := &Handler{
		store:             store,
		engine:            engine,
		auth:              auth,
		encryptionEnabled: encryptionEnabled,
		domain:            domain,
		metrics:           mc,
	}
	h.buckets = &BucketHandler{store: store, engine: engine}
	h.objects = &ObjectHandler{store: store, engine: engine, encryptionEnabled: encryptionEnabled}
	return h
}

// SetActivityFunc sets the callback for recording S3 activity.
func (h *Handler) SetActivityFunc(fn ActivityFunc) {
	h.onActivity = fn
}

// statusWriter wraps ResponseWriter to capture the status code.
type statusWriter struct {
	http.ResponseWriter
	status int
}

func (sw *statusWriter) WriteHeader(code int) {
	sw.status = code
	sw.ResponseWriter.WriteHeader(code)
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Parse bucket and key — support both path-style and virtual-hosted style
	path := strings.TrimPrefix(r.URL.Path, "/")
	bucket, key := h.parseRequest(r.Host, path)

	log.Printf("[S3] %s /%s/%s", r.Method, bucket, key)

	// Record request metrics
	if h.metrics != nil {
		h.metrics.RecordRequest(r.Method)
		if r.ContentLength > 0 {
			h.metrics.RecordBytesIn(r.ContentLength)
		}
	}

	// Wrap writer to capture status code for activity log
	sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}
	w = sw
	defer func() {
		if h.onActivity != nil && bucket != "" {
			clientIP := r.RemoteAddr
			if fwd := r.Header.Get("X-Forwarded-For"); fwd != "" {
				clientIP = strings.SplitN(fwd, ",", 2)[0]
			}
			h.onActivity(r.Method, bucket, key, sw.status, r.ContentLength, strings.TrimSpace(clientIP))
		}
	}()

	// Check for public-read policy bypass on GET/HEAD object requests
	authRequired := true
	if bucket != "" && (r.Method == http.MethodGet || r.Method == http.MethodHead) {
		if key != "" && h.store.IsBucketPublicRead(bucket) {
			authRequired = false
		}
		if h.store.IsBucketWebsite(bucket) {
			authRequired = false
		}
	}

	// Authenticate
	if authRequired {
		if err := h.auth.Authenticate(r); err != nil {
			writeS3Error(w, "AccessDenied", err.Error(), http.StatusForbidden)
			return
		}
	}

	// Static website serving — intercept before normal routing
	if bucket != "" && (r.Method == http.MethodGet || r.Method == http.MethodHead) {
		if h.store.IsBucketWebsite(bucket) {
			// Only serve website for non-API requests (no query params like ?policy, ?versioning, etc.)
			if len(r.URL.Query()) == 0 {
				h.serveWebsite(w, r, bucket, key)
				return
			}
		}
	}

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
		bq := r.URL.Query()

		// Policy operations
		if _, ok := bq["policy"]; ok {
			switch r.Method {
			case http.MethodPut:
				h.buckets.PutBucketPolicy(w, r, bucket)
			case http.MethodGet:
				h.buckets.GetBucketPolicy(w, r, bucket)
			case http.MethodDelete:
				h.buckets.DeleteBucketPolicy(w, r, bucket)
			default:
				writeS3Error(w, "MethodNotAllowed", "Method not allowed", http.StatusMethodNotAllowed)
			}
			return
		}

		// Website operations
		if _, ok := bq["website"]; ok {
			switch r.Method {
			case http.MethodPut:
				h.buckets.PutBucketWebsite(w, r, bucket)
			case http.MethodGet:
				h.buckets.GetBucketWebsite(w, r, bucket)
			case http.MethodDelete:
				h.buckets.DeleteBucketWebsite(w, r, bucket)
			default:
				writeS3Error(w, "MethodNotAllowed", "Method not allowed", http.StatusMethodNotAllowed)
			}
			return
		}

		// Lifecycle operations
		if _, ok := bq["lifecycle"]; ok {
			switch r.Method {
			case http.MethodPut:
				h.buckets.PutBucketLifecycle(w, r, bucket)
			case http.MethodGet:
				h.buckets.GetBucketLifecycle(w, r, bucket)
			case http.MethodDelete:
				h.buckets.DeleteBucketLifecycle(w, r, bucket)
			default:
				writeS3Error(w, "MethodNotAllowed", "Method not allowed", http.StatusMethodNotAllowed)
			}
			return
		}

		// Versioning operations
		if _, ok := bq["versioning"]; ok {
			switch r.Method {
			case http.MethodPut:
				h.buckets.PutBucketVersioning(w, r, bucket)
			case http.MethodGet:
				h.buckets.GetBucketVersioning(w, r, bucket)
			default:
				writeS3Error(w, "MethodNotAllowed", "Method not allowed", http.StatusMethodNotAllowed)
			}
			return
		}

		// List object versions
		if _, ok := bq["versions"]; ok {
			if r.Method == http.MethodGet {
				h.objects.ListObjectVersions(w, r, bucket)
			} else {
				writeS3Error(w, "MethodNotAllowed", "Method not allowed", http.StatusMethodNotAllowed)
			}
			return
		}

		// Quota operations
		if _, ok := bq["quota"]; ok {
			switch r.Method {
			case http.MethodPut:
				h.buckets.PutBucketQuota(w, r, bucket)
			case http.MethodGet:
				h.buckets.GetBucketQuota(w, r, bucket)
			default:
				writeS3Error(w, "MethodNotAllowed", "Method not allowed", http.StatusMethodNotAllowed)
			}
			return
		}

		switch r.Method {
		case http.MethodPut:
			h.buckets.CreateBucket(w, r, bucket)
		case http.MethodDelete:
			h.buckets.DeleteBucket(w, r, bucket)
		case http.MethodHead:
			h.buckets.HeadBucket(w, r, bucket)
		case http.MethodGet:
			h.objects.ListObjects(w, r, bucket)
		case http.MethodPost:
			if _, ok := bq["delete"]; ok {
				h.objects.BatchDelete(w, r, bucket)
			} else {
				writeS3Error(w, "MethodNotAllowed", "Method not allowed", http.StatusMethodNotAllowed)
			}
		default:
			writeS3Error(w, "MethodNotAllowed", "Method not allowed", http.StatusMethodNotAllowed)
		}

	default:
		// Object-level operations
		q := r.URL.Query()

		// Legal hold operations
		if _, ok := q["legal-hold"]; ok {
			switch r.Method {
			case http.MethodPut:
				h.objects.PutObjectLegalHold(w, r, bucket, key)
			case http.MethodGet:
				h.objects.GetObjectLegalHold(w, r, bucket, key)
			default:
				writeS3Error(w, "MethodNotAllowed", "Method not allowed", http.StatusMethodNotAllowed)
			}
			return
		}

		// Retention operations
		if _, ok := q["retention"]; ok {
			switch r.Method {
			case http.MethodPut:
				h.objects.PutObjectRetention(w, r, bucket, key)
			case http.MethodGet:
				h.objects.GetObjectRetention(w, r, bucket, key)
			default:
				writeS3Error(w, "MethodNotAllowed", "Method not allowed", http.StatusMethodNotAllowed)
			}
			return
		}

		// Check for tagging operations
		if _, ok := q["tagging"]; ok {
			switch r.Method {
			case http.MethodPut:
				h.objects.PutObjectTagging(w, r, bucket, key)
			case http.MethodGet:
				h.objects.GetObjectTagging(w, r, bucket, key)
			case http.MethodDelete:
				h.objects.DeleteObjectTagging(w, r, bucket, key)
			default:
				writeS3Error(w, "MethodNotAllowed", "Method not allowed", http.StatusMethodNotAllowed)
			}
			return
		}

		// Check for multipart upload operations
		if _, ok := q["uploads"]; ok {
			// POST /{bucket}/{key}?uploads = CreateMultipartUpload
			if r.Method == http.MethodPost {
				h.objects.CreateMultipartUpload(w, r, bucket, key)
				return
			}
		}
		if uploadID := q.Get("uploadId"); uploadID != "" {
			switch r.Method {
			case http.MethodPut:
				// PUT /{bucket}/{key}?partNumber=N&uploadId=X = UploadPart
				h.objects.UploadPart(w, r, bucket, key, uploadID)
			case http.MethodPost:
				// POST /{bucket}/{key}?uploadId=X = CompleteMultipartUpload
				h.objects.CompleteMultipartUpload(w, r, bucket, key, uploadID)
			case http.MethodDelete:
				// DELETE /{bucket}/{key}?uploadId=X = AbortMultipartUpload
				h.objects.AbortMultipartUpload(w, r, bucket, key, uploadID)
			default:
				writeS3Error(w, "MethodNotAllowed", "Method not allowed", http.StatusMethodNotAllowed)
			}
			return
		}

		switch r.Method {
		case http.MethodPut:
			if r.Header.Get("X-Amz-Copy-Source") != "" {
				h.objects.CopyObject(w, r, bucket, key)
			} else {
				h.objects.PutObject(w, r, bucket, key)
			}
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

// parseRequest extracts bucket and key from the request.
// Supports both virtual-hosted style (bucket.domain/key) and path-style (domain/bucket/key).
func (h *Handler) parseRequest(host, path string) (bucket, key string) {
	// Strip port from host
	if idx := strings.LastIndex(host, ":"); idx != -1 {
		host = host[:idx]
	}

	// Try virtual-hosted style if domain is configured
	if h.domain != "" && strings.HasSuffix(host, "."+h.domain) {
		bucket = strings.TrimSuffix(host, "."+h.domain)
		key = path
		return
	}

	// Fall back to path-style
	return parsePath(path)
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
