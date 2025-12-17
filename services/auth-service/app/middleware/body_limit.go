package middleware

import (
	"net/http"
	"os"
	"strconv"
)

// BodyLimit creates middleware that limits the size of request bodies
// to prevent DoS attacks via large payloads.
// Default limit: 1MB (1048576 bytes)
func BodyLimit(maxBytes int64) func(http.Handler) http.Handler {
	if maxBytes <= 0 {
		maxBytes = 1048576 // Default: 1MB
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Limit request body size
			r.Body = http.MaxBytesReader(w, r.Body, maxBytes)

			// Check Content-Length header if present
			if r.ContentLength > maxBytes {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusRequestEntityTooLarge)
				w.Write([]byte(`{"error":"Request body too large","code":"PAYLOAD_TOO_LARGE"}`))
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// BodyLimitFromEnv creates middleware with body size limit from environment variable.
// Environment variable: REQUEST_BODY_MAX_SIZE (in bytes, default: 1048576 = 1MB)
func BodyLimitFromEnv() func(http.Handler) http.Handler {
	maxBytes := getMaxBodySize()
	return BodyLimit(maxBytes)
}

// getMaxBodySize returns the maximum request body size from environment variable.
// Default: 1MB (1048576 bytes)
func getMaxBodySize() int64 {
	maxSizeStr := os.Getenv("REQUEST_BODY_MAX_SIZE")
	if maxSizeStr == "" {
		return 1048576 // Default: 1MB
	}

	maxSize, err := strconv.ParseInt(maxSizeStr, 10, 64)
	if err != nil || maxSize <= 0 {
		return 1048576 // Default on error
	}

	return maxSize
}

