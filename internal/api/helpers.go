package api

import (
	"encoding/json"
	"fmt"
	"net/http"
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
	return nil
}
