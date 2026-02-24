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
// Supports "*" (matches any sequence of characters) and "?" (matches any single character)
// at any position in the pattern.
func matchWildcard(pattern, value string) bool {
	// Fast path for common cases
	if pattern == "*" {
		return true
	}
	if !strings.ContainsAny(pattern, "*?") {
		return pattern == value
	}
	// DP-based wildcard matching
	return wildcardMatch(pattern, value)
}

// wildcardMatch implements full wildcard matching with * and ? at any position.
// Special case: "prefix/*" also matches "prefix" itself (for resource ARN matching).
func wildcardMatch(pattern, value string) bool {
	// Special case: arn:aws:s3:::bucket/* should match arn:aws:s3:::bucket
	if strings.HasSuffix(pattern, "/*") {
		base := strings.TrimSuffix(pattern, "/*")
		if value == base {
			return true
		}
	}
	p, v := 0, 0
	starP, starV := -1, -1
	for v < len(value) {
		if p < len(pattern) && (pattern[p] == '?' || pattern[p] == value[v]) {
			p++
			v++
		} else if p < len(pattern) && pattern[p] == '*' {
			starP = p
			starV = v
			p++
		} else if starP >= 0 {
			starV++
			v = starV
			p = starP + 1
		} else {
			return false
		}
	}
	for p < len(pattern) && pattern[p] == '*' {
		p++
	}
	return p == len(pattern)
}
