package event

import (
	"context"

	"github.com/baechuer/real-time-ressys/services/event-service/internal/domain"
)

func (s *Service) Cancel(ctx context.Context, eventID, actorID, actorRole string) (*domain.Event, error) {
	ev, err := s.repo.GetByID(ctx, eventID)
	if err != nil {
		return nil, err
	}

	if !canManage(actorID, actorRole, ev.OwnerID) {
		return nil, domain.ErrForbidden("not allowed")
	}

	if ev.Status == domain.StatusCanceled {
		return nil, domain.ErrInvalidState("event already canceled")
	}

	now := s.clock.Now().UTC()
	ev.Status = domain.StatusCanceled
	ev.CanceledAt = &now
	ev.UpdatedAt = now

	if err := s.repo.Update(ctx, ev); err != nil {
		return nil, err
	}
	return ev, nil
}
