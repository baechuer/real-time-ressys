package idempotency

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"

	amqp "github.com/rabbitmq/amqp091-go"
)

// Checker provides idempotency checking functionality
type Checker struct {
	store *Store
}

// NewChecker creates a new idempotency checker
func NewChecker(store *Store) *Checker {
	return &Checker{
		store: store,
	}
}

// GenerateMessageID generates a unique message ID from RabbitMQ delivery
func (c *Checker) GenerateMessageID(delivery amqp.Delivery) string {
	// Use delivery tag + message body hash for uniqueness
	hash := sha256.Sum256(delivery.Body)
	hashStr := hex.EncodeToString(hash[:])
	return fmt.Sprintf("%d:%s", delivery.DeliveryTag, hashStr)
}

// CheckAndMark checks if message is processed and marks it if not
// Returns: (isDuplicate, error)
// Uses atomic operation to prevent race conditions
func (c *Checker) CheckAndMark(ctx context.Context, messageID string) (bool, error) {
	// Use atomic check-and-set to prevent race conditions
	return c.store.CheckAndMarkAtomic(ctx, messageID)
}

