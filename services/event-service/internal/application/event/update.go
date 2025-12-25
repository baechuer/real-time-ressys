package event

import (
	"context"
	"time"

	"github.com/baechuer/real-time-ressys/services/event-service/internal/domain"
)

type UpdateCmd struct {
	ActorID   string
	ActorRole string
	EventID   string

	Title       *string
	Description *string
	City        *string
	Category    *string
	StartTime   *time.Time
	EndTime     *time.Time
	Capacity    *int
}

func (s *Service) Update(ctx context.Context, cmd UpdateCmd) (*domain.Event, error) {
	ev, err := s.repo.GetByID(ctx, cmd.EventID)
	if err != nil {
		return nil, err
	}

	if !canManage(cmd.ActorID, cmd.ActorRole, ev.OwnerID) {
		return nil, domain.ErrForbidden("not allowed")
	}
	if ev.Status == domain.StatusCanceled {
		return nil, domain.ErrInvalidState("canceled event cannot be updated")
	}

	// if cmd.Title != nil { ev.Title = *cmd.Title } ...

	if !ev.EndTime.After(ev.StartTime) {
		return nil, domain.ErrValidationMeta("invalid time range", map[string]string{
			"end_time": "must be after start_time",
		})
	}

	now := s.clock.Now().UTC()
	ev.UpdatedAt = now

	if err := s.repo.Update(ctx, ev); err != nil {
		return nil, err
	}
	return ev, nil
}
