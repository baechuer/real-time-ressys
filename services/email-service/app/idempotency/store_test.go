package idempotency

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestRedis(t *testing.T) (*redis.Client, func()) {
	mr, err := miniredis.Run()
	require.NoError(t, err)

	client := redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
	})

	return client, func() {
		client.Close()
		mr.Close()
	}
}

func TestStore_Key(t *testing.T) {
	client, cleanup := setupTestRedis(t)
	defer cleanup()

	store := NewStore(client)
	messageID := "test-message-123"
	expectedKey := "email:processed:test-message-123"

	assert.Equal(t, expectedKey, store.Key(messageID))
}

func TestStore_IsProcessed_NotProcessed(t *testing.T) {
	client, cleanup := setupTestRedis(t)
	defer cleanup()

	store := NewStore(client)
	ctx := context.Background()

	processed, err := store.IsProcessed(ctx, "new-message-id")
	require.NoError(t, err)
	assert.False(t, processed)
}

func TestStore_IsProcessed_AlreadyProcessed(t *testing.T) {
	client, cleanup := setupTestRedis(t)
	defer cleanup()

	store := NewStore(client)
	ctx := context.Background()

	messageID := "test-message-id"

	// Mark as processed first
	err := store.MarkProcessed(ctx, messageID)
	require.NoError(t, err)

	// Check if processed
	processed, err := store.IsProcessed(ctx, messageID)
	require.NoError(t, err)
	assert.True(t, processed)
}

func TestStore_MarkProcessed(t *testing.T) {
	client, cleanup := setupTestRedis(t)
	defer cleanup()

	store := NewStore(client)
	ctx := context.Background()

	messageID := "test-message-id"

	err := store.MarkProcessed(ctx, messageID)
	require.NoError(t, err)

	// Verify it's marked
	processed, err := store.IsProcessed(ctx, messageID)
	require.NoError(t, err)
	assert.True(t, processed)
}

func TestStore_MarkProcessed_TTL(t *testing.T) {
	client, cleanup := setupTestRedis(t)
	defer cleanup()

	store := NewStore(client)
	ctx := context.Background()

	messageID := "test-message-id"

	err := store.MarkProcessed(ctx, messageID)
	require.NoError(t, err)

	// Verify key exists
	key := store.Key(messageID)
	ttl, err := client.TTL(ctx, key).Result()
	require.NoError(t, err)

	// TTL should be approximately 7 days (within 1 minute tolerance)
	expectedTTL := 7 * 24 * time.Hour
	assert.InDelta(t, expectedTTL.Seconds(), ttl.Seconds(), 60)
}

func TestStore_IsProcessed_RedisError(t *testing.T) {
	// Create a client that will fail
	client := redis.NewClient(&redis.Options{
		Addr: "invalid-address:6379",
	})
	defer client.Close()

	store := NewStore(client)
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	_, err := store.IsProcessed(ctx, "test-id")
	assert.Error(t, err)
}

func TestStore_MarkProcessed_RedisError(t *testing.T) {
	// Create a client that will fail
	client := redis.NewClient(&redis.Options{
		Addr: "invalid-address:6379",
	})
	defer client.Close()

	store := NewStore(client)
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	err := store.MarkProcessed(ctx, "test-id")
	assert.Error(t, err)
}
