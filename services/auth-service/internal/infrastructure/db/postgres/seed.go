package postgres

import (
	"context"
	"log"

	"github.com/baechuer/real-time-ressys/services/auth-service/internal/domain"
	"github.com/google/uuid"
)

type SeederHasher interface {
	Hash(password string) (string, error)
}

type SeederRepo interface {
	Create(ctx context.Context, u domain.User) (domain.User, error)
}

func SeedUsers(ctx context.Context, repo SeederRepo, hasher SeederHasher) {
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
			ID:            uuid.NewString(),
			Email:         s.Email,
			PasswordHash:  hash,
			Role:          s.Role,
			EmailVerified: true,
			Locked:        false,
		}

		_, err = repo.Create(ctx, u)
		if err != nil {
			// ignore duplicates (restart safe)
			continue
		}
	}

	log.Println("[seed] postgres users seeded")
}
