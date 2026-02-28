package iam

import (
	"net"
	"strings"
	"time"
)

// EvaluateWithContext checks policies with additional context for condition evaluation.
func EvaluateWithContext(policies []Policy, action, resource string, ctx map[string]string) bool {
	hasAllow := false

	for _, pol := range policies {
		for _, stmt := range pol.Statement {
			// NotAction: matches if action is NOT in the list
			if len(stmt.NotAction) > 0 {
				if matchesAny(stmt.NotAction, action) {
					continue // NotAction matched means this statement doesn't apply
				}
			} else if !matchesAny(stmt.Action, action) {
				continue
			}

			// NotResource: matches if resource is NOT in the list
			if len(stmt.NotResource) > 0 {
				if matchesAny(stmt.NotResource, resource) {
					continue
				}
			} else if !matchesAny(stmt.Resource, resource) {
				continue
			}

			// Evaluate conditions
			if len(stmt.Condition) > 0 && !evaluateConditions(stmt.Condition, ctx) {
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

// evaluateConditions checks all condition operators.
func evaluateConditions(conditions map[string]map[string][]string, ctx map[string]string) bool {
	for operator, kvs := range conditions {
		for key, values := range kvs {
			ctxVal := ctx[key]
			if !evaluateOperator(operator, ctxVal, values) {
				return false
			}
		}
	}
	return true
}

func evaluateOperator(operator, actual string, expected []string) bool {
	switch operator {
	case "StringEquals":
		return stringEquals(actual, expected)
	case "StringNotEquals":
		return !stringEquals(actual, expected)
	case "StringLike":
		return stringLike(actual, expected)
	case "StringNotLike":
		return !stringLike(actual, expected)
	case "StringEqualsIgnoreCase":
		return stringEqualsIgnoreCase(actual, expected)
	case "IpAddress":
		return ipAddress(actual, expected)
	case "NotIpAddress":
		return !ipAddress(actual, expected)
	case "DateLessThan":
		return dateLessThan(actual, expected)
	case "DateGreaterThan":
		return dateGreaterThan(actual, expected)
	case "Bool":
		return stringEquals(actual, expected)
	default:
		return false // unknown operator = deny
	}
}

func stringEquals(actual string, expected []string) bool {
	for _, v := range expected {
		if actual == v {
			return true
		}
	}
	return false
}

func stringEqualsIgnoreCase(actual string, expected []string) bool {
	for _, v := range expected {
		if strings.EqualFold(actual, v) {
			return true
		}
	}
	return false
}

func stringLike(actual string, expected []string) bool {
	for _, v := range expected {
		if matchWildcard(v, actual) {
			return true
		}
	}
	return false
}

func ipAddress(actual string, cidrs []string) bool {
	ip := net.ParseIP(actual)
	if ip == nil {
		return false
	}
	for _, cidr := range cidrs {
		if strings.Contains(cidr, "/") {
			_, network, err := net.ParseCIDR(cidr)
			if err != nil {
				continue
			}
			if network.Contains(ip) {
				return true
			}
		} else {
			if actual == cidr {
				return true
			}
		}
	}
	return false
}

func dateLessThan(actual string, expected []string) bool {
	t, err := time.Parse(time.RFC3339, actual)
	if err != nil {
		return false
	}
	for _, v := range expected {
		threshold, err := time.Parse(time.RFC3339, v)
		if err != nil {
			continue
		}
		if t.Before(threshold) {
			return true
		}
	}
	return false
}

func dateGreaterThan(actual string, expected []string) bool {
	t, err := time.Parse(time.RFC3339, actual)
	if err != nil {
		return false
	}
	for _, v := range expected {
		threshold, err := time.Parse(time.RFC3339, v)
		if err != nil {
			continue
		}
		if t.After(threshold) {
			return true
		}
	}
	return false
}

// SubstitutePolicyVariables replaces ${aws:*} variables in policy strings.
func SubstitutePolicyVariables(s string, ctx map[string]string) string {
	result := s
	for k, v := range ctx {
		result = strings.ReplaceAll(result, "${"+k+"}", v)
	}
	return result
}
