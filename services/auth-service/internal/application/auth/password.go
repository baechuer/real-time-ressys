package auth

import (
	"context"

	"github.com/baechuer/real-time-ressys/services/auth-service/internal/domain"
)

// PasswordChange changes password for an authenticated user.
func (s *Service) PasswordChange(ctx context.Context, userID, oldPassword, newPassword string) error {
	if userID == "" {
		return domain.ErrTokenMissing()
	}
	if oldPassword == "" || newPassword == "" {
		return domain.ErrInvalidField("password", "empty")
	}
	if len(newPassword) < 12 {
		return domain.ErrWeakPassword("min length 12")
	}

	u, err := s.users.GetByID(ctx, userID)
	if err != nil {
		return err
	}

	if err := s.hasher.Compare(u.PasswordHash, oldPassword); err != nil {
		return domain.ErrInvalidCredentials()
	}

	newHash, err := s.hasher.Hash(newPassword)
	if err != nil {
		return domain.ErrHashFailed(err)
	}

	if err := s.users.UpdatePasswordHash(ctx, userID, newHash); err != nil {
		return err
	}

	// Recommended: revoke all sessions after password change
	_ = s.sessions.RevokeAll(ctx, userID)
	return nil
}

// PasswordResetRequest generates a one-time token and publishes an email event.
// IMPORTANT: should be non-enumerating - caller should always return 200.
func (s *Service) PasswordResetRequest(ctx context.Context, email string) error {
	u, err := s.users.GetByEmail(ctx, email)
	if err != nil {
		return nil
	}

	token, err := newOpaqueToken(32)
	if err != nil {
		return domain.ErrRandomFailed(err)
	}

	if err := s.ott.Save(ctx, TokenPasswordReset, token, u.ID, s.passwordResetTTL); err != nil {
		return err
	}

	url := s.passwordResetBaseURL + token
	return s.pub.PublishPasswordReset(ctx, PasswordResetEvent{
		UserID: u.ID,
		Email:  u.Email,
		URL:    url,
	})
}

// PasswordResetValidate optionally checks whether a reset token is valid.
func (s *Service) PasswordResetValidate(ctx context.Context, token string) error {
	if token == "" {
		return domain.ErrMissingField("token")
	}
	_, err := s.ott.Peek(ctx, TokenPasswordReset, token)
	return err
}

// PasswordResetConfirm consumes the token and sets a new password.
func (s *Service) PasswordResetConfirm(ctx context.Context, token, newPassword string) error {
	if token == "" {
		return domain.ErrMissingField("token")
	}
	if newPassword == "" {
		return domain.ErrMissingField("new_password")
	}
	if len(newPassword) < 12 {
		return domain.ErrWeakPassword("min length 12")
	}

	userID, err := s.ott.Consume(ctx, TokenPasswordReset, token)
	if err != nil {
		return err
	}

	hash, err := s.hasher.Hash(newPassword)
	if err != nil {
		return domain.ErrHashFailed(err)
	}

	if err := s.users.UpdatePasswordHash(ctx, userID, hash); err != nil {
		return err
	}

	// Recommended: revoke all sessions after reset
	_ = s.sessions.RevokeAll(ctx, userID)
	return nil
}
