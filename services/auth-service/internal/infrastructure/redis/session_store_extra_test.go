package redis

import (
	"context"
	"testing"
)

func TestSessionStore_RevokeRefreshToken_Empty_IsIdempotent(t *testing.T) {
	t.Parallel()

	s := NewRedisSessionStore(nil)

	// should be idempotent and return nil even if redis not configured
	if err := s.RevokeRefreshToken(context.Background(), ""); err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
	if err := s.RevokeRefreshToken(context.Background(), "   "); err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
}

func TestSessionStore_RevokeAll_EmptyUserID_MissingField(t *testing.T) {
	t.Parallel()

	s := NewRedisSessionStore(nil)

	err := s.RevokeAll(context.Background(), "")
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if !isMissingField(err, "user_id") {
		t.Fatalf("expected missing_field(user_id), got %v", err)
	}

	err = s.RevokeAll(context.Background(), "   ")
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if !isMissingField(err, "user_id") {
		t.Fatalf("expected missing_field(user_id), got %v", err)
	}
}
