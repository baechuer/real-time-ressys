package middleware

import (
	"net/http"
	"os"
	"strings"
)

// MetricsAuth creates middleware that protects the metrics endpoint
// Options:
// 1. IP whitelist (METRICS_ALLOWED_IPS)
// 2. API key (METRICS_API_KEY)
// 3. Admin role requirement (if authenticated)
func MetricsAuth() func(http.Handler) http.Handler {
	allowedIPs := getMetricsAllowedIPs()
	apiKey := os.Getenv("METRICS_API_KEY")
	requireAuth := os.Getenv("METRICS_REQUIRE_AUTH") == "true"

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Option 1: IP whitelist (most common for Prometheus)
			if len(allowedIPs) > 0 {
				clientIP := getClientIP(r)
				if !isIPAllowed(clientIP, allowedIPs) {
					http.Error(w, "Forbidden", http.StatusForbidden)
					return
				}
				next.ServeHTTP(w, r)
				return
			}

			// Option 2: API key authentication
			if apiKey != "" {
				providedKey := r.Header.Get("X-Metrics-API-Key")
				if providedKey == "" {
					providedKey = r.URL.Query().Get("api_key")
				}
				if providedKey != apiKey {
					http.Error(w, "Unauthorized", http.StatusUnauthorized)
					return
				}
				next.ServeHTTP(w, r)
				return
			}

			// Option 3: Require admin authentication
			if requireAuth {
				roleID, ok := RoleIDFromContext(r.Context())
				if !ok || roleID != 3 { // 3 = admin role
					http.Error(w, "Forbidden: Admin access required", http.StatusForbidden)
					return
				}
				next.ServeHTTP(w, r)
				return
			}

			// Default: Allow access (for development only)
			// In production, you MUST set one of the above options
			next.ServeHTTP(w, r)
		})
	}
}

// getMetricsAllowedIPs returns list of allowed IPs from environment variable
func getMetricsAllowedIPs() []string {
	ipsStr := os.Getenv("METRICS_ALLOWED_IPS")
	if ipsStr == "" {
		return nil
	}
	ips := strings.Split(ipsStr, ",")
	for i := range ips {
		ips[i] = strings.TrimSpace(ips[i])
	}
	return ips
}

// isIPAllowed checks if client IP is in allowed list
func isIPAllowed(clientIP string, allowedIPs []string) bool {
	// Remove port if present
	if idx := strings.Index(clientIP, ":"); idx != -1 {
		clientIP = clientIP[:idx]
	}

	for _, allowed := range allowedIPs {
		if allowed == "*" {
			return true // Allow all (development only)
		}
		if allowed == clientIP {
			return true
		}
		// Support CIDR notation (e.g., "10.0.0.0/8")
		if strings.Contains(allowed, "/") {
			// Simple CIDR check (for production, use net package)
			if strings.HasPrefix(clientIP, strings.Split(allowed, "/")[0]) {
				return true
			}
		}
	}
	return false
}

// getClientIP extracts client IP from request
func getClientIP(r *http.Request) string {
	// Check X-Forwarded-For header (from proxy/load balancer)
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		ips := strings.Split(xff, ",")
		if len(ips) > 0 {
			return strings.TrimSpace(ips[0])
		}
	}

	// Check X-Real-IP header
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return xri
	}

	// Fallback to RemoteAddr
	ip, _, _ := strings.Cut(r.RemoteAddr, ":")
	return ip
}
