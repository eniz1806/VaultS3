package s3

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/eniz1806/VaultS3/internal/iam"
	"github.com/eniz1806/VaultS3/internal/metadata"
)

// Authenticator validates S3 Signature V4 requests.
type Authenticator struct {
	adminAccessKey  string
	adminSecretKey  string
	store           *metadata.Store
	globalAllowCIDR []string
	globalBlockCIDR []string
}

func NewAuthenticator(accessKey, secretKey string, store *metadata.Store, allowCIDR, blockCIDR []string) *Authenticator {
	return &Authenticator{
		adminAccessKey:  accessKey,
		adminSecretKey:  secretKey,
		store:           store,
		globalAllowCIDR: allowCIDR,
		globalBlockCIDR: blockCIDR,
	}
}

// resolveIdentity looks up the identity for a given access key.
// Returns the identity with user info and policies.
func (a *Authenticator) resolveIdentity(accessKey string) (*iam.Identity, string, error) {
	if accessKey == a.adminAccessKey {
		return &iam.Identity{
			AccessKey: accessKey,
			IsAdmin:   true,
		}, a.adminSecretKey, nil
	}
	if a.store != nil {
		if key, err := a.store.GetAccessKey(accessKey); err == nil {
			// Check STS expiration
			if key.ExpiresAt > 0 && time.Now().Unix() > key.ExpiresAt {
				return nil, "", fmt.Errorf("credentials have expired")
			}

			// For STS keys, resolve policies from SourceUserID
			userID := key.UserID
			if key.SourceUserID != "" {
				userID = key.SourceUserID
			}

			identity := &iam.Identity{
				AccessKey: accessKey,
				UserID:    userID,
			}

			// Load policies if linked to a user
			if userID != "" {
				iamPolicies, err := a.store.GetUserPolicies(userID)
				if err == nil {
					for _, p := range iamPolicies {
						var pol iam.Policy
						if err := json.Unmarshal([]byte(p.Document), &pol); err == nil {
							identity.Policies = append(identity.Policies, pol)
						}
					}
				}

				// Load user's IP restrictions
				if user, err := a.store.GetIAMUser(userID); err == nil {
					identity.AllowedCIDRs = user.AllowedCIDRs
				}
			} else {
				// Legacy keys without a user get full access
				identity.IsAdmin = true
			}
			return identity, key.SecretKey, nil
		}
	}
	return nil, "", fmt.Errorf("invalid access key")
}

// CheckIPAccess validates client IP against global and per-user restrictions.
func (a *Authenticator) CheckIPAccess(identity *iam.Identity, clientIP string) error {
	if identity.IsAdmin {
		return nil
	}

	// Combine global and per-user CIDR lists
	blockList := a.globalBlockCIDR
	allowList := a.globalAllowCIDR

	// Per-user restrictions are additive to global
	if len(identity.AllowedCIDRs) > 0 {
		if len(allowList) == 0 {
			allowList = identity.AllowedCIDRs
		} else {
			// Both global and user allowlists — must match at least one from either
			allowList = append(append([]string{}, allowList...), identity.AllowedCIDRs...)
		}
	}

	return iam.CheckIP(clientIP, allowList, blockList)
}

// Authenticate validates the Authorization header using AWS Signature V4.
// Returns the identity of the caller.
func (a *Authenticator) Authenticate(r *http.Request) (*iam.Identity, error) {
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		if r.URL.Query().Get("X-Amz-Signature") != "" {
			return a.authenticatePresigned(r)
		}
		return nil, fmt.Errorf("missing Authorization header")
	}

	if !strings.HasPrefix(authHeader, "AWS4-HMAC-SHA256") {
		return nil, fmt.Errorf("unsupported auth scheme")
	}

	parts := strings.SplitN(authHeader, " ", 2)
	if len(parts) != 2 {
		return nil, fmt.Errorf("malformed auth header")
	}

	params := parseAuthParams(parts[1])
	credential := params["Credential"]
	signedHeaders := params["SignedHeaders"]
	signature := params["Signature"]

	if credential == "" || signedHeaders == "" || signature == "" {
		return nil, fmt.Errorf("missing auth parameters")
	}

	credParts := strings.Split(credential, "/")
	if len(credParts) != 5 {
		return nil, fmt.Errorf("malformed credential")
	}

	reqAccessKey := credParts[0]
	dateStr := credParts[1]
	region := credParts[2]
	service := credParts[3]

	identity, secretKey, err := a.resolveIdentity(reqAccessKey)
	if err != nil {
		return nil, err
	}

	canonicalRequest := buildCanonicalRequest(r, signedHeaders)
	stringToSign := buildStringToSign(dateStr, region, service, canonicalRequest, r)
	signingKey := deriveSigningKey(secretKey, dateStr, region, service)
	expectedSig := hex.EncodeToString(hmacSHA256(signingKey, []byte(stringToSign)))

	if !hmac.Equal([]byte(signature), []byte(expectedSig)) {
		return nil, fmt.Errorf("signature mismatch")
	}

	return identity, nil
}

func (a *Authenticator) authenticatePresigned(r *http.Request) (*iam.Identity, error) {
	q := r.URL.Query()
	credential := q.Get("X-Amz-Credential")
	signature := q.Get("X-Amz-Signature")
	signedHeaders := q.Get("X-Amz-SignedHeaders")
	dateStr := q.Get("X-Amz-Date")
	expires := q.Get("X-Amz-Expires")

	if credential == "" || signature == "" || dateStr == "" {
		return nil, fmt.Errorf("missing presigned parameters")
	}

	credParts := strings.Split(credential, "/")
	if len(credParts) != 5 {
		return nil, fmt.Errorf("invalid credential")
	}

	identity, _, err := a.resolveIdentity(credParts[0])
	if err != nil {
		return nil, err
	}

	t, err := time.Parse("20060102T150405Z", dateStr)
	if err != nil {
		return nil, fmt.Errorf("invalid date: %w", err)
	}
	_ = expires
	_ = signedHeaders
	if time.Since(t) > 7*24*time.Hour {
		return nil, fmt.Errorf("presigned URL expired")
	}

	return identity, nil
}

// Authorize checks if an identity is allowed to perform an action on a resource.
func (a *Authenticator) Authorize(identity *iam.Identity, action, resource string) error {
	if identity.IsAdmin {
		return nil
	}
	if iam.Evaluate(identity.Policies, action, resource) {
		return nil
	}
	return fmt.Errorf("access denied: %s on %s", action, resource)
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
