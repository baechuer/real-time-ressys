package auth

import (
	"context"
	"strings"

	"github.com/baechuer/real-time-ressys/services/auth-service/internal/domain"
)

// VerifyEmailRequest generates a one-time token and publishes an email event.
// IMPORTANT: non-enumerating - if user not found, return nil.
func (s *Service) VerifyEmailRequest(ctx context.Context, email string) error {
	email = strings.TrimSpace(strings.ToLower(email))
	if email == "" {
		return domain.ErrMissingField("email")
	}

	u, err := s.users.GetByEmail(ctx, email)
	if err != nil {
		// 防枚举：用户不存在也返回成功
		return nil
	}

	token, err := newOpaqueToken(32)
	if err != nil {
		return domain.ErrRandomFailed(err)
	}

	if err := s.ott.Save(ctx, TokenVerifyEmail, token, u.ID, s.verifyEmailTTL); err != nil {
		return err
	}

	url := s.verifyEmailBaseURL + token
	return s.pub.PublishVerifyEmail(ctx, VerifyEmailEvent{
		UserID: u.ID,
		Email:  u.Email,
		URL:    url,
	})
}

// VerifyEmailConfirm consumes token and marks user as verified.
func (s *Service) VerifyEmailConfirm(ctx context.Context, token string) error {
	token = strings.TrimSpace(token)
	if token == "" {
		return domain.ErrMissingField("token")
	}

	userID, err := s.ott.Consume(ctx, TokenVerifyEmail, token)
	if err != nil {
		return err
	}

	return s.users.SetEmailVerified(ctx, userID)
}
