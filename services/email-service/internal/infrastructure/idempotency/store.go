package idempotency

import (
	"context"
	"time"
)

type Store interface {
	// SeenOrMark returns (alreadySent, err).
	// It should atomically mark the key if it does not exist.
	SeenOrMark(ctx context.Context, key string, ttl time.Duration) (bool, error)
}
