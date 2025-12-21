package response

import "net/http"

// RequestIDFromContext extracts request_id from context if you have a middleware that sets it.
func RequestIDFromContext(r *http.Request) string {
	if v := r.Context().Value("request_id"); v != nil {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}
