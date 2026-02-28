package ratelimit

import (
	"io"
	"sync"
	"time"
)

// BandwidthLimiter throttles read/write throughput per bucket.
type BandwidthLimiter struct {
	mu      sync.Mutex
	buckets map[string]*bwBucket
	defaultBytesPerSec int64
}

type bwBucket struct {
	bytesPerSec int64
	tokens      float64
	lastTime    time.Time
}

// NewBandwidthLimiter creates a bandwidth limiter with a default bytes/sec limit per bucket.
func NewBandwidthLimiter(defaultBytesPerSec int64) *BandwidthLimiter {
	return &BandwidthLimiter{
		buckets:            make(map[string]*bwBucket),
		defaultBytesPerSec: defaultBytesPerSec,
	}
}

// SetBucketLimit sets a per-bucket bandwidth limit in bytes/sec. 0 means use default.
func (bl *BandwidthLimiter) SetBucketLimit(bucket string, bytesPerSec int64) {
	bl.mu.Lock()
	defer bl.mu.Unlock()
	if bytesPerSec <= 0 {
		delete(bl.buckets, bucket)
		return
	}
	bl.buckets[bucket] = &bwBucket{
		bytesPerSec: bytesPerSec,
		tokens:      float64(bytesPerSec),
		lastTime:    time.Now(),
	}
}

func (bl *BandwidthLimiter) getBucket(bucket string) *bwBucket {
	bl.mu.Lock()
	defer bl.mu.Unlock()
	b, ok := bl.buckets[bucket]
	if !ok {
		b = &bwBucket{
			bytesPerSec: bl.defaultBytesPerSec,
			tokens:      float64(bl.defaultBytesPerSec),
			lastTime:    time.Now(),
		}
		bl.buckets[bucket] = b
	}
	return b
}

// ThrottledReader wraps an io.Reader with bandwidth throttling.
func (bl *BandwidthLimiter) ThrottledReader(bucket string, r io.Reader) io.Reader {
	if bl.defaultBytesPerSec <= 0 {
		return r
	}
	return &throttledReader{
		reader: r,
		bw:     bl,
		bucket: bucket,
	}
}

// ThrottledWriter wraps an io.Writer with bandwidth throttling.
func (bl *BandwidthLimiter) ThrottledWriter(bucket string, w io.Writer) io.Writer {
	if bl.defaultBytesPerSec <= 0 {
		return w
	}
	return &throttledWriter{
		writer: w,
		bw:     bl,
		bucket: bucket,
	}
}

func (bl *BandwidthLimiter) waitForTokens(bucket string, n int) {
	b := bl.getBucket(bucket)
	for {
		bl.mu.Lock()
		now := time.Now()
		elapsed := now.Sub(b.lastTime).Seconds()
		b.tokens += elapsed * float64(b.bytesPerSec)
		if b.tokens > float64(b.bytesPerSec) {
			b.tokens = float64(b.bytesPerSec)
		}
		b.lastTime = now

		if b.tokens >= float64(n) {
			b.tokens -= float64(n)
			bl.mu.Unlock()
			return
		}
		bl.mu.Unlock()
		time.Sleep(10 * time.Millisecond)
	}
}

type throttledReader struct {
	reader io.Reader
	bw     *BandwidthLimiter
	bucket string
}

func (tr *throttledReader) Read(p []byte) (int, error) {
	n, err := tr.reader.Read(p)
	if n > 0 {
		tr.bw.waitForTokens(tr.bucket, n)
	}
	return n, err
}

type throttledWriter struct {
	writer io.Writer
	bw     *BandwidthLimiter
	bucket string
}

func (tw *throttledWriter) Write(p []byte) (int, error) {
	tw.bw.waitForTokens(tw.bucket, len(p))
	return tw.writer.Write(p)
}
