package event

import (
	"context"

	"github.com/baechuer/real-time-ressys/services/event-service/internal/domain"
	zlog "github.com/rs/zerolog/log"
)

func (s *Service) Cancel(ctx context.Context, eventID, actorID, actorRole string) (*domain.Event, error) {
	ev, err := s.repo.GetByID(ctx, eventID)
	if err != nil {
		return nil, err
	}

	if !canManage(actorID, actorRole, ev.OwnerID) {
		return nil, domain.ErrForbidden("not allowed")
	}

	// Optional: if it's already canceled, treat as invalid state (current behavior)
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

	// --- MQ domain event (best-effort) ---
	if s.pub != nil {
		env := DomainEventEnvelope[EventCanceledPayload]{
			Version:    1,
			Producer:   "event-service",
			TraceID:    TraceIDFromContext(ctx),
			OccurredAt: now,
			Payload: EventCanceledPayload{
				EventID:   ev.ID,
				OwnerID:   ev.OwnerID,
				City:      ev.City,
				Category:  ev.Category,
				StartTime: ev.StartTime,
				EndTime:   ev.EndTime,
				Capacity:  ev.Capacity,
				Status:    string(ev.Status),
			},
		}

		if err := s.pub.PublishEvent(ctx, "event.canceled", env); err != nil {
			zlog.Error().
				Err(err).
				Str("rk", "event.canceled").
				Str("event_id", ev.ID).
				Msg("publish domain event failed")
		}
	}

	return ev, nil
}
