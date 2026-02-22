package s3

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/url"
	"strconv"
	"time"
)

// GeneratePresignedURL creates a presigned URL for GET requests.
func GeneratePresignedURL(host, bucket, key, accessKey, secretKey, region string, expires time.Duration) string {
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

	canonicalURI := fmt.Sprintf("/%s/%s", bucket, key)
	canonicalQueryString := params.Encode()
	canonicalHeaders := fmt.Sprintf("host:%s\n", host)
	signedHeaders := "host"

	canonicalRequest := fmt.Sprintf("GET\n%s\n%s\n%s\n%s\nUNSIGNED-PAYLOAD",
		canonicalURI, canonicalQueryString, canonicalHeaders, signedHeaders)

	hash := sha256.Sum256([]byte(canonicalRequest))
	scope := fmt.Sprintf("%s/%s/s3/aws4_request", dateStr, region)
	stringToSign := fmt.Sprintf("AWS4-HMAC-SHA256\n%s\n%s\n%s",
		amzDate, scope, hex.EncodeToString(hash[:]))

	signingKey := deriveSigningKey(secretKey, dateStr, region, "s3")
	signature := hex.EncodeToString(hmacSHA256(signingKey, []byte(stringToSign)))

	params.Set("X-Amz-Signature", signature)

	return fmt.Sprintf("http://%s%s?%s", host, canonicalURI, params.Encode())
}
