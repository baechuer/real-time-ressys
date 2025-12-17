package idempotency

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// Store handles idempotency checking using Redis
type Store struct {
	client *redis.Client
	ttl    time.Duration
}

// NewStore creates a new idempotency store
func NewStore(client *redis.Client) *Store {
	return &Store{
		client: client,
		ttl:    7 * 24 * time.Hour, // 7 days
	}
}

// Key generates a Redis key for a message ID
func (s *Store) Key(messageID string) string {
	return fmt.Sprintf("email:processed:%s", messageID)
}

// IsProcessed checks if a message has already been processed
func (s *Store) IsProcessed(ctx context.Context, messageID string) (bool, error) {
	key := s.Key(messageID)
	exists, err := s.client.Exists(ctx, key).Result()
	if err != nil {
		return false, fmt.Errorf("failed to check idempotency: %w", err)
	}
	return exists > 0, nil
}

// MarkProcessed marks a message as processed
func (s *Store) MarkProcessed(ctx context.Context, messageID string) error {
	key := s.Key(messageID)
	err := s.client.Set(ctx, key, time.Now().Unix(), s.ttl).Err()
	if err != nil {
		return fmt.Errorf("failed to mark message as processed: %w", err)
	}
	return nil
}

// CheckAndMarkAtomic atomically checks if message is processed and marks it if not
// Returns: (isDuplicate, error)
// This is the atomic version that prevents race conditions
func (s *Store) CheckAndMarkAtomic(ctx context.Context, messageID string) (bool, error) {
	key := s.Key(messageID)
	
	// Use SETNX (SET if Not eXists) for atomic check-and-set
	// Returns true if key was set (new message), false if key already existed (duplicate)
	set, err := s.client.SetNX(ctx, key, time.Now().Unix(), s.ttl).Result()
	if err != nil {
		return false, fmt.Errorf("failed to atomically check and mark idempotency: %w", err)
	}
	
	// If set is false, the key already existed (duplicate)
	// If set is true, we just created the key (new message)
	return !set, nil
}

