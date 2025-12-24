package idempotency

import (
	"context"
	"time"
)

type NoopStore struct{}

func NewNoopStore() *NoopStore { return &NoopStore{} }

func (s *NoopStore) SeenOrMark(ctx context.Context, key string, ttl time.Duration) (bool, error) {
	return false, nil
}
