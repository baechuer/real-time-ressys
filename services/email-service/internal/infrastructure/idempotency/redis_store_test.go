package idempotency

import (
	"context"
	"testing"
	"time"

	"github.com/gomodule/redigo/redis"
	"github.com/rafaeljusto/redigomock/v3"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
)

func TestMarkSentNX(t *testing.T) {
	// 1. Setup Mock Connection
	mockConn := redigomock.NewConn()

	// Create a pool that returns our mock connection
	pool := &redis.Pool{
		Dial: func() (redis.Conn, error) {
			return mockConn, nil
		},
	}

	logger := zerolog.New(nil) // Quiet logger for tests
	store := NewRedisStore(pool, logger)
	ctx := context.Background()
	testKey := "msg:123"

	t.Run("success_first_time", func(t *testing.T) {
		// Expect SET msg:123 1 NX EX 60
		mockConn.Command("SET", testKey, "1", "NX", "EX", int64(60)).Expect("OK")

		success, err := store.MarkSentNX(ctx, testKey, 60*time.Second)
		assert.NoError(t, err)
		assert.True(t, success)
	})

	t.Run("already_exists", func(t *testing.T) {
		// Redis returns nil for NX if key exists
		mockConn.Command("SET", testKey, "1", "NX", "EX", int64(60)).ExpectError(redis.ErrNil)

		success, err := store.MarkSentNX(ctx, testKey, 60*time.Second)
		assert.NoError(t, err)
		assert.False(t, success) // Key exists, should return false
	})

	t.Run("empty_key_error", func(t *testing.T) {
		success, err := store.MarkSentNX(ctx, "", 60*time.Second)
		assert.Error(t, err)
		assert.False(t, success)
	})
}

func TestSeen(t *testing.T) {
	mockConn := redigomock.NewConn()
	pool := &redis.Pool{
		Dial: func() (redis.Conn, error) { return mockConn, nil },
	}
	store := NewRedisStore(pool, zerolog.New(nil))
	ctx := context.Background()

	t.Run("key_exists", func(t *testing.T) {
		mockConn.Command("EXISTS", "key1").Expect(int64(1))

		seen, err := store.Seen(ctx, "key1")
		assert.NoError(t, err)
		assert.True(t, seen)
	})

	t.Run("key_not_found", func(t *testing.T) {
		mockConn.Command("EXISTS", "key2").Expect(int64(0))

		seen, err := store.Seen(ctx, "key2")
		assert.NoError(t, err)
		assert.False(t, seen)
	})
}

func TestMarkSent(t *testing.T) {
	mockConn := redigomock.NewConn()
	pool := &redis.Pool{
		Dial: func() (redis.Conn, error) { return mockConn, nil },
	}
	store := NewRedisStore(pool, zerolog.New(nil))
	ctx := context.Background()

	t.Run("force_set_success", func(t *testing.T) {
		mockConn.Command("SET", "key1", "1", "EX", int64(3600)).Expect("OK")

		err := store.MarkSent(ctx, "key1", 1*time.Hour)
		assert.NoError(t, err)
	})
}
