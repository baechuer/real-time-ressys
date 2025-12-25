package response

import "net/http"

// RequestIDFromRequest extracts request id from HTTP headers.
// If you add a request-id middleware later, keep using the same header key.
func RequestIDFromRequest(r *http.Request) string {
	// Common conventions: X-Request-Id / X-Request-ID
	if v := r.Header.Get("X-Request-Id"); v != "" {
		return v
	}
	return r.Header.Get("X-Request-ID")
}
