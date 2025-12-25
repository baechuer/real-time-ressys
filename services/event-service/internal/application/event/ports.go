package event

import (
	"context"
	"time"

	"github.com/baechuer/real-time-ressys/services/event-service/internal/domain"
)

type Clock interface {
	Now() time.Time
}

type EventRepo interface {
	Create(ctx context.Context, e *domain.Event) error
	GetByID(ctx context.Context, id string) (*domain.Event, error)
	Update(ctx context.Context, e *domain.Event) error

	ListPublic(ctx context.Context, f ListFilter) ([]*domain.Event, int, error)
	ListByOwner(ctx context.Context, ownerID string, page, pageSize int) ([]*domain.Event, int, error)
}
