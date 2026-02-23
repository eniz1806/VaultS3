package s3

import (
	"encoding/json"
	"encoding/xml"
	"io"
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

// PutBucketPolicy handles PUT /{bucket}?policy.
func (h *BucketHandler) PutBucketPolicy(w http.ResponseWriter, r *http.Request, bucket string) {
	if !h.store.BucketExists(bucket) {
		writeS3Error(w, "NoSuchBucket", "Bucket does not exist", http.StatusNotFound)
		return
	}

	body, err := io.ReadAll(io.LimitReader(r.Body, 20*1024)) // 20KB limit
	if err != nil {
		writeS3Error(w, "InternalError", err.Error(), http.StatusInternalServerError)
		return
	}

	// Validate JSON
	var js json.RawMessage
	if err := json.Unmarshal(body, &js); err != nil {
		writeS3Error(w, "MalformedPolicy", "Policy is not valid JSON", http.StatusBadRequest)
		return
	}

	if err := h.store.PutBucketPolicy(bucket, body); err != nil {
		writeS3Error(w, "InternalError", err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// GetBucketPolicy handles GET /{bucket}?policy.
func (h *BucketHandler) GetBucketPolicy(w http.ResponseWriter, r *http.Request, bucket string) {
	if !h.store.BucketExists(bucket) {
		writeS3Error(w, "NoSuchBucket", "Bucket does not exist", http.StatusNotFound)
		return
	}

	policy, err := h.store.GetBucketPolicy(bucket)
	if err != nil {
		writeS3Error(w, "NoSuchBucketPolicy", "Bucket policy does not exist", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(policy)
}

// DeleteBucketPolicy handles DELETE /{bucket}?policy.
func (h *BucketHandler) DeleteBucketPolicy(w http.ResponseWriter, r *http.Request, bucket string) {
	if !h.store.BucketExists(bucket) {
		writeS3Error(w, "NoSuchBucket", "Bucket does not exist", http.StatusNotFound)
		return
	}

	h.store.DeleteBucketPolicy(bucket)
	w.WriteHeader(http.StatusNoContent)
}

// PutBucketQuota handles PUT /{bucket}?quota.
func (h *BucketHandler) PutBucketQuota(w http.ResponseWriter, r *http.Request, bucket string) {
	if !h.store.BucketExists(bucket) {
		writeS3Error(w, "NoSuchBucket", "Bucket does not exist", http.StatusNotFound)
		return
	}

	var req struct {
		MaxSizeBytes int64 `json:"max_size_bytes"`
		MaxObjects   int64 `json:"max_objects"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeS3Error(w, "MalformedJSON", "Could not parse request body", http.StatusBadRequest)
		return
	}

	if err := h.store.UpdateBucketQuota(bucket, req.MaxSizeBytes, req.MaxObjects); err != nil {
		writeS3Error(w, "InternalError", err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

// GetBucketQuota handles GET /{bucket}?quota.
func (h *BucketHandler) GetBucketQuota(w http.ResponseWriter, r *http.Request, bucket string) {
	if !h.store.BucketExists(bucket) {
		writeS3Error(w, "NoSuchBucket", "Bucket does not exist", http.StatusNotFound)
		return
	}

	info, err := h.store.GetBucket(bucket)
	if err != nil {
		writeS3Error(w, "InternalError", err.Error(), http.StatusInternalServerError)
		return
	}

	currentSize, currentCount, _ := h.engine.BucketSize(bucket)

	resp := struct {
		MaxSizeBytes int64 `json:"max_size_bytes"`
		MaxObjects   int64 `json:"max_objects"`
		CurrentSize  int64 `json:"current_size_bytes"`
		CurrentCount int64 `json:"current_object_count"`
	}{
		MaxSizeBytes: info.MaxSizeBytes,
		MaxObjects:   info.MaxObjects,
		CurrentSize:  currentSize,
		CurrentCount: currentCount,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(resp)
}

// PutBucketLifecycle handles PUT /{bucket}?lifecycle.
func (h *BucketHandler) PutBucketLifecycle(w http.ResponseWriter, r *http.Request, bucket string) {
	if !h.store.BucketExists(bucket) {
		writeS3Error(w, "NoSuchBucket", "Bucket does not exist", http.StatusNotFound)
		return
	}

	var req struct {
		XMLName xml.Name `xml:"LifecycleConfiguration"`
		Rules   []struct {
			Expiration struct {
				Days int `xml:"Days"`
			} `xml:"Expiration"`
			Filter struct {
				Prefix string `xml:"Prefix"`
			} `xml:"Filter"`
			Status string `xml:"Status"`
		} `xml:"Rule"`
	}
	if err := xml.NewDecoder(r.Body).Decode(&req); err != nil {
		writeS3Error(w, "MalformedXML", "Could not parse lifecycle XML", http.StatusBadRequest)
		return
	}

	if len(req.Rules) == 0 {
		writeS3Error(w, "InvalidArgument", "At least one rule is required", http.StatusBadRequest)
		return
	}

	// Store the first rule (simplified — one rule per bucket)
	rule := req.Rules[0]
	if rule.Expiration.Days <= 0 {
		writeS3Error(w, "InvalidArgument", "Expiration days must be positive", http.StatusBadRequest)
		return
	}

	if err := h.store.PutLifecycleRule(bucket, metadata.LifecycleRule{
		ExpirationDays: rule.Expiration.Days,
		Prefix:         rule.Filter.Prefix,
		Status:         rule.Status,
	}); err != nil {
		writeS3Error(w, "InternalError", err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

// GetBucketLifecycle handles GET /{bucket}?lifecycle.
func (h *BucketHandler) GetBucketLifecycle(w http.ResponseWriter, r *http.Request, bucket string) {
	if !h.store.BucketExists(bucket) {
		writeS3Error(w, "NoSuchBucket", "Bucket does not exist", http.StatusNotFound)
		return
	}

	rule, err := h.store.GetLifecycleRule(bucket)
	if err != nil {
		writeS3Error(w, "NoSuchLifecycleConfiguration", "No lifecycle configuration", http.StatusNotFound)
		return
	}

	type xmlExpiration struct {
		Days int `xml:"Days"`
	}
	type xmlFilter struct {
		Prefix string `xml:"Prefix,omitempty"`
	}
	type xmlRule struct {
		Expiration xmlExpiration `xml:"Expiration"`
		Filter     xmlFilter    `xml:"Filter"`
		Status     string       `xml:"Status"`
	}
	type xmlLifecycleConfig struct {
		XMLName xml.Name  `xml:"LifecycleConfiguration"`
		Xmlns   string    `xml:"xmlns,attr"`
		Rules   []xmlRule `xml:"Rule"`
	}

	writeXML(w, http.StatusOK, xmlLifecycleConfig{
		Xmlns: "http://s3.amazonaws.com/doc/2006-03-01/",
		Rules: []xmlRule{{
			Expiration: xmlExpiration{Days: rule.ExpirationDays},
			Filter:     xmlFilter{Prefix: rule.Prefix},
			Status:     rule.Status,
		}},
	})
}

// DeleteBucketLifecycle handles DELETE /{bucket}?lifecycle.
func (h *BucketHandler) DeleteBucketLifecycle(w http.ResponseWriter, r *http.Request, bucket string) {
	if !h.store.BucketExists(bucket) {
		writeS3Error(w, "NoSuchBucket", "Bucket does not exist", http.StatusNotFound)
		return
	}

	h.store.DeleteLifecycleRule(bucket)
	w.WriteHeader(http.StatusNoContent)
}

// PutBucketVersioning handles PUT /{bucket}?versioning.
func (h *BucketHandler) PutBucketVersioning(w http.ResponseWriter, r *http.Request, bucket string) {
	if !h.store.BucketExists(bucket) {
		writeS3Error(w, "NoSuchBucket", "Bucket does not exist", http.StatusNotFound)
		return
	}

	var req struct {
		XMLName xml.Name `xml:"VersioningConfiguration"`
		Status  string   `xml:"Status"`
	}
	if err := xml.NewDecoder(r.Body).Decode(&req); err != nil {
		writeS3Error(w, "MalformedXML", "Could not parse versioning XML", http.StatusBadRequest)
		return
	}

	if req.Status != "Enabled" && req.Status != "Suspended" {
		writeS3Error(w, "IllegalVersioningConfigurationException", "Status must be Enabled or Suspended", http.StatusBadRequest)
		return
	}

	if err := h.store.SetBucketVersioning(bucket, req.Status); err != nil {
		writeS3Error(w, "InternalError", err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

// GetBucketVersioning handles GET /{bucket}?versioning.
func (h *BucketHandler) GetBucketVersioning(w http.ResponseWriter, r *http.Request, bucket string) {
	if !h.store.BucketExists(bucket) {
		writeS3Error(w, "NoSuchBucket", "Bucket does not exist", http.StatusNotFound)
		return
	}

	status, _ := h.store.GetBucketVersioning(bucket)

	type versioningConfig struct {
		XMLName xml.Name `xml:"VersioningConfiguration"`
		Xmlns   string   `xml:"xmlns,attr"`
		Status  string   `xml:"Status,omitempty"`
	}

	writeXML(w, http.StatusOK, versioningConfig{
		Xmlns:  "http://s3.amazonaws.com/doc/2006-03-01/",
		Status: status,
	})
}

// PutBucketWebsite handles PUT /{bucket}?website.
func (h *BucketHandler) PutBucketWebsite(w http.ResponseWriter, r *http.Request, bucket string) {
	if !h.store.BucketExists(bucket) {
		writeS3Error(w, "NoSuchBucket", "Bucket does not exist", http.StatusNotFound)
		return
	}

	var req struct {
		XMLName       xml.Name `xml:"WebsiteConfiguration"`
		IndexDocument struct {
			Suffix string `xml:"Suffix"`
		} `xml:"IndexDocument"`
		ErrorDocument struct {
			Key string `xml:"Key"`
		} `xml:"ErrorDocument"`
	}
	if err := xml.NewDecoder(r.Body).Decode(&req); err != nil {
		writeS3Error(w, "MalformedXML", "Could not parse website XML", http.StatusBadRequest)
		return
	}

	if req.IndexDocument.Suffix == "" {
		writeS3Error(w, "InvalidArgument", "IndexDocument Suffix is required", http.StatusBadRequest)
		return
	}

	if err := h.store.PutWebsiteConfig(bucket, metadata.WebsiteConfig{
		IndexDocument: req.IndexDocument.Suffix,
		ErrorDocument: req.ErrorDocument.Key,
	}); err != nil {
		writeS3Error(w, "InternalError", err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

// GetBucketWebsite handles GET /{bucket}?website.
func (h *BucketHandler) GetBucketWebsite(w http.ResponseWriter, r *http.Request, bucket string) {
	if !h.store.BucketExists(bucket) {
		writeS3Error(w, "NoSuchBucket", "Bucket does not exist", http.StatusNotFound)
		return
	}

	cfg, err := h.store.GetWebsiteConfig(bucket)
	if err != nil {
		writeS3Error(w, "NoSuchWebsiteConfiguration", "No website configuration", http.StatusNotFound)
		return
	}

	type xmlIndex struct {
		Suffix string `xml:"Suffix"`
	}
	type xmlError struct {
		Key string `xml:"Key,omitempty"`
	}
	type xmlWebsiteConfig struct {
		XMLName       xml.Name `xml:"WebsiteConfiguration"`
		Xmlns         string   `xml:"xmlns,attr"`
		IndexDocument xmlIndex `xml:"IndexDocument"`
		ErrorDocument *xmlError `xml:"ErrorDocument,omitempty"`
	}

	resp := xmlWebsiteConfig{
		Xmlns:         "http://s3.amazonaws.com/doc/2006-03-01/",
		IndexDocument: xmlIndex{Suffix: cfg.IndexDocument},
	}
	if cfg.ErrorDocument != "" {
		resp.ErrorDocument = &xmlError{Key: cfg.ErrorDocument}
	}

	writeXML(w, http.StatusOK, resp)
}

// DeleteBucketWebsite handles DELETE /{bucket}?website.
func (h *BucketHandler) DeleteBucketWebsite(w http.ResponseWriter, r *http.Request, bucket string) {
	if !h.store.BucketExists(bucket) {
		writeS3Error(w, "NoSuchBucket", "Bucket does not exist", http.StatusNotFound)
		return
	}

	h.store.DeleteWebsiteConfig(bucket)
	w.WriteHeader(http.StatusNoContent)
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
