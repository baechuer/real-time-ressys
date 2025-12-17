package idempotency

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"testing"

	"github.com/alicebob/miniredis/v2"
	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestChecker(t *testing.T) (*Checker, func()) {
	mr, err := miniredis.Run()
	require.NoError(t, err)

	client := redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
	})

	store := NewStore(client)
	checker := NewChecker(store)

	return checker, func() {
		client.Close()
		mr.Close()
	}
}

func TestChecker_GenerateMessageID(t *testing.T) {
	checker, cleanup := setupTestChecker(t)
	defer cleanup()

	delivery := amqp.Delivery{
		DeliveryTag: 123,
		Body:        []byte("test message body"),
	}

	messageID := checker.GenerateMessageID(delivery)

	// Should contain delivery tag
	assert.Contains(t, messageID, "123:")

	// Should contain hash of body
	hash := sha256.Sum256(delivery.Body)
	hashStr := hex.EncodeToString(hash[:])
	assert.Contains(t, messageID, hashStr)
}

func TestChecker_GenerateMessageID_UniqueForDifferentBodies(t *testing.T) {
	checker, cleanup := setupTestChecker(t)
	defer cleanup()

	delivery1 := amqp.Delivery{
		DeliveryTag: 123,
		Body:        []byte("message 1"),
	}

	delivery2 := amqp.Delivery{
		DeliveryTag: 123,
		Body:        []byte("message 2"),
	}

	id1 := checker.GenerateMessageID(delivery1)
	id2 := checker.GenerateMessageID(delivery2)

	assert.NotEqual(t, id1, id2)
}

func TestChecker_GenerateMessageID_UniqueForDifferentTags(t *testing.T) {
	checker, cleanup := setupTestChecker(t)
	defer cleanup()

	delivery1 := amqp.Delivery{
		DeliveryTag: 123,
		Body:        []byte("same body"),
	}

	delivery2 := amqp.Delivery{
		DeliveryTag: 456,
		Body:        []byte("same body"),
	}

	id1 := checker.GenerateMessageID(delivery1)
	id2 := checker.GenerateMessageID(delivery2)

	assert.NotEqual(t, id1, id2)
}

func TestChecker_CheckAndMark_NewMessage(t *testing.T) {
	checker, cleanup := setupTestChecker(t)
	defer cleanup()

	ctx := context.Background()
	messageID := "test-message-new"

	isDuplicate, err := checker.CheckAndMark(ctx, messageID)
	require.NoError(t, err)
	assert.False(t, isDuplicate, "new message should not be duplicate")
}

func TestChecker_CheckAndMark_DuplicateMessage(t *testing.T) {
	checker, cleanup := setupTestChecker(t)
	defer cleanup()

	ctx := context.Background()
	messageID := "test-message-duplicate"

	// First check - should mark as processed
	isDuplicate, err := checker.CheckAndMark(ctx, messageID)
	require.NoError(t, err)
	assert.False(t, isDuplicate, "first check should not be duplicate")

	// Second check - should detect duplicate
	isDuplicate, err = checker.CheckAndMark(ctx, messageID)
	require.NoError(t, err)
	assert.True(t, isDuplicate, "second check should be duplicate")
}

func TestChecker_CheckAndMark_ConcurrentCalls(t *testing.T) {
	checker, cleanup := setupTestChecker(t)
	defer cleanup()

	ctx := context.Background()
	messageID := "test-message-concurrent"

	// Simulate concurrent calls
	results := make(chan bool, 10)
	errors := make(chan error, 10)

	for i := 0; i < 10; i++ {
		go func() {
			isDuplicate, err := checker.CheckAndMark(ctx, messageID)
			results <- isDuplicate
			errors <- err
		}()
	}

	// Collect results
	duplicateCount := 0
	for i := 0; i < 10; i++ {
		err := <-errors
		require.NoError(t, err)
		if <-results {
			duplicateCount++
		}
	}

	// Only one should be non-duplicate, rest should be duplicates
	assert.Equal(t, 1, 10-duplicateCount, "only one call should mark as new")
	assert.Equal(t, 9, duplicateCount, "9 calls should detect duplicate")
}

func TestChecker_CheckAndMark_StoreError(t *testing.T) {
	// Create a store with invalid Redis connection
	mr, err := miniredis.Run()
	require.NoError(t, err)

	client := redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
	})
	store := NewStore(client)
	checker := NewChecker(store)

	// Close Redis to cause error
	mr.Close()
	client.Close()

	ctx := context.Background()
	messageID := "test-message-error"

	_, err = checker.CheckAndMark(ctx, messageID)
	assert.Error(t, err)
}

func createTestDelivery(tag uint64, body []byte) amqp.Delivery {
	return amqp.Delivery{
		DeliveryTag: tag,
		Body:        body,
		Headers:     amqp.Table{},
	}
}

func TestChecker_CheckAndMark_WithDelivery(t *testing.T) {
	checker, cleanup := setupTestChecker(t)
	defer cleanup()

	ctx := context.Background()
	delivery := createTestDelivery(1, []byte("test message"))

	messageID := checker.GenerateMessageID(delivery)

	// First check
	isDuplicate, err := checker.CheckAndMark(ctx, messageID)
	require.NoError(t, err)
	assert.False(t, isDuplicate)

	// Second check with same delivery
	messageID2 := checker.GenerateMessageID(delivery)
	assert.Equal(t, messageID, messageID2)

	isDuplicate, err = checker.CheckAndMark(ctx, messageID2)
	require.NoError(t, err)
	assert.True(t, isDuplicate)
}
