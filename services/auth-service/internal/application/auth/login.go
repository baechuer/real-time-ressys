package auth

import (
	"context"
	"strings"

	"github.com/baechuer/real-time-ressys/services/auth-service/internal/domain"
)

// Login authenticates a user and issues tokens.
// IMPORTANT: must not leak whether the email exists (avoid user enumeration).
func (s *Service) Login(ctx context.Context, email, password string) (LoginResult, error) {
	email = strings.TrimSpace(email)

	if email == "" || password == "" {
		return LoginResult{}, domain.ErrInvalidCredentials()
	}

	u, err := s.users.GetByEmail(ctx, email)
	if err != nil {
		// Hide not-found behind invalid credentials
		return LoginResult{}, domain.ErrInvalidCredentials()
	}

	// Optional policy checks (uncomment if you want them enforced at login)
	// if u.Locked {
	// 	return LoginResult{}, domain.ErrAccountLocked()
	// }
	// if !u.EmailVerified {
	// 	return LoginResult{}, domain.ErrEmailNotVerified()
	// }

	// Compare password
	if err := s.hasher.Compare(u.PasswordHash, password); err != nil {
		return LoginResult{}, domain.ErrInvalidCredentials()
	}

	toks, err := s.issueTokens(ctx, u.ID, u.Role)
	if err != nil {
		return LoginResult{}, err
	}

	return LoginResult{User: u, Tokens: toks}, nil
}
