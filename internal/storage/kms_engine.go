package storage

import (
	"bytes"
	"fmt"
	"io"
)

// KMSEncryptedEngine wraps another Engine and encrypts/decrypts data using
// KMS-managed keys (SSE-KMS). Unlike EncryptedEngine which uses a static key,
// this engine fetches data encryption keys from a KMS provider (HashiCorp Vault
// or a local key) and supports key rotation.
type KMSEncryptedEngine struct {
	inner   Engine
	kms     *KMS
	keyName string
}

// NewKMSEncryptedEngine creates an encrypting wrapper using KMS for key management.
func NewKMSEncryptedEngine(inner Engine, kms *KMS, keyName string) (*KMSEncryptedEngine, error) {
	// Validate KMS is reachable by fetching the key once
	if _, err := kms.GetDataKey(keyName); err != nil {
		return nil, fmt.Errorf("KMS key fetch failed: %w", err)
	}
	return &KMSEncryptedEngine{inner: inner, kms: kms, keyName: keyName}, nil
}

func (e *KMSEncryptedEngine) CreateBucketDir(bucket string) error {
	return e.inner.CreateBucketDir(bucket)
}

func (e *KMSEncryptedEngine) DeleteBucketDir(bucket string) error {
	return e.inner.DeleteBucketDir(bucket)
}

func (e *KMSEncryptedEngine) PutObject(bucket, key string, reader io.Reader, size int64) (int64, string, error) {
	if size > maxEncryptedSize {
		return 0, "", fmt.Errorf("object too large for encryption (max %dMB)", maxEncryptedSize/(1024*1024))
	}
	plaintext, err := io.ReadAll(io.LimitReader(reader, maxEncryptedSize+1))
	if err != nil {
		return 0, "", fmt.Errorf("read plaintext: %w", err)
	}

	encrypted, err := e.kms.Encrypt(e.keyName, plaintext)
	if err != nil {
		return 0, "", fmt.Errorf("kms encrypt: %w", err)
	}

	written, etag, err := e.inner.PutObject(bucket, key, bytes.NewReader(encrypted), int64(len(encrypted)))
	if err != nil {
		return 0, "", err
	}
	_ = written
	return int64(len(plaintext)), etag, nil
}

func (e *KMSEncryptedEngine) GetObject(bucket, key string) (ReadSeekCloser, int64, error) {
	reader, _, err := e.inner.GetObject(bucket, key)
	if err != nil {
		return nil, 0, err
	}
	defer reader.Close()

	encrypted, err := io.ReadAll(io.LimitReader(reader, maxEncryptedSize+1024))
	if err != nil {
		return nil, 0, fmt.Errorf("read encrypted: %w", err)
	}

	plaintext, err := e.kms.Decrypt(e.keyName, encrypted)
	if err != nil {
		return nil, 0, fmt.Errorf("kms decrypt: %w", err)
	}

	return &bytesReadSeekCloser{Reader: bytes.NewReader(plaintext)}, int64(len(plaintext)), nil
}

func (e *KMSEncryptedEngine) DeleteObject(bucket, key string) error {
	return e.inner.DeleteObject(bucket, key)
}

func (e *KMSEncryptedEngine) ObjectExists(bucket, key string) bool {
	return e.inner.ObjectExists(bucket, key)
}

func (e *KMSEncryptedEngine) ObjectSize(bucket, key string) (int64, error) {
	return e.inner.ObjectSize(bucket, key)
}

func (e *KMSEncryptedEngine) ListObjects(bucket, prefix, startAfter string, maxKeys int) ([]ObjectInfo, bool, error) {
	return e.inner.ListObjects(bucket, prefix, startAfter, maxKeys)
}

func (e *KMSEncryptedEngine) BucketSize(bucket string) (int64, int64, error) {
	return e.inner.BucketSize(bucket)
}

func (e *KMSEncryptedEngine) PutObjectVersion(bucket, key, versionID string, reader io.Reader, size int64) (int64, string, error) {
	if size > maxEncryptedSize {
		return 0, "", fmt.Errorf("object too large for encryption (max %dMB)", maxEncryptedSize/(1024*1024))
	}
	plaintext, err := io.ReadAll(io.LimitReader(reader, maxEncryptedSize+1))
	if err != nil {
		return 0, "", fmt.Errorf("read plaintext: %w", err)
	}

	encrypted, err := e.kms.Encrypt(e.keyName, plaintext)
	if err != nil {
		return 0, "", fmt.Errorf("kms encrypt: %w", err)
	}

	written, etag, err := e.inner.PutObjectVersion(bucket, key, versionID, bytes.NewReader(encrypted), int64(len(encrypted)))
	if err != nil {
		return 0, "", err
	}
	_ = written
	return int64(len(plaintext)), etag, nil
}

func (e *KMSEncryptedEngine) GetObjectVersion(bucket, key, versionID string) (ReadSeekCloser, int64, error) {
	reader, _, err := e.inner.GetObjectVersion(bucket, key, versionID)
	if err != nil {
		return nil, 0, err
	}
	defer reader.Close()

	encrypted, err := io.ReadAll(io.LimitReader(reader, maxEncryptedSize+1024))
	if err != nil {
		return nil, 0, fmt.Errorf("read encrypted: %w", err)
	}

	plaintext, err := e.kms.Decrypt(e.keyName, encrypted)
	if err != nil {
		return nil, 0, fmt.Errorf("kms decrypt: %w", err)
	}

	return &bytesReadSeekCloser{Reader: bytes.NewReader(plaintext)}, int64(len(plaintext)), nil
}

func (e *KMSEncryptedEngine) DeleteObjectVersion(bucket, key, versionID string) error {
	return e.inner.DeleteObjectVersion(bucket, key, versionID)
}

func (e *KMSEncryptedEngine) DataDir() string {
	return e.inner.DataDir()
}

func (e *KMSEncryptedEngine) ObjectPath(bucket, key string) string {
	return e.inner.ObjectPath(bucket, key)
}
