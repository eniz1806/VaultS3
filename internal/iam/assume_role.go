package iam

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"
)

// Role represents an IAM role that can be assumed.
type Role struct {
	Name        string   `json:"name"`
	ARN         string   `json:"arn"`
	PolicyARNs  []string `json:"policy_arns"`
	TrustPolicy Policy   `json:"trust_policy"`
	MaxDuration int      `json:"max_duration_secs,omitempty"` // default 3600
	CreatedAt   time.Time `json:"created_at"`
}

// AssumeRoleRequest contains the parameters for assuming a role.
type AssumeRoleRequest struct {
	RoleARN         string `json:"role_arn"`
	SessionName     string `json:"session_name"`
	DurationSecs    int    `json:"duration_secs,omitempty"`
	InlinePolicy    string `json:"inline_policy,omitempty"`
	ExternalID      string `json:"external_id,omitempty"`
	CallerAccessKey string `json:"-"` // set by caller
}

// AssumeRoleResult contains the temporary credentials from assuming a role.
type AssumeRoleResult struct {
	AccessKey    string    `json:"access_key"`
	SecretKey    string    `json:"secret_key"`
	SessionToken string    `json:"session_token"`
	Expiration   time.Time `json:"expiration"`
	AssumedRole  string    `json:"assumed_role"`
}

// RoleStore is the interface for role persistence.
type RoleStore interface {
	GetRole(name string) (*Role, error)
}

// AssumeRole validates the trust policy and generates temporary credentials.
func AssumeRole(req AssumeRoleRequest, role *Role) (*AssumeRoleResult, error) {
	// Validate trust policy
	if !Evaluate([]Policy{role.TrustPolicy}, "sts:AssumeRole", role.ARN) {
		return nil, fmt.Errorf("not authorized to assume role %s", role.ARN)
	}

	duration := req.DurationSecs
	if duration <= 0 {
		duration = 3600
	}
	maxDuration := role.MaxDuration
	if maxDuration <= 0 {
		maxDuration = 43200
	}
	if duration > maxDuration {
		return nil, fmt.Errorf("requested duration %d exceeds maximum %d", duration, maxDuration)
	}

	accessKey, err := randomHexStr(10)
	if err != nil {
		return nil, err
	}
	secretKey, err := randomHexStr(20)
	if err != nil {
		return nil, err
	}
	sessionToken, err := randomHexStr(16)
	if err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	return &AssumeRoleResult{
		AccessKey:    accessKey,
		SecretKey:    secretKey,
		SessionToken: sessionToken,
		Expiration:   now.Add(time.Duration(duration) * time.Second),
		AssumedRole:  role.ARN,
	}, nil
}

func randomHexStr(nBytes int) (string, error) {
	b := make([]byte, nBytes)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
