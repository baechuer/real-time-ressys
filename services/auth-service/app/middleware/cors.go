package middleware

import (
	"net/http"
	"os"
	"strings"
)

// CORS creates middleware that handles Cross-Origin Resource Sharing (CORS) headers.
// It allows cross-origin requests from configured origins.
// Can be disabled by setting CORS_ENABLED=false environment variable.
func CORS() func(http.Handler) http.Handler {
	// Check if CORS is enabled
	if !isCORSEnabled() {
		// Return passthrough middleware if disabled
		return func(next http.Handler) http.Handler {
			return next
		}
	}

	allowedOrigins := getAllowedOrigins()
	allowedMethods := []string{"GET", "POST", "PUT", "DELETE", "PATCH", "OPTIONS"}
	allowedHeaders := []string{
		"Accept",
		"Authorization",
		"Content-Type",
		"X-CSRF-Token", // For CSRF token if you add it later
	}
	exposedHeaders := []string{"Authorization"}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")

			// Check if origin is allowed
			if isOriginAllowed(origin, allowedOrigins) {
				w.Header().Set("Access-Control-Allow-Origin", origin)
				w.Header().Set("Access-Control-Allow-Credentials", "true")
			}

			// Handle preflight requests
			if r.Method == "OPTIONS" {
				w.Header().Set("Access-Control-Allow-Methods", strings.Join(allowedMethods, ", "))
				w.Header().Set("Access-Control-Allow-Headers", strings.Join(allowedHeaders, ", "))
				w.Header().Set("Access-Control-Expose-Headers", strings.Join(exposedHeaders, ", "))
				w.Header().Set("Access-Control-Max-Age", "3600") // Cache preflight for 1 hour
				w.WriteHeader(http.StatusNoContent)
				return
			}

			// Set exposed headers for actual requests
			if isOriginAllowed(origin, allowedOrigins) {
				w.Header().Set("Access-Control-Expose-Headers", strings.Join(exposedHeaders, ", "))
			}

			next.ServeHTTP(w, r)
		})
	}
}

// getAllowedOrigins returns the list of allowed origins from environment variable.
// Format: comma-separated list, e.g., "http://localhost:3000,https://app.example.com"
// If not set, defaults to allowing all origins (not recommended for production).
func getAllowedOrigins() []string {
	originsStr := os.Getenv("CORS_ALLOWED_ORIGINS")
	if originsStr == "" {
		// Default: allow all origins (development only)
		// In production, you MUST set CORS_ALLOWED_ORIGINS
		return []string{"*"}
	}

	origins := strings.Split(originsStr, ",")
	for i := range origins {
		origins[i] = strings.TrimSpace(origins[i])
	}
	return origins
}

// isOriginAllowed checks if the given origin is in the allowed list.
func isOriginAllowed(origin string, allowedOrigins []string) bool {
	if origin == "" {
		return false
	}

	for _, allowed := range allowedOrigins {
		if allowed == "*" {
			return true // Allow all (development only)
		}
		if allowed == origin {
			return true
		}
		// Support wildcard subdomains: *.example.com
		// Matches app.example.com, www.example.com, but NOT example.com
		if strings.HasPrefix(allowed, "*.") {
			domain := strings.TrimPrefix(allowed, "*.")
			if strings.HasSuffix(origin, domain) {
				// Ensure it's actually a subdomain (has a dot before the domain)
				// e.g., "https://app.example.com" matches, but "https://example.com" doesn't
				prefix := strings.TrimSuffix(origin, domain)
				if prefix != "" && strings.HasSuffix(prefix, ".") {
					return true
				}
			}
		}
	}

	return false
}

// isCORSEnabled checks if CORS is enabled via environment variable.
// Defaults to true if not set (enabled by default).
func isCORSEnabled() bool {
	enabledStr := os.Getenv("CORS_ENABLED")
	if enabledStr == "" {
		return true // Default: enabled
	}
	return strings.ToLower(enabledStr) == "true"
}
