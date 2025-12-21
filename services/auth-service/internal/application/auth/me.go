package auth

import (
	"context"

	"github.com/baechuer/real-time-ressys/services/auth-service/internal/domain"
)

func (s *Service) GetUserByID(ctx context.Context, userID string) (domain.User, error) {
	return s.users.GetByID(ctx, userID)
}
