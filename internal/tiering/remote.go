package tiering

import (
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"
)

// RemoteTierConfig configures a remote S3-compatible tier.
type RemoteTierConfig struct {
	Name      string `json:"name" yaml:"name"`
	Endpoint  string `json:"endpoint" yaml:"endpoint"`
	Bucket    string `json:"bucket" yaml:"bucket"`
	AccessKey string `json:"access_key" yaml:"access_key"`
	SecretKey string `json:"secret_key" yaml:"secret_key"`
	Region    string `json:"region" yaml:"region"`
	UseSSL    bool   `json:"use_ssl" yaml:"use_ssl"`
}

// RemoteTier provides operations for a remote S3-compatible cold tier.
type RemoteTier struct {
	cfg    RemoteTierConfig
	client *http.Client
}

// NewRemoteTier creates a new remote tier client.
func NewRemoteTier(cfg RemoteTierConfig) *RemoteTier {
	return &RemoteTier{
		cfg:    cfg,
		client: &http.Client{Timeout: 120 * time.Second},
	}
}

// Upload sends an object to the remote tier.
func (t *RemoteTier) Upload(key string, reader io.Reader, size int64) error {
	scheme := "http"
	if t.cfg.UseSSL {
		scheme = "https"
	}
	url := fmt.Sprintf("%s://%s/%s/%s", scheme, t.cfg.Endpoint, t.cfg.Bucket, key)

	req, err := http.NewRequest(http.MethodPut, url, reader)
	if err != nil {
		return err
	}
	req.ContentLength = size

	resp, err := t.client.Do(req)
	if err != nil {
		return fmt.Errorf("remote tier upload: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("remote tier upload returned %d", resp.StatusCode)
	}

	slog.Debug("uploaded to remote tier", "key", key, "size", size)
	return nil
}

// Download retrieves an object from the remote tier.
func (t *RemoteTier) Download(key string) (io.ReadCloser, int64, error) {
	scheme := "http"
	if t.cfg.UseSSL {
		scheme = "https"
	}
	url := fmt.Sprintf("%s://%s/%s/%s", scheme, t.cfg.Endpoint, t.cfg.Bucket, key)

	resp, err := t.client.Get(url)
	if err != nil {
		return nil, 0, fmt.Errorf("remote tier download: %w", err)
	}

	if resp.StatusCode >= 400 {
		resp.Body.Close()
		return nil, 0, fmt.Errorf("remote tier download returned %d", resp.StatusCode)
	}

	return resp.Body, resp.ContentLength, nil
}

// Delete removes an object from the remote tier.
func (t *RemoteTier) Delete(key string) error {
	scheme := "http"
	if t.cfg.UseSSL {
		scheme = "https"
	}
	url := fmt.Sprintf("%s://%s/%s/%s", scheme, t.cfg.Endpoint, t.cfg.Bucket, key)

	req, err := http.NewRequest(http.MethodDelete, url, nil)
	if err != nil {
		return err
	}

	resp, err := t.client.Do(req)
	if err != nil {
		return fmt.Errorf("remote tier delete: %w", err)
	}
	defer resp.Body.Close()

	return nil
}
