package utils

import (
	"net"
	"net/http"
	"strings"
)

// GetClientIP extracts the client IP address from an HTTP request,
// checking X-Forwarded-For and X-Real-IP headers first
func GetClientIP(r *http.Request) string {
	// Check X-Forwarded-For header
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		// Take the first IP in the chain
		if comma := strings.Index(xff, ","); comma != -1 {
			xff = xff[:comma]
		}
		xff = strings.TrimSpace(xff)
		if xff != "" {
			return xff
		}
	}

	// Check X-Real-IP header
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return xri
	}

	// Fall back to RemoteAddr
	host, _, _ := net.SplitHostPort(r.RemoteAddr)
	return host
}
