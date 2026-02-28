package middleware

import (
	"bufio"
	"fmt"
	"net"
	"strings"
)

// ParseProxyProtocol parses a PROXY protocol v1 header from the connection.
// Returns the real client IP if found, or the original remote addr.
func ParseProxyProtocol(conn net.Conn) (clientIP string, err error) {
	reader := bufio.NewReader(conn)
	line, err := reader.ReadString('\n')
	if err != nil {
		return "", fmt.Errorf("read proxy protocol header: %w", err)
	}

	line = strings.TrimRight(line, "\r\n")

	// PROXY protocol v1: "PROXY TCP4 srcIP dstIP srcPort dstPort"
	if !strings.HasPrefix(line, "PROXY ") {
		return "", fmt.Errorf("invalid proxy protocol header")
	}

	parts := strings.Fields(line)
	if len(parts) < 6 {
		return "", fmt.Errorf("invalid proxy protocol header: not enough fields")
	}

	proto := parts[1]
	if proto != "TCP4" && proto != "TCP6" {
		return "", fmt.Errorf("unsupported proxy protocol: %s", proto)
	}

	srcIP := parts[2]
	if net.ParseIP(srcIP) == nil {
		return "", fmt.Errorf("invalid source IP: %s", srcIP)
	}

	return srcIP, nil
}
