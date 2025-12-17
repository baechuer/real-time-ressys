package middleware

import (
	"net/http"
	"os"
)

// SecurityHeaders creates middleware that sets security-related HTTP headers
// to protect against XSS, clickjacking, MIME sniffing, and other attacks.
func SecurityHeaders() func(http.Handler) http.Handler {
	// Check if security headers are enabled (default: true)
	enabled := isSecurityHeadersEnabled()
	
	// Get environment for HSTS
	environment := os.Getenv("ENVIRONMENT")
	isProduction := environment == "production"
	
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !enabled {
				next.ServeHTTP(w, r)
				return
			}

			// XSS Protection (legacy browser support)
			w.Header().Set("X-XSS-Protection", "1; mode=block")

			// Prevent MIME type sniffing
			w.Header().Set("X-Content-Type-Options", "nosniff")

			// Prevent clickjacking attacks
			w.Header().Set("X-Frame-Options", "DENY")

			// Referrer Policy - control referrer information
			w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")

			// Permissions Policy (formerly Feature Policy)
			// Restrict access to browser features
			w.Header().Set("Permissions-Policy", 
				"geolocation=(), microphone=(), camera=(), payment=(), usb=(), magnetometer=(), gyroscope=(), speaker=()")

			// Content Security Policy (CSP) - XSS prevention
			// Allow same-origin, inline scripts/styles for API (can be tightened)
			csp := "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data: https:; font-src 'self' data:; connect-src 'self'; frame-ancestors 'none'; base-uri 'self'; form-action 'self'"
			w.Header().Set("Content-Security-Policy", csp)

			// Strict Transport Security (HSTS) - only in production with HTTPS
			if isProduction {
				// 1 year, include subdomains, preload
				w.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains; preload")
			}

			// Remove server information (security through obscurity)
			// This is typically done at reverse proxy level, but good to set here too
			// Note: We can't remove headers that handlers might set later,
			// but we ensure it's not set by default

			next.ServeHTTP(w, r)
		})
	}
}

// isSecurityHeadersEnabled checks if security headers are enabled via environment variable.
// Defaults to true if not set (enabled by default).
func isSecurityHeadersEnabled() bool {
	enabledStr := os.Getenv("SECURITY_HEADERS_ENABLED")
	if enabledStr == "" {
		return true // Default: enabled
	}
	return enabledStr == "true" || enabledStr == "1"
}

