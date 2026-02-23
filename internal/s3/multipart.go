package s3

import (
	"crypto/md5"
	"crypto/rand"
	"encoding/hex"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"time"

	"github.com/eniz1806/VaultS3/internal/metadata"
)

// CreateMultipartUpload handles POST /{bucket}/{key}?uploads.
func (h *ObjectHandler) CreateMultipartUpload(w http.ResponseWriter, r *http.Request, bucket, key string) {
	if !h.store.BucketExists(bucket) {
		writeS3Error(w, "NoSuchBucket", "Bucket does not exist", http.StatusNotFound)
		return
	}

	uploadID := generateUploadID()

	ct := r.Header.Get("Content-Type")
	if ct == "" {
		ct = "application/octet-stream"
	}

	upload := metadata.MultipartUpload{
		UploadID:    uploadID,
		Bucket:      bucket,
		Key:         key,
		ContentType: ct,
		CreatedAt:   time.Now().UTC().Unix(),
	}

	if err := h.store.CreateMultipartUpload(upload); err != nil {
		writeS3Error(w, "InternalError", err.Error(), http.StatusInternalServerError)
		return
	}

	partsDir := h.multipartDir(uploadID)
	if err := os.MkdirAll(partsDir, 0755); err != nil {
		writeS3Error(w, "InternalError", err.Error(), http.StatusInternalServerError)
		return
	}

	type initResult struct {
		XMLName  xml.Name `xml:"InitiateMultipartUploadResult"`
		Xmlns    string   `xml:"xmlns,attr"`
		Bucket   string   `xml:"Bucket"`
		Key      string   `xml:"Key"`
		UploadId string   `xml:"UploadId"`
	}

	writeXML(w, http.StatusOK, initResult{
		Xmlns:    "http://s3.amazonaws.com/doc/2006-03-01/",
		Bucket:   bucket,
		Key:      key,
		UploadId: uploadID,
	})
}

// UploadPart handles PUT /{bucket}/{key}?partNumber=N&uploadId=X.
func (h *ObjectHandler) UploadPart(w http.ResponseWriter, r *http.Request, bucket, key, uploadID string) {
	_, err := h.store.GetMultipartUpload(uploadID)
	if err != nil {
		writeS3Error(w, "NoSuchUpload", "Upload not found", http.StatusNotFound)
		return
	}

	partNumStr := r.URL.Query().Get("partNumber")
	partNum, err := strconv.Atoi(partNumStr)
	if err != nil || partNum < 1 || partNum > 10000 {
		writeS3Error(w, "InvalidArgument", "Invalid part number", http.StatusBadRequest)
		return
	}

	partPath := filepath.Join(h.multipartDir(uploadID), fmt.Sprintf("part-%05d", partNum))
	f, err := os.Create(partPath)
	if err != nil {
		writeS3Error(w, "InternalError", err.Error(), http.StatusInternalServerError)
		return
	}
	defer f.Close()

	hash := md5.New()
	written, err := io.Copy(f, io.TeeReader(r.Body, hash))
	if err != nil {
		os.Remove(partPath)
		writeS3Error(w, "InternalError", err.Error(), http.StatusInternalServerError)
		return
	}

	etag := fmt.Sprintf("\"%s\"", hex.EncodeToString(hash.Sum(nil)))

	h.store.PutPart(uploadID, metadata.PartInfo{
		PartNumber: partNum,
		ETag:       etag,
		Size:       written,
	})

	w.Header().Set("ETag", etag)
	w.WriteHeader(http.StatusOK)
}

// CompleteMultipartUpload handles POST /{bucket}/{key}?uploadId=X.
func (h *ObjectHandler) CompleteMultipartUpload(w http.ResponseWriter, r *http.Request, bucket, key, uploadID string) {
	upload, err := h.store.GetMultipartUpload(uploadID)
	if err != nil {
		writeS3Error(w, "NoSuchUpload", "Upload not found", http.StatusNotFound)
		return
	}

	// Check quota (estimate size from parts)
	parts, _ := h.store.ListParts(uploadID)
	var estimatedSize int64
	for _, p := range parts {
		estimatedSize += p.Size
	}
	if !h.checkQuota(w, bucket, estimatedSize) {
		return
	}

	type completePart struct {
		PartNumber int    `xml:"PartNumber"`
		ETag       string `xml:"ETag"`
	}
	type completeRequest struct {
		XMLName xml.Name       `xml:"CompleteMultipartUpload"`
		Parts   []completePart `xml:"Part"`
	}

	var req completeRequest
	if err := xml.NewDecoder(r.Body).Decode(&req); err != nil {
		writeS3Error(w, "MalformedXML", "Could not parse request body", http.StatusBadRequest)
		return
	}

	sort.Slice(req.Parts, func(i, j int) bool {
		return req.Parts[i].PartNumber < req.Parts[j].PartNumber
	})

	// Assemble the final object
	objPath := h.engine.ObjectPath(bucket, key)
	if err := os.MkdirAll(filepath.Dir(objPath), 0755); err != nil {
		writeS3Error(w, "InternalError", err.Error(), http.StatusInternalServerError)
		return
	}

	outFile, err := os.Create(objPath)
	if err != nil {
		writeS3Error(w, "InternalError", err.Error(), http.StatusInternalServerError)
		return
	}
	defer outFile.Close()

	// Concatenate parts and compute multipart ETag
	var totalSize int64
	combinedHash := md5.New()

	for _, part := range req.Parts {
		partPath := filepath.Join(h.multipartDir(uploadID), fmt.Sprintf("part-%05d", part.PartNumber))
		pf, err := os.Open(partPath)
		if err != nil {
			os.Remove(objPath)
			writeS3Error(w, "InvalidPart", fmt.Sprintf("Part %d not found", part.PartNumber), http.StatusBadRequest)
			return
		}

		partHash := md5.New()
		written, err := io.Copy(outFile, io.TeeReader(pf, partHash))
		pf.Close()
		if err != nil {
			os.Remove(objPath)
			writeS3Error(w, "InternalError", err.Error(), http.StatusInternalServerError)
			return
		}

		totalSize += written
		combinedHash.Write(partHash.Sum(nil))
	}

	// S3 multipart ETag: md5(md5(part1) + md5(part2) + ...)-N
	etag := fmt.Sprintf("\"%s-%d\"", hex.EncodeToString(combinedHash.Sum(nil)), len(req.Parts))

	now := time.Now().UTC()

	h.store.PutObjectMeta(metadata.ObjectMeta{
		Bucket:       bucket,
		Key:          key,
		ContentType:  upload.ContentType,
		ETag:         etag,
		Size:         totalSize,
		LastModified: now.Unix(),
	})

	// Clean up
	os.RemoveAll(h.multipartDir(uploadID))
	h.store.DeleteMultipartUpload(uploadID)

	type completeResult struct {
		XMLName  xml.Name `xml:"CompleteMultipartUploadResult"`
		Xmlns    string   `xml:"xmlns,attr"`
		Location string   `xml:"Location"`
		Bucket   string   `xml:"Bucket"`
		Key      string   `xml:"Key"`
		ETag     string   `xml:"ETag"`
	}

	writeXML(w, http.StatusOK, completeResult{
		Xmlns:    "http://s3.amazonaws.com/doc/2006-03-01/",
		Location: fmt.Sprintf("/%s/%s", bucket, key),
		Bucket:   bucket,
		Key:      key,
		ETag:     etag,
	})
	if h.onNotification != nil {
		h.onNotification("s3:ObjectCreated:CompleteMultipartUpload", bucket, key, totalSize, etag, "")
	}
	if h.onReplication != nil {
		h.onReplication("s3:ObjectCreated:CompleteMultipartUpload", bucket, key, totalSize, etag, "")
	}
	if h.onScan != nil {
		h.onScan(bucket, key, totalSize)
	}
	if h.onSearchUpdate != nil {
		h.onSearchUpdate("put", bucket, key)
	}
}

// AbortMultipartUpload handles DELETE /{bucket}/{key}?uploadId=X.
func (h *ObjectHandler) AbortMultipartUpload(w http.ResponseWriter, r *http.Request, bucket, key, uploadID string) {
	_, err := h.store.GetMultipartUpload(uploadID)
	if err != nil {
		writeS3Error(w, "NoSuchUpload", "Upload not found", http.StatusNotFound)
		return
	}

	os.RemoveAll(h.multipartDir(uploadID))
	h.store.DeleteMultipartUpload(uploadID)

	w.WriteHeader(http.StatusNoContent)
}

func (h *ObjectHandler) multipartDir(uploadID string) string {
	return filepath.Join(h.engine.DataDir(), ".multipart", uploadID)
}

func generateUploadID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}
