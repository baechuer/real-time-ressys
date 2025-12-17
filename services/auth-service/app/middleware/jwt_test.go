package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/baechuer/real-time-ressys/services/auth-service/app/services"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

/*
JWT middleware test cases:
1) Missing Authorization header → 401
2) Non-Bearer Authorization header → 401
3) Invalid token → 401
4) Revoked token (blacklisted JTI) → 401
5) Valid token injects user_id and role_id and calls next
6) RequireRoles enforces allowed roles
*/

func newTestRedis(t *testing.T) *redis.Client {
	t.Helper()
	mr := miniredis.RunT(t)
	return redis.NewClient(&redis.Options{Addr: mr.Addr()})
}

func TestJWTAuth_MissingHeader(t *testing.T) {
	rdb := newTestRedis(t)
	mw := JWTAuth(rdb)

	req := httptest.NewRequest("GET", "/protected", nil)
	rr := httptest.NewRecorder()

	mw(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})).ServeHTTP(rr, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

func TestJWTAuth_NonBearerHeader(t *testing.T) {
	rdb := newTestRedis(t)
	mw := JWTAuth(rdb)

	req := httptest.NewRequest("GET", "/protected", nil)
	req.Header.Set("Authorization", "Token abc")
	rr := httptest.NewRecorder()

	mw(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})).ServeHTTP(rr, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

func TestJWTAuth_InvalidToken(t *testing.T) {
	t.Setenv("JWT_SECRET", "supersecret")
	rdb := newTestRedis(t)
	mw := JWTAuth(rdb)

	req := httptest.NewRequest("GET", "/protected", nil)
	req.Header.Set("Authorization", "Bearer invalid")
	rr := httptest.NewRecorder()

	mw(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})).ServeHTTP(rr, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

func TestJWTAuth_BlacklistedToken(t *testing.T) {
	t.Setenv("JWT_SECRET", "supersecret")
	rdb := newTestRedis(t)
	mw := JWTAuth(rdb)

	token, err := services.GenerateAccessToken(1, 2)
	require.NoError(t, err)

	// Blacklist the token by JTI
	err = services.BlacklistAccessToken(context.Background(), rdb, token)
	require.NoError(t, err)

	req := httptest.NewRequest("GET", "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()

	mw(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})).ServeHTTP(rr, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

func TestJWTAuth_ValidTokenSetsContext(t *testing.T) {
	t.Setenv("JWT_SECRET", "supersecret")
	rdb := newTestRedis(t)
	mw := JWTAuth(rdb)

	token, err := services.GenerateAccessToken(123, 7)
	require.NoError(t, err)

	req := httptest.NewRequest("GET", "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()

	var capturedUserID int64
	var capturedRoleID int

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		uid, _ := UserIDFromContext(r.Context())
		rid, _ := RoleIDFromContext(r.Context())
		capturedUserID = uid
		capturedRoleID = rid
		w.WriteHeader(http.StatusOK)
	})

	mw(next).ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, int64(123), capturedUserID)
	assert.Equal(t, 7, capturedRoleID)
}

func TestRequireRoles_AllowsAllowedRole(t *testing.T) {
	t.Setenv("JWT_SECRET", "supersecret")
	rdb := newTestRedis(t)
	token, err := services.GenerateAccessToken(10, 3)
	require.NoError(t, err)

	chain := JWTAuth(rdb)(RequireRoles(3)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})))

	req := httptest.NewRequest("GET", "/admin", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()

	chain.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestRequireRoles_RejectsForbiddenRole(t *testing.T) {
	t.Setenv("JWT_SECRET", "supersecret")
	rdb := newTestRedis(t)
	token, err := services.GenerateAccessToken(10, 2)
	require.NoError(t, err)

	chain := JWTAuth(rdb)(RequireRoles(3)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})))

	req := httptest.NewRequest("GET", "/admin", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()

	chain.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusForbidden, rr.Code)
}

