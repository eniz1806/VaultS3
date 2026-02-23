package iam

import (
	"fmt"
	"net"
	"strings"
)

// CheckIP validates a client IP against allow and block CIDR lists.
// blockedCIDRs are checked first (deny wins).
// If allowedCIDRs is non-empty, the IP must match at least one.
// If allowedCIDRs is empty, all IPs are allowed (unless blocked).
func CheckIP(clientIP string, allowedCIDRs, blockedCIDRs []string) error {
	// Strip port if present
	host := clientIP
	if h, _, err := net.SplitHostPort(clientIP); err == nil {
		host = h
	}
	host = strings.TrimSpace(host)

	ip := net.ParseIP(host)
	if ip == nil {
		return fmt.Errorf("invalid client IP: %s", clientIP)
	}

	// Check blocklist first
	for _, cidr := range blockedCIDRs {
		_, network, err := net.ParseCIDR(cidr)
		if err != nil {
			continue
		}
		if network.Contains(ip) {
			return fmt.Errorf("IP %s is blocked by policy", host)
		}
	}

	// If allowlist is empty, allow all (that aren't blocked)
	if len(allowedCIDRs) == 0 {
		return nil
	}

	// Check allowlist
	for _, cidr := range allowedCIDRs {
		_, network, err := net.ParseCIDR(cidr)
		if err != nil {
			continue
		}
		if network.Contains(ip) {
			return nil
		}
	}

	return fmt.Errorf("IP %s is not in allowed range", host)
}
