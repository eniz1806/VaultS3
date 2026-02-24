package api

import (
	"crypto"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"strings"
	"sync"
	"time"
)

// OIDCClaims represents decoded claims from an OIDC ID token.
type OIDCClaims struct {
	Iss    string      `json:"iss"`
	Sub    string      `json:"sub"`
	Aud    interface{} `json:"aud"` // string or []string
	Exp    int64       `json:"exp"`
	Iat    int64       `json:"iat"`
	Email  string      `json:"email"`
	Name   string      `json:"name"`
	Groups []string    `json:"groups"`
	HD     string      `json:"hd"` // Google hosted domain
}

// Audiences returns the audience claim as a string slice.
func (c *OIDCClaims) Audiences() []string {
	switch v := c.Aud.(type) {
	case string:
		return []string{v}
	case []interface{}:
		var auds []string
		for _, a := range v {
			if s, ok := a.(string); ok {
				auds = append(auds, s)
			}
		}
		return auds
	}
	return nil
}

type oidcJWTHeader struct {
	Alg string `json:"alg"`
	Kid string `json:"kid"`
	Typ string `json:"typ"`
}

type jwksKey struct {
	Kty string `json:"kty"`
	Kid string `json:"kid"`
	Use string `json:"use"`
	Alg string `json:"alg"`
	N   string `json:"n"`
	E   string `json:"e"`
}

type jwksResponse struct {
	Keys []jwksKey `json:"keys"`
}

type openidConfig struct {
	Issuer  string `json:"issuer"`
	JWKSURI string `json:"jwks_uri"`
}

// OIDCValidator validates OIDC ID tokens using JWKS.
type OIDCValidator struct {
	issuerURL      string
	clientID       string
	allowedDomains []string
	jwksURI        string
	keys           map[string]*rsa.PublicKey
	keysMu         sync.RWMutex
	lastFetch      time.Time
	cacheDuration  time.Duration
	httpClient     *http.Client
}

// NewOIDCValidator creates a validator and fetches the discovery document.
func NewOIDCValidator(issuerURL, clientID string, allowedDomains []string, cacheSecs int) (*OIDCValidator, error) {
	v := &OIDCValidator{
		issuerURL:      strings.TrimRight(issuerURL, "/"),
		clientID:       clientID,
		allowedDomains: allowedDomains,
		keys:           make(map[string]*rsa.PublicKey),
		cacheDuration:  time.Duration(cacheSecs) * time.Second,
		httpClient:     &http.Client{Timeout: 10 * time.Second},
	}

	if err := v.fetchDiscovery(); err != nil {
		return nil, fmt.Errorf("oidc discovery: %w", err)
	}

	if err := v.refreshKeys(); err != nil {
		return nil, fmt.Errorf("oidc jwks: %w", err)
	}

	return v, nil
}

func (v *OIDCValidator) fetchDiscovery() error {
	url := v.issuerURL + "/.well-known/openid-configuration"
	resp, err := v.httpClient.Get(url)
	if err != nil {
		return fmt.Errorf("fetch %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("discovery returned %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return err
	}

	var cfg openidConfig
	if err := json.Unmarshal(body, &cfg); err != nil {
		return fmt.Errorf("parse discovery: %w", err)
	}

	if cfg.JWKSURI == "" {
		return fmt.Errorf("discovery missing jwks_uri")
	}

	v.jwksURI = cfg.JWKSURI
	return nil
}

func (v *OIDCValidator) refreshKeys() error {
	resp, err := v.httpClient.Get(v.jwksURI)
	if err != nil {
		return fmt.Errorf("fetch jwks: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("jwks returned %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return err
	}

	var jwks jwksResponse
	if err := json.Unmarshal(body, &jwks); err != nil {
		return fmt.Errorf("parse jwks: %w", err)
	}

	keys := make(map[string]*rsa.PublicKey)
	for _, k := range jwks.Keys {
		if k.Kty != "RSA" || (k.Use != "" && k.Use != "sig") {
			continue
		}
		pub, err := parseRSAPublicKey(k.N, k.E)
		if err != nil {
			continue
		}
		keys[k.Kid] = pub
	}

	v.keysMu.Lock()
	v.keys = keys
	v.lastFetch = time.Now()
	v.keysMu.Unlock()

	return nil
}

func parseRSAPublicKey(nB64, eB64 string) (*rsa.PublicKey, error) {
	nBytes, err := base64.RawURLEncoding.DecodeString(nB64)
	if err != nil {
		return nil, fmt.Errorf("decode n: %w", err)
	}
	eBytes, err := base64.RawURLEncoding.DecodeString(eB64)
	if err != nil {
		return nil, fmt.Errorf("decode e: %w", err)
	}

	n := new(big.Int).SetBytes(nBytes)
	e := 0
	for _, b := range eBytes {
		e = e<<8 + int(b)
	}

	return &rsa.PublicKey{N: n, E: e}, nil
}

// getKey returns the RSA public key for the given kid, refreshing JWKS if needed.
func (v *OIDCValidator) getKey(kid string) (*rsa.PublicKey, error) {
	v.keysMu.RLock()
	key, ok := v.keys[kid]
	expired := time.Since(v.lastFetch) > v.cacheDuration
	v.keysMu.RUnlock()

	if ok && !expired {
		return key, nil
	}

	// Refresh keys if expired or kid not found
	if err := v.refreshKeys(); err != nil {
		if ok {
			return key, nil // use stale key
		}
		return nil, err
	}

	v.keysMu.RLock()
	key, ok = v.keys[kid]
	v.keysMu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("unknown key id: %s", kid)
	}
	return key, nil
}

// ValidateToken decodes and verifies an OIDC ID token.
func (v *OIDCValidator) ValidateToken(idToken string) (*OIDCClaims, error) {
	parts := strings.Split(idToken, ".")
	if len(parts) != 3 {
		return nil, fmt.Errorf("invalid token format")
	}

	// Decode header
	headerBytes, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return nil, fmt.Errorf("decode header: %w", err)
	}
	var header oidcJWTHeader
	if err := json.Unmarshal(headerBytes, &header); err != nil {
		return nil, fmt.Errorf("parse header: %w", err)
	}
	if header.Alg != "RS256" {
		return nil, fmt.Errorf("unsupported algorithm: %s", header.Alg)
	}

	// Get public key
	key, err := v.getKey(header.Kid)
	if err != nil {
		return nil, fmt.Errorf("get key: %w", err)
	}

	// Verify signature
	signingInput := parts[0] + "." + parts[1]
	signature, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return nil, fmt.Errorf("decode signature: %w", err)
	}

	hash := sha256.Sum256([]byte(signingInput))
	if err := rsa.VerifyPKCS1v15(key, crypto.SHA256, hash[:], signature); err != nil {
		return nil, fmt.Errorf("invalid signature")
	}

	// Decode claims
	claimsBytes, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, fmt.Errorf("decode claims: %w", err)
	}
	var claims OIDCClaims
	if err := json.Unmarshal(claimsBytes, &claims); err != nil {
		return nil, fmt.Errorf("parse claims: %w", err)
	}

	// Validate issuer
	if claims.Iss != v.issuerURL {
		return nil, fmt.Errorf("invalid issuer: %s", claims.Iss)
	}

	// Validate audience
	auds := claims.Audiences()
	found := false
	for _, a := range auds {
		if a == v.clientID {
			found = true
			break
		}
	}
	if !found {
		return nil, fmt.Errorf("invalid audience")
	}

	// Validate expiration
	now := time.Now().Unix()
	if now > claims.Exp {
		return nil, fmt.Errorf("token expired")
	}

	// Validate issued-at (reject tokens from the future, with 5min clock skew)
	if claims.Iat > 0 && claims.Iat > now+300 {
		return nil, fmt.Errorf("token issued in the future")
	}

	// Validate domain — if allowedDomains is configured, require a valid email
	if len(v.allowedDomains) > 0 {
		if claims.Email == "" {
			return nil, fmt.Errorf("email claim required when domain filtering is enabled")
		}
		domain := emailDomain(claims.Email)
		allowed := false
		for _, d := range v.allowedDomains {
			if strings.EqualFold(domain, d) {
				allowed = true
				break
			}
		}
		if !allowed {
			return nil, fmt.Errorf("email domain not allowed")
		}
	}

	return &claims, nil
}

func emailDomain(email string) string {
	parts := strings.SplitN(email, "@", 2)
	if len(parts) == 2 {
		return parts[1]
	}
	return ""
}
