package redis

import (
	"context"
	"testing"
	"time"
)

func TestClient_Ping_FailsFast(t *testing.T) {
	c := New("127.0.0.1:1", "", 0) // guaranteed unreachable

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	err := c.Ping(ctx)
	if err == nil {
		t.Fatalf("expected ping error")
	}
}

func TestClient_Close_Idempotent(t *testing.T) {
	c := New("127.0.0.1:1", "", 0)

	if err := c.Close(); err != nil {
		t.Fatalf("close failed: %v", err)
	}
	// call twice should not panic
	_ = c.Close()
}
