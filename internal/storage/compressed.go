package storage

import (
	"bytes"
	"compress/gzip"
	"crypto/md5"
	"fmt"
	"io"
	"path/filepath"
	"strings"
)

// excludedExtensions lists file extensions that should NOT be compressed
// because they are already compressed or would not benefit from compression.
var excludedExtensions = map[string]bool{
	".jpg": true, ".jpeg": true, ".png": true, ".gif": true, ".webp": true,
	".gz": true, ".tgz": true, ".bz2": true, ".xz": true, ".zst": true, ".lz4": true,
	".zip": true, ".rar": true, ".7z": true, ".tar.gz": true,
	".mp4": true, ".mkv": true, ".avi": true, ".mov": true, ".webm": true,
	".mp3": true, ".flac": true, ".ogg": true, ".aac": true,
	".woff": true, ".woff2": true,
}

// CompressedEngine wraps another Engine and compresses/decompresses data transparently.
// Uses gzip compression. Data is compressed before writing and decompressed after reading.
// Files with already-compressed extensions are passed through without compression.
type CompressedEngine struct {
	inner        Engine
	ExcludedTypes map[string]bool // additional excluded extensions
}

func NewCompressedEngine(inner Engine) *CompressedEngine {
	return &CompressedEngine{inner: inner}
}

// shouldCompress returns true if the key should be compressed.
func (c *CompressedEngine) shouldCompress(key string) bool {
	ext := strings.ToLower(filepath.Ext(key))
	if excludedExtensions[ext] {
		return false
	}
	if c.ExcludedTypes != nil && c.ExcludedTypes[ext] {
		return false
	}
	return true
}

func (c *CompressedEngine) CreateBucketDir(bucket string) error {
	return c.inner.CreateBucketDir(bucket)
}

func (c *CompressedEngine) DeleteBucketDir(bucket string) error {
	return c.inner.DeleteBucketDir(bucket)
}

func (c *CompressedEngine) PutObject(bucket, key string, reader io.Reader, size int64) (int64, string, error) {
	if !c.shouldCompress(key) {
		return c.inner.PutObject(bucket, key, reader, size)
	}
	return c.compressAndPut(reader, func(compressed io.Reader, compressedSize int64) (int64, string, error) {
		return c.inner.PutObject(bucket, key, compressed, compressedSize)
	})
}

func (c *CompressedEngine) GetObject(bucket, key string) (ReadSeekCloser, int64, error) {
	if !c.shouldCompress(key) {
		return c.inner.GetObject(bucket, key)
	}
	return c.getAndDecompress(func() (ReadSeekCloser, int64, error) {
		return c.inner.GetObject(bucket, key)
	})
}

func (c *CompressedEngine) DeleteObject(bucket, key string) error {
	return c.inner.DeleteObject(bucket, key)
}

func (c *CompressedEngine) ObjectExists(bucket, key string) bool {
	return c.inner.ObjectExists(bucket, key)
}

func (c *CompressedEngine) ObjectSize(bucket, key string) (int64, error) {
	return c.inner.ObjectSize(bucket, key)
}

func (c *CompressedEngine) ListObjects(bucket, prefix, startAfter string, maxKeys int) ([]ObjectInfo, bool, error) {
	return c.inner.ListObjects(bucket, prefix, startAfter, maxKeys)
}

func (c *CompressedEngine) BucketSize(bucket string) (int64, int64, error) {
	return c.inner.BucketSize(bucket)
}

func (c *CompressedEngine) PutObjectVersion(bucket, key, versionID string, reader io.Reader, size int64) (int64, string, error) {
	if !c.shouldCompress(key) {
		return c.inner.PutObjectVersion(bucket, key, versionID, reader, size)
	}
	return c.compressAndPut(reader, func(compressed io.Reader, compressedSize int64) (int64, string, error) {
		return c.inner.PutObjectVersion(bucket, key, versionID, compressed, compressedSize)
	})
}

func (c *CompressedEngine) GetObjectVersion(bucket, key, versionID string) (ReadSeekCloser, int64, error) {
	if !c.shouldCompress(key) {
		return c.inner.GetObjectVersion(bucket, key, versionID)
	}
	return c.getAndDecompress(func() (ReadSeekCloser, int64, error) {
		return c.inner.GetObjectVersion(bucket, key, versionID)
	})
}

func (c *CompressedEngine) DeleteObjectVersion(bucket, key, versionID string) error {
	return c.inner.DeleteObjectVersion(bucket, key, versionID)
}

func (c *CompressedEngine) DataDir() string {
	return c.inner.DataDir()
}

func (c *CompressedEngine) ObjectPath(bucket, key string) string {
	return c.inner.ObjectPath(bucket, key)
}

// maxCompressedSize is the maximum object size for in-memory compression (1GB).
const maxCompressedSize int64 = 1 * 1024 * 1024 * 1024

// compressAndPut reads all data, compresses it, computes ETag of original, writes compressed.
func (c *CompressedEngine) compressAndPut(reader io.Reader, putFn func(io.Reader, int64) (int64, string, error)) (int64, string, error) {
	plaintext, err := io.ReadAll(io.LimitReader(reader, maxCompressedSize+1))
	if err != nil {
		return 0, "", fmt.Errorf("read plaintext: %w", err)
	}
	if int64(len(plaintext)) > maxCompressedSize {
		return 0, "", fmt.Errorf("object too large for compression (max %dMB)", maxCompressedSize/(1024*1024))
	}

	// Compute ETag of original data
	h := md5.Sum(plaintext)
	etag := fmt.Sprintf("\"%x\"", h)

	// Compress
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	if _, err := gz.Write(plaintext); err != nil {
		return 0, "", fmt.Errorf("compress: %w", err)
	}
	if err := gz.Close(); err != nil {
		return 0, "", fmt.Errorf("compress close: %w", err)
	}

	// Write compressed data to inner engine
	_, _, err = putFn(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if err != nil {
		return 0, "", err
	}

	// Return original plaintext size and ETag
	return int64(len(plaintext)), etag, nil
}

// getAndDecompress reads compressed data from inner engine, decompresses it.
func (c *CompressedEngine) getAndDecompress(getFn func() (ReadSeekCloser, int64, error)) (ReadSeekCloser, int64, error) {
	reader, _, err := getFn()
	if err != nil {
		return nil, 0, err
	}
	defer reader.Close()

	compressed, err := io.ReadAll(reader)
	if err != nil {
		return nil, 0, fmt.Errorf("read compressed data: %w", err)
	}

	gz, err := gzip.NewReader(bytes.NewReader(compressed))
	if err != nil {
		return nil, 0, fmt.Errorf("gzip reader: %w", err)
	}
	defer gz.Close()

	plaintext, err := io.ReadAll(io.LimitReader(gz, maxCompressedSize+1))
	if err != nil {
		return nil, 0, fmt.Errorf("decompress: %w", err)
	}
	if int64(len(plaintext)) > maxCompressedSize {
		return nil, 0, fmt.Errorf("decompressed data exceeds size limit")
	}

	return &bytesReadSeekCloser{Reader: bytes.NewReader(plaintext)}, int64(len(plaintext)), nil
}
