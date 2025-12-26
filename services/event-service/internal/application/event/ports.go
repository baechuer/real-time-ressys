package event

import (
	"context"

	"github.com/baechuer/real-time-ressys/services/event-service/internal/domain"
)

type EventRepo interface {
	Create(ctx context.Context, e *domain.Event) error
	GetByID(ctx context.Context, id string) (*domain.Event, error)
	Update(ctx context.Context, e *domain.Event) error

	ListByOwner(ctx context.Context, ownerID string, page, pageSize int) ([]*domain.Event, int, error)
	ListPublic(ctx context.Context, f ListFilter) ([]*domain.Event, int, error)
}

type EventPublisher interface {
	PublishEvent(ctx context.Context, routingKey string, payload any) error
}
