package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/baechuer/real-time-ressys/services/auth-service/app/services"
	"github.com/redis/go-redis/v9"
)

type ctxKey string

const (
	ctxUserID ctxKey = "userID"
	ctxRoleID ctxKey = "roleID"
)

// TestContextUserID returns the context key for userID (exported for testing)
func TestContextUserID() ctxKey {
	return ctxUserID
}

// JWTAuth creates middleware that validates JWT access tokens and injects user info into context.
func JWTAuth(rdb *redis.Client) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
				http.Error(w, "missing or invalid authorization header", http.StatusUnauthorized)
				return
			}

			tokenStr := strings.TrimPrefix(authHeader, "Bearer ")
			claims, err := services.ValidateAccessToken(r.Context(), rdb, tokenStr)
			if err != nil {
				http.Error(w, "invalid or revoked token", http.StatusUnauthorized)
				return
			}

			ctx := context.WithValue(r.Context(), ctxUserID, claims.UserID)
			ctx = context.WithValue(ctx, ctxRoleID, claims.RoleID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// UserIDFromContext retrieves user ID set by JWTAuth middleware.
func UserIDFromContext(ctx context.Context) (int64, bool) {
	val := ctx.Value(ctxUserID)
	if v, ok := val.(int64); ok {
		return v, true
	}
	return 0, false
}

// RoleIDFromContext retrieves role ID set by JWTAuth middleware.
func RoleIDFromContext(ctx context.Context) (int, bool) {
	val := ctx.Value(ctxRoleID)
	if v, ok := val.(int); ok {
		return v, true
	}
	return 0, false
}

// RequireRoles enforces that the caller's role is in the allowed list.
func RequireRoles(allowed ...int) func(http.Handler) http.Handler {
	allowedSet := make(map[int]struct{}, len(allowed))
	for _, r := range allowed {
		allowedSet[r] = struct{}{}
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			roleID, ok := RoleIDFromContext(r.Context())
			if !ok {
				http.Error(w, "role not found in context", http.StatusForbidden)
				return
			}
			if _, ok := allowedSet[roleID]; !ok {
				http.Error(w, "forbidden", http.StatusForbidden)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
