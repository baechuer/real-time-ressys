//go:build integration
// +build integration

package redis_test

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/baechuer/real-time-ressys/services/join-service/internal/domain"
	rediscache "github.com/baechuer/real-time-ressys/services/join-service/internal/infrastructure/redis"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

func redisAddrForTest() string {
	// 兼容几种常见命名
	for _, k := range []string{"TEST_REDIS_ADDR", "REDIS_ADDR"} {
		if v := strings.TrimSpace(os.Getenv(k)); v != "" {
			return v
		}
	}
	return "127.0.0.1:6379"
}

func TestRedisCache_EventCapacity_GetSetAndMiss(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cache := rediscache.New(redisAddrForTest(), "", 0)

	eventID := uuid.New()

	// miss
	_, err := cache.GetEventCapacity(ctx, eventID)
	require.True(t, errors.Is(err, domain.ErrCacheMiss))

	// set then get
	require.NoError(t, cache.SetEventCapacity(ctx, eventID, 123))
	got, err := cache.GetEventCapacity(ctx, eventID)
	require.NoError(t, err)
	require.Equal(t, 123, got)

	// closed sentinel
	eventID2 := uuid.New()
	require.NoError(t, cache.SetEventCapacity(ctx, eventID2, -1))
	got2, err := cache.GetEventCapacity(ctx, eventID2)
	require.NoError(t, err)
	require.Equal(t, -1, got2)
}
func testRedisAddr(t *testing.T) string {
	t.Helper()
	addr := os.Getenv("TEST_REDIS_ADDR")
	if addr == "" {
		t.Skip("TEST_REDIS_ADDR not set")
	}
	return addr
}

func TestCache_EventCapacity_GetSet(t *testing.T) {
	addr := testRedisAddr(t)

	rdb := redis.NewClient(&redis.Options{Addr: addr})
	defer rdb.Close()

	require.NoError(t, rdb.Ping(context.Background()).Err())
	require.NoError(t, rdb.FlushDB(context.Background()).Err())

	cache := &rediscache.Cache{Client: rdb}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	eventID := uuid.New()

	_, err := cache.GetEventCapacity(ctx, eventID)
	require.ErrorIs(t, err, domain.ErrCacheMiss)

	require.NoError(t, cache.SetEventCapacity(ctx, eventID, 123))

	got, err := cache.GetEventCapacity(ctx, eventID)
	require.NoError(t, err)
	require.Equal(t, 123, got)
}

func TestCache_AllowRequest_FixedWindow(t *testing.T) {
	addr := testRedisAddr(t)

	rdb := redis.NewClient(&redis.Options{Addr: addr})
	defer rdb.Close()
	require.NoError(t, rdb.FlushDB(context.Background()).Err())

	cache := &rediscache.Cache{Client: rdb}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	ip := "1.2.3.4"
	limit := 3
	window := 2 * time.Second

	for i := 0; i < limit; i++ {
		ok, err := cache.AllowRequest(ctx, ip, limit, window)
		require.NoError(t, err)
		require.True(t, ok)
	}
	ok, err := cache.AllowRequest(ctx, ip, limit, window)
	require.NoError(t, err)
	require.False(t, ok, "4th request should be blocked")

	// wait window => allow again
	time.Sleep(window + 200*time.Millisecond)
	ok, err = cache.AllowRequest(ctx, ip, limit, window)
	require.NoError(t, err)
	require.True(t, ok)
}
