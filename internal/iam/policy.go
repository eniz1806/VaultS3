package iam

import "strings"

// Policy represents an IAM policy document.
type Policy struct {
	Version   string      `json:"Version"`
	Statement []Statement `json:"Statement"`
}

// Statement represents a single policy statement.
type Statement struct {
	Effect   string   `json:"Effect"`
	Action   []string `json:"Action"`
	Resource []string `json:"Resource"`
}

// Evaluate checks all statements against an action and resource.
// Returns true if access is allowed, false if denied.
// Logic: explicit Deny wins, then explicit Allow, else default deny.
func Evaluate(policies []Policy, action, resource string) bool {
	hasAllow := false

	for _, pol := range policies {
		for _, stmt := range pol.Statement {
			if !matchesAny(stmt.Action, action) {
				continue
			}
			if !matchesAny(stmt.Resource, resource) {
				continue
			}
			if stmt.Effect == "Deny" {
				return false
			}
			if stmt.Effect == "Allow" {
				hasAllow = true
			}
		}
	}

	return hasAllow
}

// matchesAny checks if the value matches any of the patterns.
func matchesAny(patterns []string, value string) bool {
	for _, p := range patterns {
		if matchWildcard(p, value) {
			return true
		}
	}
	return false
}

// matchWildcard matches a pattern against a value.
// Supports "*" as a wildcard that matches everything, and trailing "*" for prefix matching.
func matchWildcard(pattern, value string) bool {
	if pattern == "*" {
		return true
	}

	// Handle s3:* matching any s3 action
	if strings.HasSuffix(pattern, ":*") {
		prefix := strings.TrimSuffix(pattern, "*")
		return strings.HasPrefix(value, prefix)
	}

	// Handle resource wildcards like arn:aws:s3:::bucket/*
	if strings.HasSuffix(pattern, "/*") {
		prefix := strings.TrimSuffix(pattern, "/*")
		return value == prefix || strings.HasPrefix(value, prefix+"/")
	}

	if strings.HasSuffix(pattern, "*") {
		prefix := strings.TrimSuffix(pattern, "*")
		return strings.HasPrefix(value, prefix)
	}

	return pattern == value
}
