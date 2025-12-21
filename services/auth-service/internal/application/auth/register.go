package auth

import (
	"context"

	"github.com/google/uuid"

	"github.com/baechuer/real-time-ressys/services/auth-service/internal/domain"
)

func (s *Service) Register(ctx context.Context, email, password string) (RegisterResult, error) {
	if email == "" || password == "" {
		return RegisterResult{}, domain.ErrInvalidField("email/password", "empty")
	}

	hash, err := s.hasher.Hash(password)
	if err != nil {
		return RegisterResult{}, domain.ErrHashFailed(err)
	}

	u := domain.User{
		ID:            uuid.NewString(), // ✅ ADD THIS
		Email:         email,
		PasswordHash:  hash,
		Role:          "user",
		EmailVerified: false,
		Locked:        false,
	}

	created, err := s.users.Create(ctx, u)
	if err != nil {
		return RegisterResult{}, err
	}

	toks, err := s.issueTokens(ctx, created.ID, created.Role)
	if err != nil {
		return RegisterResult{}, err
	}

	return RegisterResult{User: created, Tokens: toks}, nil
}
