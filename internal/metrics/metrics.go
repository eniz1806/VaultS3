package metrics

import (
	"fmt"
	"net/http"
	"runtime"
	"sync/atomic"
	"time"

	"github.com/eniz1806/VaultS3/internal/metadata"
	"github.com/eniz1806/VaultS3/internal/storage"
)

// Collector tracks request metrics and exposes Prometheus-compatible /metrics.
type Collector struct {
	store  *metadata.Store
	engine storage.Engine

	// Request counters by method
	requestsTotal   [methodCount]atomic.Int64
	requestErrors   atomic.Int64
	bytesIn         atomic.Int64
	bytesOut        atomic.Int64
	startTime       time.Time
}

// HTTP method indices for counter array
const (
	mGET = iota
	mPUT
	mDELETE
	mHEAD
	mPOST
	mOTHER
	methodCount
)

func methodIndex(method string) int {
	switch method {
	case http.MethodGet:
		return mGET
	case http.MethodPut:
		return mPUT
	case http.MethodDelete:
		return mDELETE
	case http.MethodHead:
		return mHEAD
	case http.MethodPost:
		return mPOST
	default:
		return mOTHER
	}
}

func methodLabel(idx int) string {
	switch idx {
	case mGET:
		return "GET"
	case mPUT:
		return "PUT"
	case mDELETE:
		return "DELETE"
	case mHEAD:
		return "HEAD"
	case mPOST:
		return "POST"
	default:
		return "OTHER"
	}
}

func NewCollector(store *metadata.Store, engine storage.Engine) *Collector {
	return &Collector{
		store:     store,
		engine:    engine,
		startTime: time.Now(),
	}
}

// StartTime returns when the collector was created (server start time).
func (c *Collector) StartTime() time.Time {
	return c.startTime
}

// RecordRequest increments the request counter for the given method.
func (c *Collector) RecordRequest(method string) {
	c.requestsTotal[methodIndex(method)].Add(1)
}

// RecordError increments the error counter.
func (c *Collector) RecordError() {
	c.requestErrors.Add(1)
}

// RecordBytesIn adds to the ingress byte counter.
func (c *Collector) RecordBytesIn(n int64) {
	c.bytesIn.Add(n)
}

// RecordBytesOut adds to the egress byte counter.
func (c *Collector) RecordBytesOut(n int64) {
	c.bytesOut.Add(n)
}

// ServeHTTP handles GET /metrics in Prometheus exposition format.
func (c *Collector) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")

	// Process metrics
	var totalRequests int64
	for i := 0; i < methodCount; i++ {
		v := c.requestsTotal[i].Load()
		totalRequests += v
		fmt.Fprintf(w, "vaults3_requests_total{method=%q} %d\n", methodLabel(i), v)
	}
	fmt.Fprintf(w, "vaults3_requests_total_sum %d\n", totalRequests)
	fmt.Fprintf(w, "vaults3_request_errors_total %d\n", c.requestErrors.Load())
	fmt.Fprintf(w, "vaults3_bytes_received_total %d\n", c.bytesIn.Load())
	fmt.Fprintf(w, "vaults3_bytes_sent_total %d\n", c.bytesOut.Load())

	// Uptime
	fmt.Fprintf(w, "vaults3_uptime_seconds %.0f\n", time.Since(c.startTime).Seconds())

	// Storage metrics — per bucket
	buckets, err := c.store.ListBuckets()
	if err == nil {
		fmt.Fprintf(w, "vaults3_buckets_total %d\n", len(buckets))

		var totalSize, totalObjects int64
		for _, b := range buckets {
			size, count, err := c.engine.BucketSize(b.Name)
			if err != nil {
				continue
			}
			totalSize += size
			totalObjects += count
			fmt.Fprintf(w, "vaults3_bucket_size_bytes{bucket=%q} %d\n", b.Name, size)
			fmt.Fprintf(w, "vaults3_bucket_objects_total{bucket=%q} %d\n", b.Name, count)
			if b.MaxSizeBytes > 0 {
				fmt.Fprintf(w, "vaults3_bucket_quota_size_bytes{bucket=%q} %d\n", b.Name, b.MaxSizeBytes)
			}
			if b.MaxObjects > 0 {
				fmt.Fprintf(w, "vaults3_bucket_quota_objects{bucket=%q} %d\n", b.Name, b.MaxObjects)
			}
		}
		fmt.Fprintf(w, "vaults3_storage_size_bytes_total %d\n", totalSize)
		fmt.Fprintf(w, "vaults3_objects_total %d\n", totalObjects)
	}

	// Go runtime metrics
	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)
	fmt.Fprintf(w, "vaults3_go_goroutines %d\n", runtime.NumGoroutine())
	fmt.Fprintf(w, "vaults3_go_memory_alloc_bytes %d\n", mem.Alloc)
	fmt.Fprintf(w, "vaults3_go_memory_sys_bytes %d\n", mem.Sys)
	fmt.Fprintf(w, "vaults3_go_gc_total %d\n", mem.NumGC)
}
