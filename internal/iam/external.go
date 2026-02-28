package iam

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// ExternalAuthConfig holds configuration for external auth webhook.
type ExternalAuthConfig struct {
	URL     string        `json:"url" yaml:"url"`
	Timeout time.Duration `json:"timeout" yaml:"timeout"`
}

// ExternalAuth calls an external webhook for access decisions.
type ExternalAuth struct {
	cfg    ExternalAuthConfig
	client *http.Client
}

// NewExternalAuth creates a new external auth plugin.
func NewExternalAuth(cfg ExternalAuthConfig) *ExternalAuth {
	timeout := cfg.Timeout
	if timeout == 0 {
		timeout = 5 * time.Second
	}
	return &ExternalAuth{
		cfg:    cfg,
		client: &http.Client{Timeout: timeout},
	}
}

// AuthRequest is sent to the external auth webhook.
type AuthRequest struct {
	AccessKey string `json:"accessKey"`
	Action    string `json:"action"`
	Resource  string `json:"resource"`
	SourceIP  string `json:"sourceIP,omitempty"`
}

// AuthResponse is expected from the external auth webhook.
type AuthResponse struct {
	Allow  bool   `json:"allow"`
	Reason string `json:"reason,omitempty"`
}

// Authorize calls the webhook to check access.
func (e *ExternalAuth) Authorize(accessKey, action, resource, sourceIP string) (bool, error) {
	reqBody := AuthRequest{
		AccessKey: accessKey,
		Action:    action,
		Resource:  resource,
		SourceIP:  sourceIP,
	}
	body, err := json.Marshal(reqBody)
	if err != nil {
		return false, err
	}

	resp, err := e.client.Post(e.cfg.URL, "application/json", bytes.NewReader(body))
	if err != nil {
		return false, fmt.Errorf("external auth webhook error: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return false, fmt.Errorf("external auth webhook returned %d", resp.StatusCode)
	}

	var authResp AuthResponse
	if err := json.NewDecoder(resp.Body).Decode(&authResp); err != nil {
		return false, fmt.Errorf("external auth webhook invalid response: %w", err)
	}

	return authResp.Allow, nil
}
