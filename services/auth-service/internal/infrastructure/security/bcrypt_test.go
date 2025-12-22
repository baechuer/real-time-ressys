package security

import (
	"testing"

	"github.com/baechuer/real-time-ressys/services/auth-service/internal/domain"
	"golang.org/x/crypto/bcrypt"
)

func TestNewBcryptHasher_DefaultCostWhenNonPositive(t *testing.T) {
	t.Parallel()

	h := NewBcryptHasher(0)
	if h == nil {
		t.Fatalf("expected hasher, got nil")
	}
	if h.cost != bcrypt.DefaultCost {
		t.Fatalf("expected cost=%d, got %d", bcrypt.DefaultCost, h.cost)
	}
}

func TestBcryptHasher_HashAndCompare_Success(t *testing.T) {
	t.Parallel()

	h := NewBcryptHasher(4) // lower cost for test speed
	pw := "P@ssw0rd123!"

	hash, err := h.Hash(pw)
	if err != nil {
		t.Fatalf("hash err: %v", err)
	}
	if hash == "" {
		t.Fatalf("expected non-empty hash")
	}
	if hash == pw {
		t.Fatalf("hash should not equal plaintext")
	}

	if err := h.Compare(hash, pw); err != nil {
		t.Fatalf("compare should succeed, got %v", err)
	}
}

func TestBcryptHasher_Compare_WrongPassword_Fails(t *testing.T) {
	t.Parallel()

	h := NewBcryptHasher(4)
	hash, err := h.Hash("correct-password")
	if err != nil {
		t.Fatalf("hash err: %v", err)
	}

	if err := h.Compare(hash, "wrong-password"); err == nil {
		t.Fatalf("expected error, got nil")
	}
}

func TestBcryptHasher_Hash_TooHighCost_ReturnsDomainHashFailed(t *testing.T) {
	t.Parallel()

	// bcrypt will error if cost is out of range; use an invalid cost > 31.
	h := NewBcryptHasher(100)

	_, err := h.Hash("pw")
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if !domain.Is(err, "hash_failed") {
		t.Fatalf("expected hash_failed, got %v", err)
	}
}
