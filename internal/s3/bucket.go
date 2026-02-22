package s3

import (
	"encoding/xml"
	"net/http"
	"time"

	"github.com/eniz1806/VaultS3/internal/metadata"
	"github.com/eniz1806/VaultS3/internal/storage"
)

type BucketHandler struct {
	store  *metadata.Store
	engine storage.Engine
}

// ListBuckets responds to GET / with a list of all buckets.
func (h *BucketHandler) ListBuckets(w http.ResponseWriter, r *http.Request) {
	buckets, err := h.store.ListBuckets()
	if err != nil {
		writeS3Error(w, "InternalError", err.Error(), http.StatusInternalServerError)
		return
	}

	type xmlBucket struct {
		Name         string `xml:"Name"`
		CreationDate string `xml:"CreationDate"`
	}
	type xmlResponse struct {
		XMLName xml.Name    `xml:"ListAllMyBucketsResult"`
		Xmlns   string      `xml:"xmlns,attr"`
		Owner   xmlOwner    `xml:"Owner"`
		Buckets []xmlBucket `xml:"Buckets>Bucket"`
	}

	resp := xmlResponse{
		Xmlns: "http://s3.amazonaws.com/doc/2006-03-01/",
		Owner: xmlOwner{ID: "vaults3", DisplayName: "VaultS3"},
	}
	for _, b := range buckets {
		resp.Buckets = append(resp.Buckets, xmlBucket{
			Name:         b.Name,
			CreationDate: b.CreatedAt.Format(time.RFC3339),
		})
	}

	writeXML(w, http.StatusOK, resp)
}

// CreateBucket handles PUT /{bucket}.
func (h *BucketHandler) CreateBucket(w http.ResponseWriter, r *http.Request, bucket string) {
	if !isValidBucketName(bucket) {
		writeS3Error(w, "InvalidBucketName", "Invalid bucket name", http.StatusBadRequest)
		return
	}

	if err := h.store.CreateBucket(bucket); err != nil {
		writeS3Error(w, "BucketAlreadyExists", err.Error(), http.StatusConflict)
		return
	}

	if err := h.engine.CreateBucketDir(bucket); err != nil {
		h.store.DeleteBucket(bucket) // rollback
		writeS3Error(w, "InternalError", err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Location", "/"+bucket)
	w.WriteHeader(http.StatusOK)
}

// DeleteBucket handles DELETE /{bucket}.
func (h *BucketHandler) DeleteBucket(w http.ResponseWriter, r *http.Request, bucket string) {
	if !h.store.BucketExists(bucket) {
		writeS3Error(w, "NoSuchBucket", "Bucket does not exist", http.StatusNotFound)
		return
	}

	// Check if bucket is empty
	objects, _, err := h.engine.ListObjects(bucket, "", "", 1)
	if err != nil {
		writeS3Error(w, "InternalError", err.Error(), http.StatusInternalServerError)
		return
	}
	if len(objects) > 0 {
		writeS3Error(w, "BucketNotEmpty", "Bucket is not empty", http.StatusConflict)
		return
	}

	if err := h.engine.DeleteBucketDir(bucket); err != nil {
		writeS3Error(w, "InternalError", err.Error(), http.StatusInternalServerError)
		return
	}

	if err := h.store.DeleteBucket(bucket); err != nil {
		writeS3Error(w, "InternalError", err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// HeadBucket handles HEAD /{bucket}.
func (h *BucketHandler) HeadBucket(w http.ResponseWriter, r *http.Request, bucket string) {
	if !h.store.BucketExists(bucket) {
		writeS3Error(w, "NoSuchBucket", "Bucket does not exist", http.StatusNotFound)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func isValidBucketName(name string) bool {
	if len(name) < 3 || len(name) > 63 {
		return false
	}
	for _, c := range name {
		if !((c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '-' || c == '.') {
			return false
		}
	}
	return true
}
