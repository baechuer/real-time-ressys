package middleware

import (
	"net/http"

	"github.com/baechuer/real-time-ressys/services/auth-service/internal/domain"
)

// RequireAtLeast enforces role hierarchy: admin >= moderator >= user.
// Assumes Auth() middleware has already injected role into context.
func RequireAtLeast(minRole string, writeErr WriteErrFunc) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			role, ok := RoleFromContext(r.Context())
			if !ok {
				// Middleware ordering issue (Auth not applied) or context missing
				writeErr(w, r, domain.ErrTokenInvalid())
				return
			}

			if !domain.IsValidRole(role) || !domain.IsValidRole(minRole) {
				// Unknown role or misconfig
				writeErr(w, r, domain.ErrForbidden())
				return
			}

			if domain.RoleRank(role) < domain.RoleRank(minRole) {
				writeErr(w, r, domain.ErrInsufficientRole(minRole))
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
