package event

import (
	"context"
	"time"

	"github.com/baechuer/real-time-ressys/services/event-service/internal/domain"
)

type EventRepo interface {
	Create(ctx context.Context, e *domain.Event) error
	GetByID(ctx context.Context, id string) (*domain.Event, error)
	Update(ctx context.Context, e *domain.Event) error

	ListByOwner(ctx context.Context, ownerID string, page, pageSize int) ([]*domain.Event, int, error)

	// Keyset - time
	ListPublicTimeKeyset(
		ctx context.Context,
		f ListFilter,
		hasCursor bool,
		afterStart time.Time,
		afterID string,
	) ([]*domain.Event, error)

	// Keyset - relevance
	ListPublicRelevanceKeyset(
		ctx context.Context,
		f ListFilter,
		hasCursor bool,
		afterRank float64,
		afterStart time.Time,
		afterID string,
	) ([]*domain.Event, []float64, error)
}

type EventPublisher interface {
	PublishEvent(ctx context.Context, routingKey string, payload any) error
}
