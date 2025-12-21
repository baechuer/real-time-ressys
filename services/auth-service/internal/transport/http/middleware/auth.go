package middleware

import (
	"net/http"
	"strings"

	"github.com/baechuer/real-time-ressys/services/auth-service/internal/application/auth"
	"github.com/baechuer/real-time-ressys/services/auth-service/internal/domain"
)

type TokenVerifier interface {
	VerifyAccessToken(token string) (auth.TokenClaims, error)
}

type WriteErrFunc func(http.ResponseWriter, *http.Request, error)

// Auth verifies Authorization: Bearer <access_token> and injects claims into context.
func Auth(verifier TokenVerifier, writeErr WriteErrFunc) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			h := r.Header.Get("Authorization")
			if h == "" {
				writeErr(w, r, domain.ErrTokenMissing())
				return
			}

			parts := strings.SplitN(h, " ", 2)
			if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
				writeErr(w, r, domain.ErrTokenInvalid())
				return
			}

			raw := strings.TrimSpace(parts[1])
			if raw == "" {
				writeErr(w, r, domain.ErrTokenInvalid())
				return
			}

			claims, err := verifier.VerifyAccessToken(raw)
			if err != nil {
				writeErr(w, r, err)
				return
			}

			ctx := WithUser(r.Context(), claims.UserID, claims.Role)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
