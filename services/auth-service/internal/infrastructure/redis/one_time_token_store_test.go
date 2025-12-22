package redis

import (
	"context"
	"testing"
	"time"

	"github.com/baechuer/real-time-ressys/services/auth-service/internal/application/auth"
	"github.com/baechuer/real-time-ressys/services/auth-service/internal/domain"
)

func TestOTT_Save_Validation(t *testing.T) {
	t.Parallel()

	s := NewOneTimeTokenStore(nil)
	ctx := context.Background()

	// token missing
	if err := s.Save(ctx, auth.OneTimeTokenKind("verify_email"), "", "u1", time.Minute); err == nil || !domain.Is(err, "missing_field") || err.(*domain.Error).Meta["field"] != "token" {
		t.Fatalf("expected missing_field(token), got %v", err)
	}

	// user_id missing
	if err := s.Save(ctx, auth.OneTimeTokenKind("verify_email"), "tok", "", time.Minute); err == nil || !domain.Is(err, "missing_field") || err.(*domain.Error).Meta["field"] != "user_id" {
		t.Fatalf("expected missing_field(user_id), got %v", err)
	}

	// ttl missing/invalid
	if err := s.Save(ctx, auth.OneTimeTokenKind("verify_email"), "tok", "u1", 0); err == nil || !domain.Is(err, "missing_field") || err.(*domain.Error).Meta["field"] != "ttl" {
		t.Fatalf("expected missing_field(ttl), got %v", err)
	}
}

func TestOTT_Save_RedisNotConfigured(t *testing.T) {
	t.Parallel()

	s := NewOneTimeTokenStore(nil)
	ctx := context.Background()

	// inputs valid, but redis nil => infra error string
	err := s.Save(ctx, auth.OneTimeTokenKind("verify_email"), "tok", "u1", time.Minute)
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if err.Error() != "redis one-time-token store not configured" {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestOTT_Consume_EmptyToken(t *testing.T) {
	t.Parallel()

	s := NewOneTimeTokenStore(nil)
	_, err := s.Consume(context.Background(), auth.OneTimeTokenKind("verify_email"), "")
	if err == nil || !domain.Is(err, "missing_field") || err.(*domain.Error).Meta["field"] != "token" {
		t.Fatalf("expected missing_field(token), got %v", err)
	}
}

func TestOTT_Peek_EmptyToken(t *testing.T) {
	t.Parallel()

	s := NewOneTimeTokenStore(nil)
	_, err := s.Peek(context.Background(), auth.OneTimeTokenKind("verify_email"), "")
	if err == nil || !domain.Is(err, "missing_field") || err.(*domain.Error).Meta["field"] != "token" {
		t.Fatalf("expected missing_field(token), got %v", err)
	}
}
