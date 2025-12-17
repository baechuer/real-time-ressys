package config

import (
	"context"
	"os"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

/*
Redis config test cases:
1) NewRedisClient success with custom env (addr, password, db, pools)
2) NewRedisClient uses defaults for optional env when unset
3) NewRedisClient fails when Redis is unreachable
*/

// TestNewRedisClient_SuccessWithCustomEnv verifies client creation with env overrides.
func TestNewRedisClient_SuccessWithCustomEnv(t *testing.T) {
	s, err := miniredis.Run()
	require.NoError(t, err)
	defer s.Close()

	s.RequireAuth("secret")

	t.Setenv("REDIS_ADDR", s.Addr())
	t.Setenv("REDIS_PASSWORD", "secret")
	t.Setenv("REDIS_DB", "2")
	t.Setenv("REDIS_POOL_SIZE", "15")
	t.Setenv("REDIS_MIN_IDLE_CONNS", "3")

	client, err := NewRedisClient()
	require.NoError(t, err)
	defer client.Close()

	opts := client.Options()
	assert.Equal(t, 2, opts.DB)
	assert.Equal(t, "secret", opts.Password)
	assert.Equal(t, 15, opts.PoolSize)
	assert.Equal(t, 3, opts.MinIdleConns)
	assert.NoError(t, client.Ping(context.Background()).Err())
}

// TestNewRedisClient_DefaultsForOptionalEnv ensures defaults are used when optional env vars are unset.
func TestNewRedisClient_DefaultsForOptionalEnv(t *testing.T) {
	// Ensure any pre-existing Redis env is cleared so fallbacks are used.
	_ = os.Unsetenv("REDIS_PASSWORD")
	_ = os.Unsetenv("REDIS_DB")
	_ = os.Unsetenv("REDIS_POOL_SIZE")
	_ = os.Unsetenv("REDIS_MIN_IDLE_CONNS")

	// Bind miniredis to a known address and use it via REDIS_ADDR only.
	s, err := miniredis.Run()
	require.NoError(t, err)
	defer s.Close()

	t.Setenv("REDIS_ADDR", s.Addr())

	client, err := NewRedisClient()
	require.NoError(t, err)
	defer client.Close()

	opts := client.Options()
	assert.Equal(t, "", opts.Password)
	assert.Equal(t, 0, opts.DB)
	assert.Equal(t, 10, opts.PoolSize)
	assert.Equal(t, 2, opts.MinIdleConns)
	assert.NoError(t, client.Ping(context.Background()).Err())
}

// TestNewRedisClient_Unreachable returns error when Redis cannot be reached.
func TestNewRedisClient_Unreachable(t *testing.T) {
	t.Setenv("REDIS_ADDR", "127.0.0.1:6399") // assume nothing listens here
	_ = os.Unsetenv("REDIS_PASSWORD")

	client, err := NewRedisClient()

	assert.Error(t, err, "expected error when Redis is unreachable")
	assert.Nil(t, client)
}
