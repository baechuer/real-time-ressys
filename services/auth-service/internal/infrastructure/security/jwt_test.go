package security

import (
	"strings"
	"testing"
	"time"

	"github.com/baechuer/real-time-ressys/services/auth-service/internal/domain"
	"github.com/golang-jwt/jwt/v5"
)

func TestJWTSigner_SignAndVerify_Success(t *testing.T) {
	t.Parallel()

	s := NewJWTSigner("secret", "auth-service")
	tok, err := s.SignAccessToken("u1", "user", 2*time.Minute)
	if err != nil {
		t.Fatalf("sign err: %v", err)
	}
	if tok == "" {
		t.Fatalf("expected non-empty token")
	}

	claims, err := s.VerifyAccessToken(tok)
	if err != nil {
		t.Fatalf("verify err: %v", err)
	}
	if claims.UserID != "u1" || claims.Role != "user" {
		t.Fatalf("unexpected claims: %+v", claims)
	}
	if claims.Exp.IsZero() {
		t.Fatalf("expected exp to be set")
	}
}

func TestJWTSigner_Verify_Expired_ReturnsTokenExpired(t *testing.T) {
	t.Parallel()

	s := NewJWTSigner("secret", "auth-service")
	tok, err := s.SignAccessToken("u1", "user", -1*time.Second) // already expired
	if err != nil {
		t.Fatalf("sign err: %v", err)
	}

	_, verr := s.VerifyAccessToken(tok)
	if verr == nil {
		t.Fatalf("expected error, got nil")
	}
	if !domain.Is(verr, "token_expired") {
		t.Fatalf("expected token_expired, got %v", verr)
	}
}

func TestJWTSigner_Verify_WrongSecret_ReturnsTokenInvalid(t *testing.T) {
	t.Parallel()

	s1 := NewJWTSigner("secret1", "auth-service")
	s2 := NewJWTSigner("secret2", "auth-service")

	tok, err := s1.SignAccessToken("u1", "user", time.Minute)
	if err != nil {
		t.Fatalf("sign err: %v", err)
	}

	_, verr := s2.VerifyAccessToken(tok)
	if verr == nil {
		t.Fatalf("expected error, got nil")
	}
	if !domain.Is(verr, "token_invalid") {
		t.Fatalf("expected token_invalid, got %v", verr)
	}
}

func TestJWTSigner_Verify_AlgConfusion_Rejected(t *testing.T) {
	t.Parallel()

	// Create a token with "none" alg (unsigned). Verify should reject.
	claims := jwt.MapClaims{
		"uid":  "u1",
		"role": "user",
		"iss":  "auth-service",
		"sub":  "u1",
		"exp":  time.Now().Add(time.Minute).Unix(),
		"iat":  time.Now().Unix(),
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodNone, claims)

	unsigned, err := tok.SignedString(jwt.UnsafeAllowNoneSignatureType)
	if err != nil {
		t.Fatalf("unexpected signing err: %v", err)
	}

	s := NewJWTSigner("secret", "auth-service")
	_, verr := s.VerifyAccessToken(unsigned)
	if verr == nil {
		t.Fatalf("expected error, got nil")
	}
	if !domain.Is(verr, "token_invalid") {
		t.Fatalf("expected token_invalid, got %v", verr)
	}
}

func TestJWTSigner_Verify_Garbage_ReturnsTokenInvalid(t *testing.T) {
	t.Parallel()

	s := NewJWTSigner("secret", "auth-service")

	_, err := s.VerifyAccessToken("not.a.jwt")
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if !domain.Is(err, "token_invalid") {
		t.Fatalf("expected token_invalid, got %v", err)
	}
}

func TestJWTSigner_Sign_WrapsTokenSignFailed_WhenSecretInvalid(t *testing.T) {
	t.Parallel()

	// HS256 accepts any byte slice as secret, so it's hard to force SignedString to fail.
	// But we can at least ensure token contains 3 segments (header.payload.sig).
	s := NewJWTSigner("", "auth-service")
	tok, err := s.SignAccessToken("u1", "user", time.Minute)
	if err != nil {
		// if it errors in future library versions, ensure it maps to token_sign_failed
		if !domain.Is(err, "token_sign_failed") {
			t.Fatalf("expected token_sign_failed, got %v", err)
		}
		return
	}
	if strings.Count(tok, ".") != 2 {
		t.Fatalf("expected jwt with 3 segments, got %q", tok)
	}
}
