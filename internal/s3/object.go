package s3

import (
	"crypto/rand"
	"encoding/hex"
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
	store             *metadata.Store
	engine            storage.Engine
	encryptionEnabled bool
	onNotification    NotificationFunc
	onReplication     ReplicationFunc
}

// checkQuota verifies bucket quota limits before writing.
func (h *ObjectHandler) checkQuota(w http.ResponseWriter, bucket string, incomingSize int64) bool {
	info, err := h.store.GetBucket(bucket)
	if err != nil {
		return true // no bucket info, allow
	}
	if info.MaxSizeBytes == 0 && info.MaxObjects == 0 {
		return true // no limits
	}

	currentSize, currentCount, _ := h.engine.BucketSize(bucket)

	if info.MaxObjects > 0 && currentCount >= info.MaxObjects {
		writeS3Error(w, "QuotaExceeded", "Maximum object count exceeded", http.StatusForbidden)
		return false
	}
	if info.MaxSizeBytes > 0 && incomingSize > 0 && currentSize+incomingSize > info.MaxSizeBytes {
		writeS3Error(w, "QuotaExceeded", "Maximum bucket size exceeded", http.StatusForbidden)
		return false
	}

	return true
}

// generateVersionID creates a unique version ID using timestamp + random bytes.
func generateVersionID() string {
	b := make([]byte, 8)
	rand.Read(b)
	return fmt.Sprintf("%016x%s", time.Now().UnixNano(), hex.EncodeToString(b[:4]))
}

// detectContentType determines the content type for an object.
func detectContentType(r *http.Request, key string) string {
	ct := r.Header.Get("Content-Type")
	if ct == "" || ct == "application/octet-stream" {
		if detected := mime.TypeByExtension(filepath.Ext(key)); detected != "" {
			return detected
		}
		return "application/octet-stream"
	}
	return ct
}

// PutObject handles PUT /{bucket}/{key}.
func (h *ObjectHandler) PutObject(w http.ResponseWriter, r *http.Request, bucket, key string) {
	if !h.store.BucketExists(bucket) {
		writeS3Error(w, "NoSuchBucket", "Bucket does not exist", http.StatusNotFound)
		return
	}

	if !h.checkQuota(w, bucket, r.ContentLength) {
		return
	}

	versioning, _ := h.store.GetBucketVersioning(bucket)
	ct := detectContentType(r, key)
	now := time.Now().UTC()

	if versioning == "Enabled" {
		versionID := generateVersionID()

		written, etag, err := h.engine.PutObjectVersion(bucket, key, versionID, r.Body, r.ContentLength)
		if err != nil {
			writeS3Error(w, "InternalError", err.Error(), http.StatusInternalServerError)
			return
		}

		// Mark previous latest as not latest
		if oldMeta, err := h.store.GetObjectMeta(bucket, key); err == nil && oldMeta.VersionID != "" {
			oldMeta.IsLatest = false
			h.store.PutObjectVersion(*oldMeta)
		}

		meta := metadata.ObjectMeta{
			Bucket:       bucket,
			Key:          key,
			ContentType:  ct,
			ETag:         etag,
			Size:         written,
			LastModified: now.Unix(),
			VersionID:    versionID,
			IsLatest:     true,
		}

		h.store.PutObjectVersion(meta)
		h.store.PutObjectMeta(meta) // update "latest pointer"

		w.Header().Set("ETag", etag)
		w.Header().Set("X-Amz-Version-Id", versionID)
		if h.encryptionEnabled {
			w.Header().Set("X-Amz-Server-Side-Encryption", "AES256")
		}
		w.WriteHeader(http.StatusOK)
		if h.onNotification != nil {
			h.onNotification("s3:ObjectCreated:Put", bucket, key, written, etag, versionID)
		}
		if h.onReplication != nil {
			h.onReplication("s3:ObjectCreated:Put", bucket, key, written, etag, versionID)
		}
		return
	}

	// Non-versioned path (unchanged behavior)
	written, etag, err := h.engine.PutObject(bucket, key, r.Body, r.ContentLength)
	if err != nil {
		writeS3Error(w, "InternalError", err.Error(), http.StatusInternalServerError)
		return
	}

	h.store.PutObjectMeta(metadata.ObjectMeta{
		Bucket:       bucket,
		Key:          key,
		ContentType:  ct,
		ETag:         etag,
		Size:         written,
		LastModified: now.Unix(),
	})

	w.Header().Set("ETag", etag)
	if h.encryptionEnabled {
		w.Header().Set("X-Amz-Server-Side-Encryption", "AES256")
	}
	w.WriteHeader(http.StatusOK)
	if h.onNotification != nil {
		h.onNotification("s3:ObjectCreated:Put", bucket, key, written, etag, "")
	}
	if h.onReplication != nil {
		h.onReplication("s3:ObjectCreated:Put", bucket, key, written, etag, "")
	}
}

// GetObject handles GET /{bucket}/{key} with optional Range support and ?versionId.
func (h *ObjectHandler) GetObject(w http.ResponseWriter, r *http.Request, bucket, key string) {
	if !h.store.BucketExists(bucket) {
		writeS3Error(w, "NoSuchBucket", "Bucket does not exist", http.StatusNotFound)
		return
	}

	versionID := r.URL.Query().Get("versionId")

	var reader storage.ReadSeekCloser
	var size int64
	var meta *metadata.ObjectMeta
	var err error

	if versionID != "" {
		// Get specific version
		meta, err = h.store.GetObjectVersion(bucket, key, versionID)
		if err != nil {
			writeS3Error(w, "NoSuchVersion", "Version not found", http.StatusNotFound)
			return
		}
		if meta.DeleteMarker {
			w.Header().Set("X-Amz-Delete-Marker", "true")
			w.Header().Set("X-Amz-Version-Id", versionID)
			writeS3Error(w, "NoSuchKey", "Object is a delete marker", http.StatusNotFound)
			return
		}
		reader, size, err = h.engine.GetObjectVersion(bucket, key, versionID)
		if err != nil {
			writeS3Error(w, "NoSuchKey", "Object not found", http.StatusNotFound)
			return
		}
		w.Header().Set("X-Amz-Version-Id", versionID)
	} else {
		// Get latest version
		meta, _ = h.store.GetObjectMeta(bucket, key)
		if meta != nil && meta.DeleteMarker {
			w.Header().Set("X-Amz-Delete-Marker", "true")
			if meta.VersionID != "" {
				w.Header().Set("X-Amz-Version-Id", meta.VersionID)
			}
			writeS3Error(w, "NoSuchKey", "Object not found", http.StatusNotFound)
			return
		}

		if meta != nil && meta.VersionID != "" {
			// Versioned bucket — read from version storage
			reader, size, err = h.engine.GetObjectVersion(bucket, key, meta.VersionID)
			w.Header().Set("X-Amz-Version-Id", meta.VersionID)
		} else {
			reader, size, err = h.engine.GetObject(bucket, key)
		}
		if err != nil {
			writeS3Error(w, "NoSuchKey", "Object not found", http.StatusNotFound)
			return
		}
	}
	defer reader.Close()

	if meta != nil {
		w.Header().Set("Content-Type", meta.ContentType)
		w.Header().Set("ETag", meta.ETag)
	}
	w.Header().Set("Accept-Ranges", "bytes")
	if h.encryptionEnabled {
		w.Header().Set("X-Amz-Server-Side-Encryption", "AES256")
	}

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

	versionID := r.URL.Query().Get("versionId")
	versioning, _ := h.store.GetBucketVersioning(bucket)

	if versionID != "" {
		// Delete specific version permanently
		// Check object lock first
		if err := h.checkObjectLock(bucket, key, versionID); err != nil {
			writeS3Error(w, "AccessDenied", err.Error(), http.StatusForbidden)
			return
		}

		h.engine.DeleteObjectVersion(bucket, key, versionID)
		h.store.DeleteObjectVersion(bucket, key, versionID)

		// If we deleted the latest, find the new latest
		versions, _, _ := h.store.ListObjectVersions(bucket, key, "", "", 1)
		if len(versions) > 0 {
			// There's still a version — make it latest
			versions[0].IsLatest = true
			h.store.UpdateObjectVersionMeta(versions[0])
		} else {
			// No versions left — remove from objects bucket
			h.store.DeleteObjectMeta(bucket, key)
		}

		w.Header().Set("X-Amz-Version-Id", versionID)
		w.WriteHeader(http.StatusNoContent)
		if h.onNotification != nil {
			h.onNotification("s3:ObjectRemoved:Delete", bucket, key, 0, "", versionID)
		}
		if h.onReplication != nil {
			h.onReplication("s3:ObjectRemoved:Delete", bucket, key, 0, "", versionID)
		}
		return
	}

	if versioning == "Enabled" {
		// Create a delete marker instead of actually deleting
		dmVersionID := generateVersionID()

		// Mark previous latest as not latest
		if oldMeta, err := h.store.GetObjectMeta(bucket, key); err == nil && oldMeta.VersionID != "" {
			oldMeta.IsLatest = false
			h.store.PutObjectVersion(*oldMeta)
		}

		dm := metadata.ObjectMeta{
			Bucket:       bucket,
			Key:          key,
			VersionID:    dmVersionID,
			IsLatest:     true,
			DeleteMarker: true,
			LastModified: time.Now().UTC().Unix(),
		}
		h.store.PutObjectVersion(dm)
		h.store.PutObjectMeta(dm) // latest pointer now points to delete marker

		w.Header().Set("X-Amz-Delete-Marker", "true")
		w.Header().Set("X-Amz-Version-Id", dmVersionID)
		w.WriteHeader(http.StatusNoContent)
		if h.onNotification != nil {
			h.onNotification("s3:ObjectRemoved:Delete", bucket, key, 0, "", dmVersionID)
		}
		if h.onReplication != nil {
			h.onReplication("s3:ObjectRemoved:Delete", bucket, key, 0, "", dmVersionID)
		}
		return
	}

	// Non-versioned: delete normally
	if err := h.engine.DeleteObject(bucket, key); err != nil {
		writeS3Error(w, "InternalError", err.Error(), http.StatusInternalServerError)
		return
	}

	h.store.DeleteObjectMeta(bucket, key)
	w.WriteHeader(http.StatusNoContent)
	if h.onNotification != nil {
		h.onNotification("s3:ObjectRemoved:Delete", bucket, key, 0, "", "")
	}
	if h.onReplication != nil {
		h.onReplication("s3:ObjectRemoved:Delete", bucket, key, 0, "", "")
	}
}

// checkObjectLock checks if an object version is locked (legal hold or retention).
func (h *ObjectHandler) checkObjectLock(bucket, key, versionID string) error {
	meta, err := h.store.GetObjectVersion(bucket, key, versionID)
	if err != nil {
		return nil // version doesn't exist in metadata, allow delete
	}

	if meta.LegalHold {
		return fmt.Errorf("object is under legal hold")
	}

	if meta.RetentionMode != "" && meta.RetentionUntil > 0 {
		if time.Now().UTC().Unix() < meta.RetentionUntil {
			return fmt.Errorf("object is under %s retention until %s",
				meta.RetentionMode,
				time.Unix(meta.RetentionUntil, 0).UTC().Format(time.RFC3339))
		}
	}

	return nil
}

// HeadObject handles HEAD /{bucket}/{key}.
func (h *ObjectHandler) HeadObject(w http.ResponseWriter, r *http.Request, bucket, key string) {
	if !h.store.BucketExists(bucket) {
		writeS3Error(w, "NoSuchBucket", "Bucket does not exist", http.StatusNotFound)
		return
	}

	versionID := r.URL.Query().Get("versionId")

	var meta *metadata.ObjectMeta

	if versionID != "" {
		var err error
		meta, err = h.store.GetObjectVersion(bucket, key, versionID)
		if err != nil {
			writeS3Error(w, "NoSuchVersion", "Version not found", http.StatusNotFound)
			return
		}
		if meta.DeleteMarker {
			w.Header().Set("X-Amz-Delete-Marker", "true")
			w.Header().Set("X-Amz-Version-Id", versionID)
			writeS3Error(w, "NoSuchKey", "Object is a delete marker", http.StatusNotFound)
			return
		}
		w.Header().Set("X-Amz-Version-Id", versionID)
	} else {
		meta, _ = h.store.GetObjectMeta(bucket, key)
		if meta != nil && meta.DeleteMarker {
			w.Header().Set("X-Amz-Delete-Marker", "true")
			if meta.VersionID != "" {
				w.Header().Set("X-Amz-Version-Id", meta.VersionID)
			}
			writeS3Error(w, "NoSuchKey", "Object not found", http.StatusNotFound)
			return
		}
		if meta == nil {
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
			w.Header().Set("Accept-Ranges", "bytes")
			w.WriteHeader(http.StatusOK)
			return
		}
		if meta.VersionID != "" {
			w.Header().Set("X-Amz-Version-Id", meta.VersionID)
		}
	}

	w.Header().Set("Content-Type", meta.ContentType)
	w.Header().Set("ETag", meta.ETag)
	w.Header().Set("Content-Length", strconv.FormatInt(meta.Size, 10))
	w.Header().Set("Accept-Ranges", "bytes")
	if h.encryptionEnabled {
		w.Header().Set("X-Amz-Server-Side-Encryption", "AES256")
	}
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
	if h.onNotification != nil {
		h.onNotification("s3:ObjectCreated:Copy", bucket, key, written, etag, "")
	}
	if h.onReplication != nil {
		h.onReplication("s3:ObjectCreated:Copy", bucket, key, written, etag, "")
	}
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
			if h.onNotification != nil {
				h.onNotification("s3:ObjectRemoved:Delete", bucket, obj.Key, 0, "", "")
			}
			if h.onReplication != nil {
				h.onReplication("s3:ObjectRemoved:Delete", bucket, obj.Key, 0, "", "")
			}
		}
	}

	writeXML(w, http.StatusOK, result)
}

// PutObjectTagging handles PUT /{bucket}/{key}?tagging.
func (h *ObjectHandler) PutObjectTagging(w http.ResponseWriter, r *http.Request, bucket, key string) {
	if !h.store.BucketExists(bucket) {
		writeS3Error(w, "NoSuchBucket", "Bucket does not exist", http.StatusNotFound)
		return
	}
	if !h.engine.ObjectExists(bucket, key) {
		writeS3Error(w, "NoSuchKey", "Object not found", http.StatusNotFound)
		return
	}

	var req taggingRequest
	if err := xml.NewDecoder(r.Body).Decode(&req); err != nil {
		writeS3Error(w, "MalformedXML", "Could not parse tagging XML", http.StatusBadRequest)
		return
	}

	if len(req.TagSet.Tags) > 10 {
		writeS3Error(w, "BadRequest", "Object tags cannot be greater than 10", http.StatusBadRequest)
		return
	}

	meta, err := h.store.GetObjectMeta(bucket, key)
	if err != nil {
		writeS3Error(w, "InternalError", err.Error(), http.StatusInternalServerError)
		return
	}

	meta.Tags = make(map[string]string, len(req.TagSet.Tags))
	for _, tag := range req.TagSet.Tags {
		meta.Tags[tag.Key] = tag.Value
	}

	if err := h.store.PutObjectMeta(*meta); err != nil {
		writeS3Error(w, "InternalError", err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

// GetObjectTagging handles GET /{bucket}/{key}?tagging.
func (h *ObjectHandler) GetObjectTagging(w http.ResponseWriter, r *http.Request, bucket, key string) {
	if !h.store.BucketExists(bucket) {
		writeS3Error(w, "NoSuchBucket", "Bucket does not exist", http.StatusNotFound)
		return
	}
	if !h.engine.ObjectExists(bucket, key) {
		writeS3Error(w, "NoSuchKey", "Object not found", http.StatusNotFound)
		return
	}

	meta, err := h.store.GetObjectMeta(bucket, key)
	if err != nil {
		// No metadata yet — return empty tag set
		writeXML(w, http.StatusOK, taggingResponse{
			Xmlns: "http://s3.amazonaws.com/doc/2006-03-01/",
		})
		return
	}

	resp := taggingResponse{
		Xmlns: "http://s3.amazonaws.com/doc/2006-03-01/",
	}
	for k, v := range meta.Tags {
		resp.TagSet.Tags = append(resp.TagSet.Tags, xmlTag{Key: k, Value: v})
	}

	writeXML(w, http.StatusOK, resp)
}

// DeleteObjectTagging handles DELETE /{bucket}/{key}?tagging.
func (h *ObjectHandler) DeleteObjectTagging(w http.ResponseWriter, r *http.Request, bucket, key string) {
	if !h.store.BucketExists(bucket) {
		writeS3Error(w, "NoSuchBucket", "Bucket does not exist", http.StatusNotFound)
		return
	}
	if !h.engine.ObjectExists(bucket, key) {
		writeS3Error(w, "NoSuchKey", "Object not found", http.StatusNotFound)
		return
	}

	meta, err := h.store.GetObjectMeta(bucket, key)
	if err != nil {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	meta.Tags = nil
	h.store.PutObjectMeta(*meta)
	w.WriteHeader(http.StatusNoContent)
}

// ListObjectVersions handles GET /{bucket}?versions.
func (h *ObjectHandler) ListObjectVersions(w http.ResponseWriter, r *http.Request, bucket string) {
	if !h.store.BucketExists(bucket) {
		writeS3Error(w, "NoSuchBucket", "Bucket does not exist", http.StatusNotFound)
		return
	}

	prefix := r.URL.Query().Get("prefix")
	keyMarker := r.URL.Query().Get("key-marker")
	versionMarker := r.URL.Query().Get("version-id-marker")
	maxKeysStr := r.URL.Query().Get("max-keys")
	maxKeys := 1000
	if maxKeysStr != "" {
		if mk, err := strconv.Atoi(maxKeysStr); err == nil && mk > 0 {
			maxKeys = mk
		}
	}

	versions, truncated, err := h.store.ListObjectVersions(bucket, prefix, keyMarker, versionMarker, maxKeys)
	if err != nil {
		writeS3Error(w, "InternalError", err.Error(), http.StatusInternalServerError)
		return
	}

	type xmlVersion struct {
		Key          string `xml:"Key"`
		VersionId    string `xml:"VersionId"`
		IsLatest     bool   `xml:"IsLatest"`
		LastModified string `xml:"LastModified"`
		ETag         string `xml:"ETag,omitempty"`
		Size         int64  `xml:"Size"`
		StorageClass string `xml:"StorageClass,omitempty"`
	}
	type xmlDeleteMarker struct {
		Key          string `xml:"Key"`
		VersionId    string `xml:"VersionId"`
		IsLatest     bool   `xml:"IsLatest"`
		LastModified string `xml:"LastModified"`
	}
	type xmlListVersionsResult struct {
		XMLName         xml.Name          `xml:"ListVersionsResult"`
		Xmlns           string            `xml:"xmlns,attr"`
		Name            string            `xml:"Name"`
		Prefix          string            `xml:"Prefix,omitempty"`
		KeyMarker       string            `xml:"KeyMarker"`
		VersionIdMarker string            `xml:"VersionIdMarker"`
		MaxKeys         int               `xml:"MaxKeys"`
		IsTruncated     bool              `xml:"IsTruncated"`
		Versions        []xmlVersion      `xml:"Version,omitempty"`
		DeleteMarkers   []xmlDeleteMarker `xml:"DeleteMarker,omitempty"`
	}

	resp := xmlListVersionsResult{
		Xmlns:           "http://s3.amazonaws.com/doc/2006-03-01/",
		Name:            bucket,
		Prefix:          prefix,
		KeyMarker:       keyMarker,
		VersionIdMarker: versionMarker,
		MaxKeys:         maxKeys,
		IsTruncated:     truncated,
	}

	for _, v := range versions {
		if v.DeleteMarker {
			resp.DeleteMarkers = append(resp.DeleteMarkers, xmlDeleteMarker{
				Key:          v.Key,
				VersionId:    v.VersionID,
				IsLatest:     v.IsLatest,
				LastModified: time.Unix(v.LastModified, 0).UTC().Format(time.RFC3339),
			})
		} else {
			resp.Versions = append(resp.Versions, xmlVersion{
				Key:          v.Key,
				VersionId:    v.VersionID,
				IsLatest:     v.IsLatest,
				LastModified: time.Unix(v.LastModified, 0).UTC().Format(time.RFC3339),
				ETag:         v.ETag,
				Size:         v.Size,
				StorageClass: "STANDARD",
			})
		}
	}

	writeXML(w, http.StatusOK, resp)
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
