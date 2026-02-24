package api

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strings"
)

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func readJSON(r *http.Request, v interface{}) error {
	defer r.Body.Close()
	return json.NewDecoder(r.Body).Decode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

var bucketNameRe = regexp.MustCompile(`^[a-z0-9][a-z0-9.\-]{1,61}[a-z0-9]$`)

// validateBucketName checks DNS-compatible bucket naming rules.
func validateBucketName(name string) error {
	if len(name) < 3 || len(name) > 63 {
		return fmt.Errorf("bucket name must be between 3 and 63 characters")
	}
	if !bucketNameRe.MatchString(name) {
		return fmt.Errorf("bucket name must be lowercase alphanumeric, may contain hyphens and dots, cannot start or end with hyphen/dot")
	}
	if strings.Contains(name, "..") {
		return fmt.Errorf("bucket name must not contain consecutive dots")
	}
	return nil
}

// validateObjectKey checks object key constraints.
func validateObjectKey(key string) error {
	if len(key) == 0 {
		return fmt.Errorf("object key must not be empty")
	}
	if len(key) > 1024 {
		return fmt.Errorf("object key must not exceed 1024 characters")
	}
	if strings.ContainsRune(key, 0) {
		return fmt.Errorf("object key must not contain null bytes")
	}
	// Prevent path traversal
	for _, segment := range strings.Split(key, "/") {
		if segment == ".." {
			return fmt.Errorf("object key must not contain '..' path segments")
		}
	}
	return nil
}

// ValidateWebhookURL checks that a URL is safe to call (prevents SSRF).
func ValidateWebhookURL(rawURL string) error {
	u, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("invalid URL: %w", err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("URL scheme must be http or https")
	}
	host := u.Hostname()
	if host == "" {
		return fmt.Errorf("URL must have a host")
	}
	// Block well-known internal/metadata addresses
	if host == "localhost" || host == "127.0.0.1" || host == "::1" || host == "0.0.0.0" {
		return fmt.Errorf("URL must not point to localhost")
	}
	if strings.HasPrefix(host, "169.254.") || host == "metadata.google.internal" {
		return fmt.Errorf("URL must not point to cloud metadata service")
	}
	// Block private IP ranges
	if ip := net.ParseIP(host); ip != nil {
		if ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() {
			return fmt.Errorf("URL must not point to loopback or link-local address")
		}
		privateRanges := []string{"10.0.0.0/8", "172.16.0.0/12", "192.168.0.0/16", "fc00::/7"}
		for _, cidr := range privateRanges {
			_, network, _ := net.ParseCIDR(cidr)
			if network.Contains(ip) {
				return fmt.Errorf("URL must not point to private network (%s)", cidr)
			}
		}
	}
	return nil
}
