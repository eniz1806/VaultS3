package s3

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/eniz1806/VaultS3/internal/metadata"
)

// Authenticator validates S3 Signature V4 requests.
type Authenticator struct {
	adminAccessKey string
	adminSecretKey string
	store          *metadata.Store
}

func NewAuthenticator(accessKey, secretKey string, store *metadata.Store) *Authenticator {
	return &Authenticator{
		adminAccessKey: accessKey,
		adminSecretKey: secretKey,
		store:          store,
	}
}

// resolveSecret looks up the secret key for a given access key.
// Checks admin credentials first, then metadata store.
func (a *Authenticator) resolveSecret(accessKey string) (string, error) {
	if accessKey == a.adminAccessKey {
		return a.adminSecretKey, nil
	}
	if a.store != nil {
		if key, err := a.store.GetAccessKey(accessKey); err == nil {
			return key.SecretKey, nil
		}
	}
	return "", fmt.Errorf("invalid access key")
}

// Authenticate validates the Authorization header using AWS Signature V4.
func (a *Authenticator) Authenticate(r *http.Request) error {
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		if r.URL.Query().Get("X-Amz-Signature") != "" {
			return a.authenticatePresigned(r)
		}
		return fmt.Errorf("missing Authorization header")
	}

	if !strings.HasPrefix(authHeader, "AWS4-HMAC-SHA256") {
		return fmt.Errorf("unsupported auth scheme")
	}

	parts := strings.SplitN(authHeader, " ", 2)
	if len(parts) != 2 {
		return fmt.Errorf("malformed auth header")
	}

	params := parseAuthParams(parts[1])
	credential := params["Credential"]
	signedHeaders := params["SignedHeaders"]
	signature := params["Signature"]

	if credential == "" || signedHeaders == "" || signature == "" {
		return fmt.Errorf("missing auth parameters")
	}

	credParts := strings.Split(credential, "/")
	if len(credParts) != 5 {
		return fmt.Errorf("malformed credential")
	}

	reqAccessKey := credParts[0]
	dateStr := credParts[1]
	region := credParts[2]
	service := credParts[3]

	secretKey, err := a.resolveSecret(reqAccessKey)
	if err != nil {
		return err
	}

	canonicalRequest := buildCanonicalRequest(r, signedHeaders)
	stringToSign := buildStringToSign(dateStr, region, service, canonicalRequest, r)
	signingKey := deriveSigningKey(secretKey, dateStr, region, service)
	expectedSig := hex.EncodeToString(hmacSHA256(signingKey, []byte(stringToSign)))

	if !hmac.Equal([]byte(signature), []byte(expectedSig)) {
		return fmt.Errorf("signature mismatch")
	}

	return nil
}

func (a *Authenticator) authenticatePresigned(r *http.Request) error {
	q := r.URL.Query()
	credential := q.Get("X-Amz-Credential")
	signature := q.Get("X-Amz-Signature")
	signedHeaders := q.Get("X-Amz-SignedHeaders")
	dateStr := q.Get("X-Amz-Date")
	expires := q.Get("X-Amz-Expires")

	if credential == "" || signature == "" || dateStr == "" {
		return fmt.Errorf("missing presigned parameters")
	}

	credParts := strings.Split(credential, "/")
	if len(credParts) != 5 {
		return fmt.Errorf("invalid credential")
	}

	if _, err := a.resolveSecret(credParts[0]); err != nil {
		return err
	}

	t, err := time.Parse("20060102T150405Z", dateStr)
	if err != nil {
		return fmt.Errorf("invalid date: %w", err)
	}
	_ = expires
	_ = signedHeaders
	if time.Since(t) > 7*24*time.Hour {
		return fmt.Errorf("presigned URL expired")
	}

	return nil
}

func parseAuthParams(s string) map[string]string {
	params := make(map[string]string)
	for _, part := range strings.Split(s, ", ") {
		kv := strings.SplitN(part, "=", 2)
		if len(kv) == 2 {
			params[strings.TrimSpace(kv[0])] = strings.TrimSpace(kv[1])
		}
	}
	return params
}

func buildCanonicalRequest(r *http.Request, signedHeaders string) string {
	method := r.Method

	uri := r.URL.Path
	if uri == "" {
		uri = "/"
	}

	queryString := r.URL.Query()
	keys := make([]string, 0, len(queryString))
	for k := range queryString {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var queryParts []string
	for _, k := range keys {
		for _, v := range queryString[k] {
			queryParts = append(queryParts, fmt.Sprintf("%s=%s", k, v))
		}
	}
	canonicalQuery := strings.Join(queryParts, "&")

	headerNames := strings.Split(signedHeaders, ";")
	var canonicalHeaders strings.Builder
	for _, name := range headerNames {
		value := strings.TrimSpace(r.Header.Get(name))
		if name == "host" && value == "" {
			value = r.Host
		}
		canonicalHeaders.WriteString(name)
		canonicalHeaders.WriteString(":")
		canonicalHeaders.WriteString(value)
		canonicalHeaders.WriteString("\n")
	}

	payloadHash := r.Header.Get("X-Amz-Content-Sha256")
	if payloadHash == "" {
		payloadHash = "UNSIGNED-PAYLOAD"
	}

	return fmt.Sprintf("%s\n%s\n%s\n%s\n%s\n%s",
		method, uri, canonicalQuery,
		canonicalHeaders.String(), signedHeaders, payloadHash)
}

func buildStringToSign(dateStr, region, service, canonicalRequest string, r *http.Request) string {
	amzDate := r.Header.Get("X-Amz-Date")
	if amzDate == "" {
		amzDate = time.Now().UTC().Format("20060102T150405Z")
	}

	scope := fmt.Sprintf("%s/%s/%s/aws4_request", dateStr, region, service)
	hash := sha256.Sum256([]byte(canonicalRequest))

	return fmt.Sprintf("AWS4-HMAC-SHA256\n%s\n%s\n%s",
		amzDate, scope, hex.EncodeToString(hash[:]))
}

func deriveSigningKey(secretKey, dateStr, region, service string) []byte {
	kDate := hmacSHA256([]byte("AWS4"+secretKey), []byte(dateStr))
	kRegion := hmacSHA256(kDate, []byte(region))
	kService := hmacSHA256(kRegion, []byte(service))
	kSigning := hmacSHA256(kService, []byte("aws4_request"))
	return kSigning
}

func hmacSHA256(key, data []byte) []byte {
	h := hmac.New(sha256.New, key)
	h.Write(data)
	return h.Sum(nil)
}

func (a *Authenticator) GetAccessKey() string {
	return a.adminAccessKey
}

func (a *Authenticator) GetSecretKey() string {
	return a.adminSecretKey
}
