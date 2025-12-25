package event

import (
	"context"

	"github.com/baechuer/real-time-ressys/services/event-service/internal/domain"
)

func (s *Service) GetPublic(ctx context.Context, id string) (*domain.Event, error) {
	e, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	// Public: only published and not canceled; ended is still viewable (MVP)
	if e.Status != domain.StatusPublished {
		return nil, domain.ErrNotFound("event not found")
	}
	if e.Status == domain.StatusCanceled {
		return nil, domain.ErrNotFound("event not found")
	}
	return e, nil
}

func (s *Service) GetForOwner(ctx context.Context, id, actorID, actorRole string) (*domain.Event, error) {
	e, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if !canEdit(e.OwnerID, actorID, actorRole) {
		return nil, domain.ErrForbidden("not allowed")
	}
	return e, nil
}
