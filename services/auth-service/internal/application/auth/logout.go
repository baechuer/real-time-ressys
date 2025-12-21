package auth

import "context"

// Logout revokes the current refresh token (single session logout).
// If the refresh token is missing/empty, it becomes a no-op.
func (s *Service) Logout(ctx context.Context, refreshToken string) error {
	if refreshToken == "" {
		return nil
	}
	return s.sessions.RevokeRefreshToken(ctx, refreshToken)
}
