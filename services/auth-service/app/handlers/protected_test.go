package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/alicebob/miniredis/v2"
	authmw "github.com/baechuer/real-time-ressys/services/auth-service/app/middleware"
	"github.com/baechuer/real-time-ressys/services/auth-service/app/services"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestRedis(t *testing.T) *redis.Client {
	t.Helper()
	mr := miniredis.RunT(t)
	return redis.NewClient(&redis.Options{Addr: mr.Addr()})
}

func TestProtectedRoute_AllowsValidToken(t *testing.T) {
	t.Setenv("JWT_SECRET", "supersecret")
	rdb := newTestRedis(t)
	app := &application{redisClient: rdb}

	token, err := services.GenerateAccessToken(123, 2)
	require.NoError(t, err)

	handler := authmw.JWTAuth(rdb)(http.HandlerFunc(app.meHandler))

	req := httptest.NewRequest("GET", "/auth/v1/me", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	var body map[string]interface{}
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&body))
	assert.Equal(t, float64(123), body["user_id"])
	assert.Equal(t, float64(2), body["role_id"])
}

func TestProtectedRoute_MissingToken(t *testing.T) {
	t.Setenv("JWT_SECRET", "supersecret")
	rdb := newTestRedis(t)
	app := &application{redisClient: rdb}

	handler := authmw.JWTAuth(rdb)(http.HandlerFunc(app.meHandler))

	req := httptest.NewRequest("GET", "/auth/v1/me", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

func TestAdminRoute_EnforcesRole(t *testing.T) {
	t.Setenv("JWT_SECRET", "supersecret")
	rdb := newTestRedis(t)
	app := &application{redisClient: rdb}

	userToken, err := services.GenerateAccessToken(200, 1)
	require.NoError(t, err)
	adminToken, err := services.GenerateAccessToken(201, 3)
	require.NoError(t, err)

	handler := authmw.JWTAuth(rdb)(http.HandlerFunc(app.adminOnlyHandler))

	// Non-admin should be forbidden
	reqUser := httptest.NewRequest("GET", "/auth/v1/admin", nil)
	reqUser.Header.Set("Authorization", "Bearer "+userToken)
	rrUser := httptest.NewRecorder()
	handler.ServeHTTP(rrUser, reqUser)
	assert.Equal(t, http.StatusForbidden, rrUser.Code)

	// Admin should pass
	reqAdmin := httptest.NewRequest("GET", "/auth/v1/admin", nil)
	reqAdmin.Header.Set("Authorization", "Bearer "+adminToken)
	rrAdmin := httptest.NewRecorder()
	handler.ServeHTTP(rrAdmin, reqAdmin)
	assert.Equal(t, http.StatusOK, rrAdmin.Code)
}

