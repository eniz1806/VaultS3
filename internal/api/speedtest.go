package api

import (
	"bytes"
	"crypto/rand"
	"io"
	"net/http"
	"time"
)

type speedtestResult struct {
	WriteThroughputMBps float64 `json:"writeThroughputMBps"`
	ReadThroughputMBps  float64 `json:"readThroughputMBps"`
	Duration            string  `json:"duration"`
}

// handleSpeedtest handles POST /api/v1/speedtest — runs a drive benchmark.
func (h *APIHandler) handleSpeedtest(w http.ResponseWriter, r *http.Request) {
	const testSize = 64 * 1024 * 1024 // 64MB
	testBucket := "__speedtest__"
	testKey := "__speedtest_object__"

	// Generate random data
	data := make([]byte, testSize)
	rand.Read(data)

	h.engine.CreateBucketDir(testBucket)

	// Write benchmark
	start := time.Now()
	_, _, err := h.engine.PutObject(testBucket, testKey, bytes.NewReader(data), int64(len(data)))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "write benchmark failed: "+err.Error())
		return
	}
	writeDur := time.Since(start)

	// Read benchmark
	start = time.Now()
	reader, _, err := h.engine.GetObject(testBucket, testKey)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "read benchmark failed: "+err.Error())
		return
	}
	io.Copy(io.Discard, reader)
	reader.Close()
	readDur := time.Since(start)

	// Cleanup
	h.engine.DeleteObject(testBucket, testKey)
	h.engine.DeleteBucketDir(testBucket)

	totalDur := writeDur + readDur

	writeJSON(w, http.StatusOK, speedtestResult{
		WriteThroughputMBps: float64(testSize) / (1024 * 1024) / writeDur.Seconds(),
		ReadThroughputMBps:  float64(testSize) / (1024 * 1024) / readDur.Seconds(),
		Duration:            totalDur.String(),
	})
}
