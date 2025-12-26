package event

import (
	"context"

	"github.com/baechuer/real-time-ressys/services/event-service/internal/domain"
	zlog "github.com/rs/zerolog/log"
)

func (s *Service) Publish(ctx context.Context, eventID, actorID, actorRole string) (*domain.Event, error) {
	ev, err := s.repo.GetByID(ctx, eventID)
	if err != nil {
		return nil, err
	}

	if !canManage(actorID, actorRole, ev.OwnerID) {
		return nil, domain.ErrForbidden("not allowed")
	}

	switch ev.Status {
	case domain.StatusCanceled:
		return nil, domain.ErrInvalidState("event already canceled")
	case domain.StatusPublished:
		return nil, domain.ErrInvalidState("event already published")
	}

	now := s.clock.Now().UTC()

	// MVP rule: cannot publish if start_time is in the past
	if !ev.StartTime.IsZero() && ev.StartTime.Before(now) {
		return nil, domain.ErrValidationMeta("invalid start_time", map[string]string{
			"start_time": "cannot publish event in the past",
		})
	}

	ev.Status = domain.StatusPublished
	ev.PublishedAt = &now
	ev.UpdatedAt = now

	if err := s.repo.Update(ctx, ev); err != nil {
		return nil, err
	}

	// --- MQ domain event (best-effort) ---
	if s.pub != nil {
		env := DomainEventEnvelope[EventPublishedPayload]{
			Version:    1,
			Producer:   "event-service",
			TraceID:    TraceIDFromContext(ctx),
			OccurredAt: now,
			Payload: EventPublishedPayload{
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

		if err := s.pub.PublishEvent(ctx, "event.published", env); err != nil {
			zlog.Error().
				Err(err).
				Str("rk", "event.published").
				Str("event_id", ev.ID).
				Msg("publish domain event failed")
		}
	}

	return ev, nil
}
