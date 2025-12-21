package memory

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"log"

	"github.com/baechuer/real-time-ressys/services/auth-service/internal/domain"
)

// Hasher is the minimal surface we need for seeding.
type Hasher interface {
	Hash(password string) (string, error)
}

// SeedUsers creates initial users for local development (in-memory only).
// Safe to call multiple times (duplicates ignored).
func SeedUsers(ctx context.Context, users *UserRepo, hasher Hasher) {
	type seedUser struct {
		Email string
		Role  string
		Pass  string
	}

	seeds := []seedUser{
		{Email: "admin@example.com", Role: "admin", Pass: "AdminPassword123!"},
		{Email: "moderator@example.com", Role: "moderator", Pass: "ModeratorPassword123!"},
		{Email: "user@example.com", Role: "user", Pass: "UserPassword123!"},
	}

	for _, s := range seeds {
		hash, err := hasher.Hash(s.Pass)
		if err != nil {
			log.Printf("[seed] hash failed (%s): %v", s.Email, err)
			continue
		}

		u := domain.User{
			ID:            newID(),
			Email:         s.Email,
			PasswordHash:  hash,
			Role:          s.Role,
			EmailVerified: true,
			Locked:        false,
		}

		_, err = users.Create(ctx, u)
		if err != nil {
			// ignore duplicates / restart
			continue
		}
	}

	log.Println("[seed] in-memory users seeded")
}

func newID() string {
	// 16 bytes => 32 hex chars; good enough for in-memory dev IDs
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
