package redis

import (
	"context"
	"testing"
	"time"

	"github.com/baechuer/real-time-ressys/services/auth-service/internal/domain"
)

func TestSessionStore_CreateRefreshToken_RedisNil(t *testing.T) {
	s := NewRedisSessionStore(nil)

	_, err := s.CreateRefreshToken(context.Background(), "u1", time.Hour)
	if err == nil {
		t.Fatalf("expected error when redis not configured")
	}
}

func TestSessionStore_CreateRefreshToken_MissingUser(t *testing.T) {
	s := NewRedisSessionStore(nil)

	_, err := s.CreateRefreshToken(context.Background(), "", time.Hour)
	if !domain.Is(err, "missing_field") {
		t.Fatalf("expected missing_field")
	}
}

func TestSessionStore_Rotate_EmptyToken(t *testing.T) {
	s := NewRedisSessionStore(nil)

	_, err := s.RotateRefreshToken(context.Background(), "", time.Hour)
	if !domain.Is(err, "refresh_token_invalid") {
		t.Fatalf("expected refresh_token_invalid")
	}
}

func TestParseUIDVer(t *testing.T) {
	uid, ver, err := parseUIDVer("abc:3")
	if err != nil {
		t.Fatalf("unexpected error")
	}
	if uid != "abc" || ver != 3 {
		t.Fatalf("bad parse")
	}
}

func TestParseUIDVer_Invalid(t *testing.T) {
	cases := []string{
		"",
		"abc",
		"abc:",
		":1",
		"abc:x",
	}

	for _, c := range cases {
		if _, _, err := parseUIDVer(c); err == nil {
			t.Fatalf("expected error for %q", c)
		}
	}
}
