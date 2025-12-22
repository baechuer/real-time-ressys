package redis

import (
	"context"
	"testing"
	"time"
)

func TestFixedWindowLimiter_LimitNegative_Allows(t *testing.T) {
	t.Parallel()

	l := NewFixedWindowLimiter(nil)

	d, err := l.AllowFixedWindow(context.Background(), "k", -5, time.Minute)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if !d.Allowed {
		t.Fatalf("expected allowed=true, got false")
	}
	// your implementation returns Remaining=limit; for negative limit that stays negative.
	// we only assert it doesn't block.
}

func TestFixedWindowLimiter_WindowZero_DefaultsAndAllows_WhenRedisNil(t *testing.T) {
	t.Parallel()

	l := NewFixedWindowLimiter(nil)

	d, err := l.AllowFixedWindow(context.Background(), "k", 5, 0)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if !d.Allowed {
		t.Fatalf("expected allowed=true, got false")
	}
	if d.Limit != 5 {
		t.Fatalf("expected Limit=5, got %d", d.Limit)
	}
	// Remaining should be limit in redis-nil fail-open path
	if d.Remaining != 5 {
		t.Fatalf("expected Remaining=5, got %d", d.Remaining)
	}
}
