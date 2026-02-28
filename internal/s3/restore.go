package s3

import (
	"encoding/xml"
	"io"
	"log/slog"
	"net/http"
	"time"
)

type restoreRequest struct {
	XMLName xml.Name `xml:"RestoreRequest"`
	Days    int      `xml:"Days"`
	Tier    string   `xml:"GlacierJobParameters>Tier,omitempty"`
}

// RestoreObject handles POST /{bucket}/{key}?restore.
func (h *ObjectHandler) RestoreObject(w http.ResponseWriter, r *http.Request, bucket, key string) {
	if !h.store.BucketExists(bucket) {
		writeS3Error(w, "NoSuchBucket", "Bucket does not exist", http.StatusNotFound)
		return
	}

	meta, err := h.store.GetObjectMeta(bucket, key)
	if err != nil || meta == nil {
		writeS3Error(w, "NoSuchKey", "Object not found", http.StatusNotFound)
		return
	}

	// Only cold-tiered objects need restoration
	if meta.Tier != "cold" {
		writeS3Error(w, "InvalidObjectState", "Object is not in a cold storage class", http.StatusConflict)
		return
	}

	var req restoreRequest
	body, err := io.ReadAll(io.LimitReader(r.Body, 64*1024))
	if err != nil {
		writeS3Error(w, "InternalError", "Failed to read body", http.StatusInternalServerError)
		return
	}
	if len(body) > 0 {
		if err := xml.Unmarshal(body, &req); err != nil {
			writeS3Error(w, "MalformedXML", "Invalid restore request", http.StatusBadRequest)
			return
		}
	}
	if req.Days <= 0 {
		req.Days = 1
	}

	// Mark object as being restored (change tier to hot)
	meta.Tier = "hot"
	meta.LastModified = time.Now().UTC().Unix()
	if err := h.store.PutObjectMeta(*meta); err != nil {
		slog.Error("restore error", "error", err)
		writeS3Error(w, "InternalError", "Failed to update metadata", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusAccepted)
}
