package s3

import (
	"encoding/json"
	"encoding/xml"
	"io"
	"net/http"
)

// ReplicationConfig stored in metadata.
type replicationConfigXML struct {
	XMLName xml.Name             `xml:"ReplicationConfiguration"`
	Xmlns   string               `xml:"xmlns,attr,omitempty"`
	Role    string               `xml:"Role,omitempty"`
	Rules   []replicationRuleXML `xml:"Rule"`
}

type replicationRuleXML struct {
	ID                        string `xml:"ID,omitempty"`
	Status                    string `xml:"Status"`
	Priority                  int    `xml:"Priority,omitempty"`
	Prefix                    string `xml:"Filter>Prefix,omitempty"`
	DestinationBucket         string `xml:"Destination>Bucket"`
	DestinationStorageClass   string `xml:"Destination>StorageClass,omitempty"`
	DeleteMarkerReplication   string `xml:"DeleteMarkerReplication>Status,omitempty"`
	ExistingObjectReplication string `xml:"ExistingObjectReplication>Status,omitempty"`
}

// PutBucketReplication handles PUT /{bucket}?replication.
func (h *BucketHandler) PutBucketReplication(w http.ResponseWriter, r *http.Request, bucket string) {
	if !h.store.BucketExists(bucket) {
		writeS3Error(w, "NoSuchBucket", "Bucket does not exist", http.StatusNotFound)
		return
	}

	body, err := io.ReadAll(io.LimitReader(r.Body, 256*1024))
	if err != nil {
		writeS3Error(w, "InternalError", "Failed to read body", http.StatusInternalServerError)
		return
	}

	var cfg replicationConfigXML
	if err := xml.Unmarshal(body, &cfg); err != nil {
		writeS3Error(w, "MalformedXML", "Invalid replication configuration XML", http.StatusBadRequest)
		return
	}

	// Store as JSON in metadata
	data, _ := json.Marshal(cfg)
	if err := h.store.PutReplicationConfig(bucket, string(data)); err != nil {
		writeS3Error(w, "InternalError", "Failed to save replication config", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

// GetBucketReplication handles GET /{bucket}?replication.
func (h *BucketHandler) GetBucketReplication(w http.ResponseWriter, r *http.Request, bucket string) {
	if !h.store.BucketExists(bucket) {
		writeS3Error(w, "NoSuchBucket", "Bucket does not exist", http.StatusNotFound)
		return
	}

	data, err := h.store.GetReplicationConfig(bucket)
	if err != nil || data == "" {
		writeS3Error(w, "ReplicationConfigurationNotFoundError", "No replication configuration", http.StatusNotFound)
		return
	}

	var cfg replicationConfigXML
	if err := json.Unmarshal([]byte(data), &cfg); err != nil {
		writeS3Error(w, "InternalError", "Failed to parse stored config", http.StatusInternalServerError)
		return
	}
	cfg.Xmlns = "http://s3.amazonaws.com/doc/2006-03-01/"

	writeXML(w, http.StatusOK, cfg)
}

// DeleteBucketReplication handles DELETE /{bucket}?replication.
func (h *BucketHandler) DeleteBucketReplication(w http.ResponseWriter, r *http.Request, bucket string) {
	if !h.store.BucketExists(bucket) {
		writeS3Error(w, "NoSuchBucket", "Bucket does not exist", http.StatusNotFound)
		return
	}

	h.store.DeleteReplicationConfig(bucket)
	w.WriteHeader(http.StatusNoContent)
}
