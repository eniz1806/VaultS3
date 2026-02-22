package s3

import (
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/eniz1806/VaultS3/internal/metadata"
	"github.com/eniz1806/VaultS3/internal/storage"
)

type ObjectHandler struct {
	store  *metadata.Store
	engine storage.Engine
}

// PutObject handles PUT /{bucket}/{key}.
func (h *ObjectHandler) PutObject(w http.ResponseWriter, r *http.Request, bucket, key string) {
	if !h.store.BucketExists(bucket) {
		writeS3Error(w, "NoSuchBucket", "Bucket does not exist", http.StatusNotFound)
		return
	}

	written, err := h.engine.PutObject(bucket, key, r.Body, r.ContentLength)
	if err != nil {
		writeS3Error(w, "InternalError", err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("ETag", fmt.Sprintf("\"%x\"", written))
	w.WriteHeader(http.StatusOK)
}

// GetObject handles GET /{bucket}/{key}.
func (h *ObjectHandler) GetObject(w http.ResponseWriter, r *http.Request, bucket, key string) {
	if !h.store.BucketExists(bucket) {
		writeS3Error(w, "NoSuchBucket", "Bucket does not exist", http.StatusNotFound)
		return
	}

	reader, size, err := h.engine.GetObject(bucket, key)
	if err != nil {
		writeS3Error(w, "NoSuchKey", "Object not found", http.StatusNotFound)
		return
	}
	defer reader.Close()

	w.Header().Set("Content-Length", strconv.FormatInt(size, 10))
	w.Header().Set("Accept-Ranges", "bytes")
	w.WriteHeader(http.StatusOK)

	io.Copy(w, reader)
}

// DeleteObject handles DELETE /{bucket}/{key}.
func (h *ObjectHandler) DeleteObject(w http.ResponseWriter, r *http.Request, bucket, key string) {
	if !h.store.BucketExists(bucket) {
		writeS3Error(w, "NoSuchBucket", "Bucket does not exist", http.StatusNotFound)
		return
	}

	if err := h.engine.DeleteObject(bucket, key); err != nil {
		writeS3Error(w, "InternalError", err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// HeadObject handles HEAD /{bucket}/{key}.
func (h *ObjectHandler) HeadObject(w http.ResponseWriter, r *http.Request, bucket, key string) {
	if !h.store.BucketExists(bucket) {
		writeS3Error(w, "NoSuchBucket", "Bucket does not exist", http.StatusNotFound)
		return
	}

	if !h.engine.ObjectExists(bucket, key) {
		writeS3Error(w, "NoSuchKey", "Object not found", http.StatusNotFound)
		return
	}

	size, err := h.engine.ObjectSize(bucket, key)
	if err != nil {
		writeS3Error(w, "InternalError", err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Length", strconv.FormatInt(size, 10))
	w.WriteHeader(http.StatusOK)
}

// ListObjects handles GET /{bucket}?list-type=2.
func (h *ObjectHandler) ListObjects(w http.ResponseWriter, r *http.Request, bucket string) {
	if !h.store.BucketExists(bucket) {
		writeS3Error(w, "NoSuchBucket", "Bucket does not exist", http.StatusNotFound)
		return
	}

	prefix := r.URL.Query().Get("prefix")
	startAfter := r.URL.Query().Get("start-after")
	maxKeysStr := r.URL.Query().Get("max-keys")
	maxKeys := 1000
	if maxKeysStr != "" {
		if mk, err := strconv.Atoi(maxKeysStr); err == nil && mk > 0 {
			maxKeys = mk
		}
	}

	objects, truncated, err := h.engine.ListObjects(bucket, prefix, startAfter, maxKeys)
	if err != nil {
		writeS3Error(w, "InternalError", err.Error(), http.StatusInternalServerError)
		return
	}

	type xmlContent struct {
		Key          string `xml:"Key"`
		LastModified string `xml:"LastModified"`
		ETag         string `xml:"ETag"`
		Size         int64  `xml:"Size"`
		StorageClass string `xml:"StorageClass"`
	}
	type xmlResponse struct {
		XMLName     xml.Name     `xml:"ListBucketResult"`
		Xmlns       string       `xml:"xmlns,attr"`
		Name        string       `xml:"Name"`
		Prefix      string       `xml:"Prefix"`
		MaxKeys     int          `xml:"MaxKeys"`
		IsTruncated bool         `xml:"IsTruncated"`
		Contents    []xmlContent `xml:"Contents"`
		KeyCount    int          `xml:"KeyCount"`
	}

	resp := xmlResponse{
		Xmlns:       "http://s3.amazonaws.com/doc/2006-03-01/",
		Name:        bucket,
		Prefix:      prefix,
		MaxKeys:     maxKeys,
		IsTruncated: truncated,
		KeyCount:    len(objects),
	}

	for _, obj := range objects {
		resp.Contents = append(resp.Contents, xmlContent{
			Key:          obj.Key,
			LastModified: time.Unix(obj.LastModified, 0).UTC().Format(time.RFC3339),
			ETag:         obj.ETag,
			Size:         obj.Size,
			StorageClass: "STANDARD",
		})
	}

	writeXML(w, http.StatusOK, resp)
}
