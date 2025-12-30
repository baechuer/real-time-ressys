package middleware

import (
	"net/http"
)

func InternalAuth(secret string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Fail-safe: if secret is effectively empty or default dev-secret in prod (though config should catch that),
			// we still strictly enforce presence.
			if secret == "" {
				http.Error(w, "internal auth misconfigured", http.StatusInternalServerError)
				return
			}

			// Constant-time comparison preferable but simple string compare ok for MVP high entropy secrets
			if r.Header.Get("X-Internal-Secret") != secret {
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
