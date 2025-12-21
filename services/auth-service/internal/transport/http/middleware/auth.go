package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/baechuer/real-time-ressys/services/auth-service/internal/application/auth"
	"github.com/baechuer/real-time-ressys/services/auth-service/internal/domain"
)

type TokenVerifier interface {
	VerifyAccessToken(token string) (auth.TokenClaims, error)
}

type WriteErrFunc func(http.ResponseWriter, *http.Request, error)

// UserVersionReader is the minimal interface the middleware needs to validate
// that the access token has not been revoked (via token_version bump).
type UserVersionReader interface {
	// GetTokenVersion returns the current token_version for a user from the source of truth (DB).
	GetTokenVersion(ctx context.Context, userID string) (int64, error)
}

// Auth verifies Authorization: Bearer <access_token>, validates the token_version,
// and injects claims into request context.
func Auth(verifier TokenVerifier, users UserVersionReader, writeErr WriteErrFunc) func(http.Handler) http.Handler {
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

			// Defensive checks
			if strings.TrimSpace(claims.UserID) == "" {
				writeErr(w, r, domain.ErrTokenInvalid())
				return
			}

			// Compare JWT "ver" against the current token_version in DB.
			// If jwt.ver < current_version, the token was revoked.
			if users != nil {
				currentVer, err := users.GetTokenVersion(r.Context(), claims.UserID)
				if err != nil {
					writeErr(w, r, err)
					return
				}

				// claims.Ver must exist in your auth.TokenClaims.
				// If your field is named differently (e.g. TokenVersion), change it here.
				if claims.Ver < currentVer {
					// Use a dedicated domain error if you have one.
					// Otherwise ErrUnauthorized() is fine.
					writeErr(w, r, domain.ErrTokenInvalid())
					return
				}
			}

			// Put identity into context for handlers.
			ctx := WithUser(r.Context(), claims.UserID, claims.Role)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
