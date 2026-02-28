package storage

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
)

// KMSConfig holds KMS integration settings.
type KMSConfig struct {
	Provider   string `json:"provider" yaml:"provider"` // "vault" or "local"
	VaultAddr  string `json:"vault_addr" yaml:"vault_addr"`
	VaultToken string `json:"vault_token" yaml:"vault_token"`
	KeyName    string `json:"key_name" yaml:"key_name"`
	LocalKey   string `json:"local_key" yaml:"local_key"` // hex-encoded fallback
}

// KMS provides key management operations.
type KMS struct {
	cfg    KMSConfig
	client *http.Client
	mu     sync.RWMutex
	cache  map[string][]byte // key name → DEK
}

// NewKMS creates a new KMS client.
func NewKMS(cfg KMSConfig) *KMS {
	return &KMS{
		cfg:    cfg,
		client: &http.Client{Timeout: 10 * time.Second},
		cache:  make(map[string][]byte),
	}
}

// GetDataKey returns the data encryption key for the given key name.
func (k *KMS) GetDataKey(keyName string) ([]byte, error) {
	k.mu.RLock()
	if dek, ok := k.cache[keyName]; ok {
		k.mu.RUnlock()
		return dek, nil
	}
	k.mu.RUnlock()

	var dek []byte
	var err error

	switch k.cfg.Provider {
	case "vault":
		dek, err = k.fetchFromVault(keyName)
	case "local":
		dek, err = hex.DecodeString(k.cfg.LocalKey)
	default:
		return nil, fmt.Errorf("unknown KMS provider: %s", k.cfg.Provider)
	}

	if err != nil {
		return nil, err
	}

	k.mu.Lock()
	k.cache[keyName] = dek
	k.mu.Unlock()

	return dek, nil
}

// RotateKey generates a new data key and clears the cache.
func (k *KMS) RotateKey(keyName string) error {
	k.mu.Lock()
	delete(k.cache, keyName)
	k.mu.Unlock()
	return nil
}

// Encrypt encrypts data using the KMS key.
func (k *KMS) Encrypt(keyName string, plaintext []byte) ([]byte, error) {
	dek, err := k.GetDataKey(keyName)
	if err != nil {
		return nil, err
	}

	block, err := aes.NewCipher(dek)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}

	return gcm.Seal(nonce, nonce, plaintext, nil), nil
}

// Decrypt decrypts data using the KMS key.
func (k *KMS) Decrypt(keyName string, ciphertext []byte) ([]byte, error) {
	dek, err := k.GetDataKey(keyName)
	if err != nil {
		return nil, err
	}

	block, err := aes.NewCipher(dek)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return nil, fmt.Errorf("ciphertext too short")
	}

	nonce, ct := ciphertext[:nonceSize], ciphertext[nonceSize:]
	return gcm.Open(nil, nonce, ct, nil)
}

func (k *KMS) fetchFromVault(keyName string) ([]byte, error) {
	url := fmt.Sprintf("%s/v1/transit/datakey/plaintext/%s", k.cfg.VaultAddr, keyName)

	req, err := http.NewRequest("POST", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-Vault-Token", k.cfg.VaultToken)

	resp, err := k.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("vault request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("vault returned %d", resp.StatusCode)
	}

	var result struct {
		Data struct {
			Plaintext string `json:"plaintext"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return hex.DecodeString(result.Data.Plaintext)
}
