package storage

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"fmt"
	"io"
)

// EncryptedEngine wraps another Engine and encrypts/decrypts data transparently.
// Uses AES-256-GCM with a random 12-byte nonce prepended to the ciphertext.
type EncryptedEngine struct {
	inner Engine
	gcm   cipher.AEAD
}

// NewEncryptedEngine creates an encrypting wrapper around the given engine.
// key must be exactly 32 bytes (256 bits).
func NewEncryptedEngine(inner Engine, key []byte) (*EncryptedEngine, error) {
	if len(key) != 32 {
		return nil, fmt.Errorf("encryption key must be 32 bytes, got %d", len(key))
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("create AES cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("create GCM: %w", err)
	}

	return &EncryptedEngine{inner: inner, gcm: gcm}, nil
}

func (e *EncryptedEngine) CreateBucketDir(bucket string) error {
	return e.inner.CreateBucketDir(bucket)
}

func (e *EncryptedEngine) DeleteBucketDir(bucket string) error {
	return e.inner.DeleteBucketDir(bucket)
}

func (e *EncryptedEngine) PutObject(bucket, key string, reader io.Reader, size int64) (int64, string, error) {
	// Read all plaintext into memory for encryption
	plaintext, err := io.ReadAll(reader)
	if err != nil {
		return 0, "", fmt.Errorf("read plaintext: %w", err)
	}

	// Generate random nonce
	nonce := make([]byte, e.gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return 0, "", fmt.Errorf("generate nonce: %w", err)
	}

	// Encrypt: nonce + ciphertext (includes GCM auth tag)
	ciphertext := e.gcm.Seal(nil, nonce, plaintext, nil)
	encrypted := append(nonce, ciphertext...)

	// Write encrypted data to inner engine
	written, etag, err := e.inner.PutObject(bucket, key, bytes.NewReader(encrypted), int64(len(encrypted)))
	if err != nil {
		return 0, "", err
	}
	_ = written

	// Return original plaintext size and etag
	return int64(len(plaintext)), etag, nil
}

func (e *EncryptedEngine) GetObject(bucket, key string) (ReadSeekCloser, int64, error) {
	reader, _, err := e.inner.GetObject(bucket, key)
	if err != nil {
		return nil, 0, err
	}
	defer reader.Close()

	// Read all encrypted data
	encrypted, err := io.ReadAll(reader)
	if err != nil {
		return nil, 0, fmt.Errorf("read encrypted data: %w", err)
	}

	nonceSize := e.gcm.NonceSize()
	if len(encrypted) < nonceSize {
		return nil, 0, fmt.Errorf("encrypted data too short")
	}

	nonce := encrypted[:nonceSize]
	ciphertext := encrypted[nonceSize:]

	plaintext, err := e.gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, 0, fmt.Errorf("decrypt: %w", err)
	}

	return &bytesReadSeekCloser{Reader: bytes.NewReader(plaintext)}, int64(len(plaintext)), nil
}

func (e *EncryptedEngine) DeleteObject(bucket, key string) error {
	return e.inner.DeleteObject(bucket, key)
}

func (e *EncryptedEngine) ObjectExists(bucket, key string) bool {
	return e.inner.ObjectExists(bucket, key)
}

func (e *EncryptedEngine) ObjectSize(bucket, key string) (int64, error) {
	// For encrypted objects, the file size on disk is larger than the plaintext.
	// We need to decrypt to get the real size, but that's expensive.
	// Return the on-disk size — callers should use metadata for accurate size.
	return e.inner.ObjectSize(bucket, key)
}

func (e *EncryptedEngine) ListObjects(bucket, prefix, startAfter string, maxKeys int) ([]ObjectInfo, bool, error) {
	return e.inner.ListObjects(bucket, prefix, startAfter, maxKeys)
}

func (e *EncryptedEngine) BucketSize(bucket string) (int64, int64, error) {
	return e.inner.BucketSize(bucket)
}

func (e *EncryptedEngine) DataDir() string {
	return e.inner.DataDir()
}

func (e *EncryptedEngine) ObjectPath(bucket, key string) string {
	return e.inner.ObjectPath(bucket, key)
}

// bytesReadSeekCloser wraps a bytes.Reader to implement ReadSeekCloser.
type bytesReadSeekCloser struct {
	*bytes.Reader
}

func (b *bytesReadSeekCloser) Close() error {
	return nil
}
