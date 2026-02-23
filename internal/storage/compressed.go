package storage

import (
	"bytes"
	"compress/gzip"
	"crypto/md5"
	"fmt"
	"io"
)

// CompressedEngine wraps another Engine and compresses/decompresses data transparently.
// Uses gzip compression. Data is compressed before writing and decompressed after reading.
type CompressedEngine struct {
	inner Engine
}

func NewCompressedEngine(inner Engine) *CompressedEngine {
	return &CompressedEngine{inner: inner}
}

func (c *CompressedEngine) CreateBucketDir(bucket string) error {
	return c.inner.CreateBucketDir(bucket)
}

func (c *CompressedEngine) DeleteBucketDir(bucket string) error {
	return c.inner.DeleteBucketDir(bucket)
}

func (c *CompressedEngine) PutObject(bucket, key string, reader io.Reader, size int64) (int64, string, error) {
	return c.compressAndPut(reader, func(compressed io.Reader, compressedSize int64) (int64, string, error) {
		return c.inner.PutObject(bucket, key, compressed, compressedSize)
	})
}

func (c *CompressedEngine) GetObject(bucket, key string) (ReadSeekCloser, int64, error) {
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
	return c.compressAndPut(reader, func(compressed io.Reader, compressedSize int64) (int64, string, error) {
		return c.inner.PutObjectVersion(bucket, key, versionID, compressed, compressedSize)
	})
}

func (c *CompressedEngine) GetObjectVersion(bucket, key, versionID string) (ReadSeekCloser, int64, error) {
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

// compressAndPut reads all data, compresses it, computes ETag of original, writes compressed.
func (c *CompressedEngine) compressAndPut(reader io.Reader, putFn func(io.Reader, int64) (int64, string, error)) (int64, string, error) {
	plaintext, err := io.ReadAll(reader)
	if err != nil {
		return 0, "", fmt.Errorf("read plaintext: %w", err)
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

	plaintext, err := io.ReadAll(gz)
	if err != nil {
		return nil, 0, fmt.Errorf("decompress: %w", err)
	}

	return &bytesReadSeekCloser{Reader: bytes.NewReader(plaintext)}, int64(len(plaintext)), nil
}
