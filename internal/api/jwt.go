package api

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// JWTService implements HS256 JWT signing and validation with zero dependencies.
type JWTService struct {
	secret []byte
}

type jwtHeader struct {
	Alg string `json:"alg"`
	Typ string `json:"typ"`
}

type JWTClaims struct {
	Sub string `json:"sub"`
	Iat int64  `json:"iat"`
	Exp int64  `json:"exp"`
}

func NewJWTService(adminSecret string) *JWTService {
	h := hmac.New(sha256.New, []byte("vaults3-jwt-signing-key"))
	h.Write([]byte(adminSecret))
	return &JWTService{secret: h.Sum(nil)}
}

func (j *JWTService) Generate(subject string, ttl time.Duration) (string, error) {
	header := jwtHeader{Alg: "HS256", Typ: "JWT"}
	claims := JWTClaims{
		Sub: subject,
		Iat: time.Now().Unix(),
		Exp: time.Now().Add(ttl).Unix(),
	}

	headerJSON, err := json.Marshal(header)
	if err != nil {
		return "", err
	}
	claimsJSON, err := json.Marshal(claims)
	if err != nil {
		return "", err
	}

	headerB64 := base64.RawURLEncoding.EncodeToString(headerJSON)
	claimsB64 := base64.RawURLEncoding.EncodeToString(claimsJSON)

	signingInput := headerB64 + "." + claimsB64
	sig := j.sign([]byte(signingInput))
	sigB64 := base64.RawURLEncoding.EncodeToString(sig)

	return signingInput + "." + sigB64, nil
}

func (j *JWTService) Validate(tokenStr string) (*JWTClaims, error) {
	parts := strings.SplitN(tokenStr, ".", 3)
	if len(parts) != 3 {
		return nil, fmt.Errorf("invalid token format")
	}

	// Verify signature
	signingInput := parts[0] + "." + parts[1]
	sig, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return nil, fmt.Errorf("invalid signature encoding")
	}

	expected := j.sign([]byte(signingInput))
	if !hmac.Equal(sig, expected) {
		return nil, fmt.Errorf("invalid signature")
	}

	// Decode claims
	claimsJSON, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, fmt.Errorf("invalid claims encoding")
	}

	var claims JWTClaims
	if err := json.Unmarshal(claimsJSON, &claims); err != nil {
		return nil, fmt.Errorf("invalid claims: %w", err)
	}

	// Check expiry
	if time.Now().Unix() > claims.Exp {
		return nil, fmt.Errorf("token expired")
	}

	return &claims, nil
}

func (j *JWTService) sign(data []byte) []byte {
	h := hmac.New(sha256.New, j.secret)
	h.Write(data)
	return h.Sum(nil)
}
