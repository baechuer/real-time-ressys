package event

import (
	"context"

	"github.com/baechuer/real-time-ressys/services/event-service/internal/domain"
)

func (s *Service) Publish(ctx context.Context, eventID, actorID, actorRole string) (*domain.Event, error) {
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
	if ev.Status != domain.StatusDraft {
		return nil, domain.ErrInvalidState("only draft can be published")
	}

	now := s.clock.Now().UTC()

	if !ev.EndTime.After(ev.StartTime) {
		return nil, domain.ErrValidationMeta("invalid time range", map[string]string{
			"end_time": "must be after start_time",
		})
	}
	if !ev.StartTime.After(now) {
		return nil, domain.ErrValidationMeta("invalid start_time", map[string]string{
			"start_time": "must be in the future when publishing",
		})
	}

	ev.Status = domain.StatusPublished
	ev.PublishedAt = &now
	ev.UpdatedAt = now

	if err := s.repo.Update(ctx, ev); err != nil {
		return nil, err
	}
	return ev, nil
}
