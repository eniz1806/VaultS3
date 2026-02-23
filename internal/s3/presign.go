package s3

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// GeneratePresignedURL creates a presigned URL for GET requests.
func GeneratePresignedURL(host, bucket, key, accessKey, secretKey, region string, expires time.Duration) string {
	return generatePresignedURLMethod("GET", host, bucket, key, accessKey, secretKey, region, expires, nil)
}

// PresignedUploadRestrictions defines restrictions for presigned PUT URLs.
type PresignedUploadRestrictions struct {
	MaxSize       int64  // max upload size in bytes (0 = no limit)
	AllowTypes    string // comma-separated allowed content types (empty = any)
	RequirePrefix string // required key prefix (empty = any)
}

// GeneratePresignedPutURL creates a presigned URL for PUT requests with optional restrictions.
func GeneratePresignedPutURL(host, bucket, key, accessKey, secretKey, region string, expires time.Duration, restrictions *PresignedUploadRestrictions) string {
	var extra map[string]string
	if restrictions != nil {
		extra = make(map[string]string)
		if restrictions.MaxSize > 0 {
			extra["X-Vault-MaxSize"] = strconv.FormatInt(restrictions.MaxSize, 10)
		}
		if restrictions.AllowTypes != "" {
			extra["X-Vault-AllowTypes"] = restrictions.AllowTypes
		}
		if restrictions.RequirePrefix != "" {
			extra["X-Vault-RequirePrefix"] = restrictions.RequirePrefix
		}
	}
	return generatePresignedURLMethod("PUT", host, bucket, key, accessKey, secretKey, region, expires, extra)
}

func generatePresignedURLMethod(method, host, bucket, key, accessKey, secretKey, region string, expires time.Duration, extraParams map[string]string) string {
	now := time.Now().UTC()
	dateStr := now.Format("20060102")
	amzDate := now.Format("20060102T150405Z")
	credential := fmt.Sprintf("%s/%s/%s/s3/aws4_request", accessKey, dateStr, region)

	params := url.Values{}
	params.Set("X-Amz-Algorithm", "AWS4-HMAC-SHA256")
	params.Set("X-Amz-Credential", credential)
	params.Set("X-Amz-Date", amzDate)
	params.Set("X-Amz-Expires", strconv.Itoa(int(expires.Seconds())))
	params.Set("X-Amz-SignedHeaders", "host")

	for k, v := range extraParams {
		params.Set(k, v)
	}

	canonicalURI := fmt.Sprintf("/%s/%s", bucket, key)
	canonicalQueryString := params.Encode()
	canonicalHeaders := fmt.Sprintf("host:%s\n", host)
	signedHeaders := "host"

	canonicalRequest := fmt.Sprintf("%s\n%s\n%s\n%s\n%s\nUNSIGNED-PAYLOAD",
		method, canonicalURI, canonicalQueryString, canonicalHeaders, signedHeaders)

	hash := sha256.Sum256([]byte(canonicalRequest))
	scope := fmt.Sprintf("%s/%s/s3/aws4_request", dateStr, region)
	stringToSign := fmt.Sprintf("AWS4-HMAC-SHA256\n%s\n%s\n%s",
		amzDate, scope, hex.EncodeToString(hash[:]))

	signingKey := deriveSigningKey(secretKey, dateStr, region, "s3")
	signature := hex.EncodeToString(hmacSHA256(signingKey, []byte(stringToSign)))

	params.Set("X-Amz-Signature", signature)

	return fmt.Sprintf("http://%s%s?%s", host, canonicalURI, params.Encode())
}

// ValidatePresignedRestrictions checks presigned upload restrictions on an incoming request.
// Returns nil if restrictions pass, or an error message if violated.
func ValidatePresignedRestrictions(r *http.Request, bucket, key string) error {
	q := r.URL.Query()

	// Only check on presigned PUT requests
	if r.Method != http.MethodPut || q.Get("X-Amz-Signature") == "" {
		return nil
	}

	// Check max size
	if maxSizeStr := q.Get("X-Vault-MaxSize"); maxSizeStr != "" {
		maxSize, err := strconv.ParseInt(maxSizeStr, 10, 64)
		if err == nil && maxSize > 0 && r.ContentLength > maxSize {
			return fmt.Errorf("upload exceeds maximum size limit of %d bytes", maxSize)
		}
	}

	// Check content type whitelist
	if allowTypes := q.Get("X-Vault-AllowTypes"); allowTypes != "" {
		ct := r.Header.Get("Content-Type")
		if ct == "" {
			ct = "application/octet-stream"
		}
		// Strip charset etc for comparison
		if idx := strings.Index(ct, ";"); idx >= 0 {
			ct = strings.TrimSpace(ct[:idx])
		}
		allowed := false
		for _, t := range strings.Split(allowTypes, ",") {
			t = strings.TrimSpace(t)
			if strings.EqualFold(ct, t) {
				allowed = true
				break
			}
		}
		if !allowed {
			return fmt.Errorf("content type '%s' not allowed; allowed types: %s", ct, allowTypes)
		}
	}

	// Check required prefix
	if reqPrefix := q.Get("X-Vault-RequirePrefix"); reqPrefix != "" {
		if !strings.HasPrefix(key, reqPrefix) {
			return fmt.Errorf("key must start with prefix '%s'", reqPrefix)
		}
	}

	return nil
}
