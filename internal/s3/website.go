package s3

import (
	"io"
	"mime"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
)

// serveWebsite handles static website requests for website-enabled buckets.
func (h *Handler) serveWebsite(w http.ResponseWriter, r *http.Request, bucket, key string) {
	cfg, err := h.store.GetWebsiteConfig(bucket)
	if err != nil {
		writeS3Error(w, "NoSuchWebsiteConfiguration", "No website configuration", http.StatusNotFound)
		return
	}

	// Resolve index document for root or directory paths
	resolvedKey := key
	if resolvedKey == "" || strings.HasSuffix(resolvedKey, "/") {
		resolvedKey += cfg.IndexDocument
	}

	// Try to serve the resolved object
	reader, size, err := h.engine.GetObject(bucket, resolvedKey)
	if err != nil {
		// Object not found — try error document
		if cfg.ErrorDocument != "" {
			h.serveErrorDocument(w, bucket, cfg.ErrorDocument)
			return
		}
		http.Error(w, "404 Not Found", http.StatusNotFound)
		return
	}
	defer reader.Close()

	// Set Content-Type from extension
	ct := mime.TypeByExtension(filepath.Ext(resolvedKey))
	if ct == "" {
		ct = "application/octet-stream"
	}
	w.Header().Set("Content-Type", ct)
	w.Header().Set("Content-Length", strconv.FormatInt(size, 10))
	w.WriteHeader(http.StatusOK)
	io.Copy(w, reader)
}

// serveErrorDocument serves the custom error document with 404 status.
func (h *Handler) serveErrorDocument(w http.ResponseWriter, bucket, errorDoc string) {
	reader, size, err := h.engine.GetObject(bucket, errorDoc)
	if err != nil {
		http.Error(w, "404 Not Found", http.StatusNotFound)
		return
	}
	defer reader.Close()

	ct := mime.TypeByExtension(filepath.Ext(errorDoc))
	if ct == "" {
		ct = "text/html"
	}
	w.Header().Set("Content-Type", ct)
	w.Header().Set("Content-Length", strconv.FormatInt(size, 10))
	w.WriteHeader(http.StatusNotFound)
	io.Copy(w, reader)
}
