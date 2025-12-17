package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestRedisRL(t *testing.T) *redis.Client {
	t.Helper()
	mr := miniredis.RunT(t)
	return redis.NewClient(&redis.Options{Addr: mr.Addr()})
}

func TestRateLimit_AllowsWithinCapacity(t *testing.T) {
	rdb := newTestRedisRL(t)
	rl := RouteLimit{Name: "test", Capacity: 2, Window: time.Minute}

	handler := RateLimit(rdb, rl, PrincipalIP())(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/", nil)
	rec1 := httptest.NewRecorder()
	handler.ServeHTTP(rec1, req)
	assert.Equal(t, http.StatusOK, rec1.Code)

	rec2 := httptest.NewRecorder()
	handler.ServeHTTP(rec2, req)
	assert.Equal(t, http.StatusOK, rec2.Code)

	// Third should be limited
	rec3 := httptest.NewRecorder()
	handler.ServeHTTP(rec3, req)
	assert.Equal(t, http.StatusTooManyRequests, rec3.Code)
}

func TestRateLimit_PrincipalUserPreferred(t *testing.T) {
	rdb := newTestRedisRL(t)
	rl := RouteLimit{Name: "test", Capacity: 1, Window: time.Minute}

	handler := RateLimit(rdb, rl, PrincipalUserOrIP())(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// First request with user in context passes
	req := httptest.NewRequest("GET", "/", nil).WithContext(withUserCtx(42, 1))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)

	// Second request same user should be limited
	rec2 := httptest.NewRecorder()
	handler.ServeHTTP(rec2, req)
	assert.Equal(t, http.StatusTooManyRequests, rec2.Code)
}

func TestRateLimit_RetryAfterHeader(t *testing.T) {
	rdb := newTestRedisRL(t)
	rl := RouteLimit{Name: "test", Capacity: 1, Window: time.Second * 10}

	handler := RateLimit(rdb, rl, PrincipalIP())(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/", nil)
	rec1 := httptest.NewRecorder()
	handler.ServeHTTP(rec1, req)
	require.Equal(t, http.StatusOK, rec1.Code)

	rec2 := httptest.NewRecorder()
	handler.ServeHTTP(rec2, req)
	assert.Equal(t, http.StatusTooManyRequests, rec2.Code)
	assert.NotEmpty(t, rec2.Header().Get("Retry-After"))
}

// withUserCtx sets both user and role in context for testing.
func withUserCtx(userID int64, roleID int) context.Context {
	ctx := context.Background()
	ctx = context.WithValue(ctx, ctxUserID, userID)
	ctx = context.WithValue(ctx, ctxRoleID, roleID)
	return ctx
}
