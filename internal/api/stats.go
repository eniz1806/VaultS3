package api

import (
	"net/http"
	"runtime"
	"time"
)

type bucketStat struct {
	Name         string `json:"name"`
	Size         int64  `json:"size"`
	ObjectCount  int64  `json:"objectCount"`
	MaxSizeBytes int64  `json:"maxSizeBytes,omitempty"`
	MaxObjects   int64  `json:"maxObjects,omitempty"`
}

type requestMethodStat struct {
	Method string `json:"method"`
	Count  int64  `json:"count"`
}

type statsResponse struct {
	TotalBuckets     int                 `json:"totalBuckets"`
	TotalObjects     int64               `json:"totalObjects"`
	TotalSize        int64               `json:"totalSize"`
	UptimeSeconds    float64             `json:"uptimeSeconds"`
	Goroutines       int                 `json:"goroutines"`
	MemoryMB         float64             `json:"memoryMB"`
	Buckets          []bucketStat        `json:"buckets"`
	RequestsByMethod []requestMethodStat `json:"requestsByMethod"`
	TotalRequests    int64               `json:"totalRequests"`
	TotalErrors      int64               `json:"totalErrors"`
	BytesIn          int64               `json:"bytesIn"`
	BytesOut         int64               `json:"bytesOut"`
}

func (h *APIHandler) handleStats(w http.ResponseWriter, _ *http.Request) {
	buckets, err := h.store.ListBuckets()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get stats")
		return
	}

	var totalSize, totalObjects int64
	bucketStats := make([]bucketStat, 0, len(buckets))

	for _, b := range buckets {
		size, count, _ := h.engine.BucketSize(b.Name)
		totalSize += size
		totalObjects += count
		bucketStats = append(bucketStats, bucketStat{
			Name:         b.Name,
			Size:         size,
			ObjectCount:  count,
			MaxSizeBytes: b.MaxSizeBytes,
			MaxObjects:   b.MaxObjects,
		})
	}

	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)

	// Request metrics
	reqByMethod := h.metrics.RequestsByMethod()
	methodStats := make([]requestMethodStat, 0, len(reqByMethod))
	for method, count := range reqByMethod {
		methodStats = append(methodStats, requestMethodStat{Method: method, Count: count})
	}

	writeJSON(w, http.StatusOK, statsResponse{
		TotalBuckets:     len(buckets),
		TotalObjects:     totalObjects,
		TotalSize:        totalSize,
		UptimeSeconds:    time.Since(h.metrics.StartTime()).Seconds(),
		Goroutines:       runtime.NumGoroutine(),
		MemoryMB:         float64(mem.Alloc) / 1024 / 1024,
		Buckets:          bucketStats,
		RequestsByMethod: methodStats,
		TotalRequests:    h.metrics.TotalRequests(),
		TotalErrors:      h.metrics.TotalErrors(),
		BytesIn:          h.metrics.TotalBytesIn(),
		BytesOut:         h.metrics.TotalBytesOut(),
	})
}
