package security

import (
	"testing"
	"time"

	"github.com/baechuer/real-time-ressys/services/auth-service/internal/domain"
)

func TestStubSigner_SignAndVerify_Success(t *testing.T) {
	t.Parallel()

	s := NewStubSigner()

	tok, err := s.SignAccessToken("u1", "user", time.Minute)
	if err != nil {
		t.Fatalf("sign err: %v", err)
	}

	claims, err := s.VerifyAccessToken(tok)
	if err != nil {
		t.Fatalf("verify err: %v", err)
	}
	if claims.UserID != "u1" || claims.Role != "user" {
		t.Fatalf("unexpected claims: %+v", claims)
	}
	if claims.Exp.IsZero() {
		t.Fatalf("expected exp set")
	}
}

func TestStubSigner_Verify_InvalidFormat_TokenInvalid(t *testing.T) {
	t.Parallel()

	s := NewStubSigner()

	_, err := s.VerifyAccessToken("stub.u1.user") // missing exp part
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if !domain.Is(err, "token_invalid") {
		t.Fatalf("expected token_invalid, got %v", err)
	}
}

func TestStubSigner_Verify_Expired_TokenExpired(t *testing.T) {
	t.Parallel()

	s := NewStubSigner()

	tok, err := s.SignAccessToken("u1", "user", -1*time.Second)
	if err != nil {
		t.Fatalf("sign err: %v", err)
	}

	_, err = s.VerifyAccessToken(tok)
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if !domain.Is(err, "token_expired") {
		t.Fatalf("expected token_expired, got %v", err)
	}
}
