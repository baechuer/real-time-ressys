package middleware

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/baechuer/real-time-ressys/services/event-service/internal/transport/http/response"
)

type ctxKey string

const (
	ctxUserID ctxKey = "user_id"
	ctxRole   ctxKey = "role"
	ctxVer    ctxKey = "ver"
)

type Claims struct {
	UserID string `json:"uid"`
	Role   string `json:"role"`
	Ver    int64  `json:"ver"`
	jwt.RegisteredClaims
}

type AuthMiddleware struct {
	secret []byte
	issuer string
}

func NewAuth(secret, issuer string) *AuthMiddleware {
	return &AuthMiddleware{secret: []byte(secret), issuer: issuer}
}

func (a *AuthMiddleware) Require(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		uid, role, ver, err := a.parse(r)
		if err != nil {
			// auth-service style error body
			response.Fail(
				w,
				http.StatusUnauthorized,
				"unauthorized",
				"unauthorized",
				map[string]string{"reason": err.Error()},
				response.RequestIDFromRequest(r),
			)
			return
		}

		ctx := context.WithValue(r.Context(), ctxUserID, uid)
		ctx = context.WithValue(ctx, ctxRole, role)
		ctx = context.WithValue(ctx, ctxVer, ver)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (a *AuthMiddleware) parse(r *http.Request) (string, string, int64, error) {
	h := strings.TrimSpace(r.Header.Get("Authorization"))
	if !strings.HasPrefix(h, "Bearer ") {
		return "", "", 0, errors.New("missing bearer token")
	}
	raw := strings.TrimSpace(strings.TrimPrefix(h, "Bearer "))

	claims := &Claims{}
	tok, err := jwt.ParseWithClaims(raw, claims, func(t *jwt.Token) (any, error) {
		if t.Method.Alg() != jwt.SigningMethodHS256.Alg() {
			return nil, errors.New("unexpected signing method")
		}
		return a.secret, nil
	}, jwt.WithLeeway(30*time.Second))
	if err != nil || !tok.Valid {
		return "", "", 0, errors.New("invalid token")
	}

	if a.issuer != "" && claims.Issuer != a.issuer {
		return "", "", 0, errors.New("invalid issuer")
	}
	if strings.TrimSpace(claims.UserID) == "" {
		return "", "", 0, errors.New("missing uid")
	}
	role := strings.TrimSpace(claims.Role)
	if role == "" {
		role = "user"
	}
	return claims.UserID, role, claims.Ver, nil
}

func UserID(r *http.Request) string {
	if v, ok := r.Context().Value(ctxUserID).(string); ok {
		return v
	}
	return ""
}

func Role(r *http.Request) string {
	if v, ok := r.Context().Value(ctxRole).(string); ok {
		return v
	}
	return ""
}

func Ver(r *http.Request) int64 {
	if v, ok := r.Context().Value(ctxVer).(int64); ok {
		return v
	}
	return 0
}
