package redis

import (
	"context"
	"testing"
	"time"
)

func TestFixedWindowLimiter_RedisNil_Allows(t *testing.T) {
	l := NewFixedWindowLimiter(nil)

	d, err := l.AllowFixedWindow(context.Background(), "k", 10, time.Minute)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !d.Allowed {
		t.Fatalf("expected allowed when redis disabled")
	}
	if d.Remaining != 10 {
		t.Fatalf("unexpected remaining: %d", d.Remaining)
	}
}

func TestFixedWindowLimiter_LimitZero_Allows(t *testing.T) {
	l := NewFixedWindowLimiter(nil)

	d, _ := l.AllowFixedWindow(context.Background(), "k", 0, time.Minute)
	if !d.Allowed {
		t.Fatalf("limit=0 should allow")
	}
}
