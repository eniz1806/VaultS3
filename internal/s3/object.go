package s3

import (
	"encoding/xml"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"
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

	written, etag, err := h.engine.PutObject(bucket, key, r.Body, r.ContentLength)
	if err != nil {
		writeS3Error(w, "InternalError", err.Error(), http.StatusInternalServerError)
		return
	}

	// Determine Content-Type
	ct := r.Header.Get("Content-Type")
	if ct == "" || ct == "application/octet-stream" {
		if detected := mime.TypeByExtension(filepath.Ext(key)); detected != "" {
			ct = detected
		} else {
			ct = "application/octet-stream"
		}
	}

	// Store object metadata
	h.store.PutObjectMeta(metadata.ObjectMeta{
		Bucket:       bucket,
		Key:          key,
		ContentType:  ct,
		ETag:         etag,
		Size:         written,
		LastModified: time.Now().UTC().Unix(),
	})

	w.Header().Set("ETag", etag)
	w.WriteHeader(http.StatusOK)
}

// GetObject handles GET /{bucket}/{key} with optional Range support.
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

	// Set Content-Type from metadata
	if meta, err := h.store.GetObjectMeta(bucket, key); err == nil {
		w.Header().Set("Content-Type", meta.ContentType)
		w.Header().Set("ETag", meta.ETag)
	}
	w.Header().Set("Accept-Ranges", "bytes")

	// Handle Range request
	rangeHeader := r.Header.Get("Range")
	if rangeHeader != "" {
		h.serveRange(w, reader, size, rangeHeader)
		return
	}

	w.Header().Set("Content-Length", strconv.FormatInt(size, 10))
	w.WriteHeader(http.StatusOK)
	io.Copy(w, reader)
}

// serveRange handles partial content responses.
func (h *ObjectHandler) serveRange(w http.ResponseWriter, reader storage.ReadSeekCloser, totalSize int64, rangeHeader string) {
	// Parse "bytes=START-END"
	if !strings.HasPrefix(rangeHeader, "bytes=") {
		writeS3Error(w, "InvalidRange", "Invalid Range header", http.StatusRequestedRangeNotSatisfiable)
		return
	}
	spec := strings.TrimPrefix(rangeHeader, "bytes=")
	parts := strings.SplitN(spec, "-", 2)
	if len(parts) != 2 {
		writeS3Error(w, "InvalidRange", "Invalid Range header", http.StatusRequestedRangeNotSatisfiable)
		return
	}

	var start, end int64

	if parts[0] == "" {
		// Suffix range: bytes=-500 (last 500 bytes)
		suffix, err := strconv.ParseInt(parts[1], 10, 64)
		if err != nil || suffix <= 0 {
			writeS3Error(w, "InvalidRange", "Invalid Range header", http.StatusRequestedRangeNotSatisfiable)
			return
		}
		start = totalSize - suffix
		if start < 0 {
			start = 0
		}
		end = totalSize - 1
	} else {
		var err error
		start, err = strconv.ParseInt(parts[0], 10, 64)
		if err != nil || start < 0 {
			writeS3Error(w, "InvalidRange", "Invalid Range header", http.StatusRequestedRangeNotSatisfiable)
			return
		}
		if parts[1] == "" {
			// Open-ended: bytes=500-
			end = totalSize - 1
		} else {
			end, err = strconv.ParseInt(parts[1], 10, 64)
			if err != nil {
				writeS3Error(w, "InvalidRange", "Invalid Range header", http.StatusRequestedRangeNotSatisfiable)
				return
			}
		}
	}

	if start > end || start >= totalSize {
		writeS3Error(w, "InvalidRange", "Range not satisfiable", http.StatusRequestedRangeNotSatisfiable)
		return
	}
	if end >= totalSize {
		end = totalSize - 1
	}

	length := end - start + 1

	if _, err := reader.Seek(start, io.SeekStart); err != nil {
		writeS3Error(w, "InternalError", "Seek failed", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, end, totalSize))
	w.Header().Set("Content-Length", strconv.FormatInt(length, 10))
	w.WriteHeader(http.StatusPartialContent)
	io.CopyN(w, reader, length)
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

	h.store.DeleteObjectMeta(bucket, key)
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

	if meta, err := h.store.GetObjectMeta(bucket, key); err == nil {
		w.Header().Set("Content-Type", meta.ContentType)
		w.Header().Set("ETag", meta.ETag)
	}
	w.Header().Set("Content-Length", strconv.FormatInt(size, 10))
	w.Header().Set("Accept-Ranges", "bytes")
	w.WriteHeader(http.StatusOK)
}

// CopyObject handles PUT /{bucket}/{key} with x-amz-copy-source header.
func (h *ObjectHandler) CopyObject(w http.ResponseWriter, r *http.Request, bucket, key string) {
	if !h.store.BucketExists(bucket) {
		writeS3Error(w, "NoSuchBucket", "Destination bucket does not exist", http.StatusNotFound)
		return
	}

	// Parse x-amz-copy-source: /source-bucket/source-key or source-bucket/source-key
	copySource := r.Header.Get("X-Amz-Copy-Source")
	copySource, _ = url.PathUnescape(copySource)
	copySource = strings.TrimPrefix(copySource, "/")

	srcBucket, srcKey := parseCopySource(copySource)
	if srcBucket == "" || srcKey == "" {
		writeS3Error(w, "InvalidArgument", "Invalid x-amz-copy-source", http.StatusBadRequest)
		return
	}

	if !h.store.BucketExists(srcBucket) {
		writeS3Error(w, "NoSuchBucket", "Source bucket does not exist", http.StatusNotFound)
		return
	}

	// Read source object
	reader, size, err := h.engine.GetObject(srcBucket, srcKey)
	if err != nil {
		writeS3Error(w, "NoSuchKey", "Source object not found", http.StatusNotFound)
		return
	}
	defer reader.Close()

	// Write to destination
	written, etag, err := h.engine.PutObject(bucket, key, reader, size)
	if err != nil {
		writeS3Error(w, "InternalError", err.Error(), http.StatusInternalServerError)
		return
	}

	now := time.Now().UTC()

	// Copy metadata from source, or detect fresh
	ct := "application/octet-stream"
	if srcMeta, err := h.store.GetObjectMeta(srcBucket, srcKey); err == nil {
		ct = srcMeta.ContentType
	}

	h.store.PutObjectMeta(metadata.ObjectMeta{
		Bucket:       bucket,
		Key:          key,
		ContentType:  ct,
		ETag:         etag,
		Size:         written,
		LastModified: now.Unix(),
	})

	type copyResult struct {
		XMLName      xml.Name `xml:"CopyObjectResult"`
		ETag         string   `xml:"ETag"`
		LastModified string   `xml:"LastModified"`
	}

	writeXML(w, http.StatusOK, copyResult{
		ETag:         etag,
		LastModified: now.Format(time.RFC3339),
	})
}

func parseCopySource(source string) (bucket, key string) {
	parts := strings.SplitN(source, "/", 2)
	if len(parts) != 2 {
		return "", ""
	}
	return parts[0], parts[1]
}

// BatchDelete handles POST /{bucket}?delete.
func (h *ObjectHandler) BatchDelete(w http.ResponseWriter, r *http.Request, bucket string) {
	if !h.store.BucketExists(bucket) {
		writeS3Error(w, "NoSuchBucket", "Bucket does not exist", http.StatusNotFound)
		return
	}

	var req deleteRequest
	if err := xml.NewDecoder(r.Body).Decode(&req); err != nil {
		writeS3Error(w, "MalformedXML", "Could not parse request body", http.StatusBadRequest)
		return
	}

	var result deleteResult
	for _, obj := range req.Objects {
		err := h.engine.DeleteObject(bucket, obj.Key)
		if err != nil {
			result.Errors = append(result.Errors, deleteError{
				Key:     obj.Key,
				Code:    "InternalError",
				Message: err.Error(),
			})
		} else {
			h.store.DeleteObjectMeta(bucket, obj.Key)
			if !req.Quiet {
				result.Deleted = append(result.Deleted, deletedObject{Key: obj.Key})
			}
		}
	}

	writeXML(w, http.StatusOK, result)
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
