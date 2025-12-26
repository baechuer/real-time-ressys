package router

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/baechuer/real-time-ressys/services/event-service/internal/application/event"
	"github.com/baechuer/real-time-ressys/services/event-service/internal/domain"
	"github.com/baechuer/real-time-ressys/services/event-service/internal/transport/http/handlers"
	authmw "github.com/baechuer/real-time-ressys/services/event-service/internal/transport/http/middleware"
	"github.com/stretchr/testify/assert"
)

// stubClock prevents nil pointer panic in handlers
type stubClock struct{}

func (stubClock) Now() time.Time { return time.Date(2025, 12, 26, 12, 0, 0, 0, time.UTC) }

// stubRepo prevents nil pointer panic in service
// stubRepo prevents nil pointer panic in service
type stubRepo struct{}

func (s *stubRepo) Create(ctx context.Context, e *domain.Event) error { return nil }
func (s *stubRepo) GetByID(ctx context.Context, id string) (*domain.Event, error) {
	return &domain.Event{}, nil
}
func (s *stubRepo) Update(ctx context.Context, e *domain.Event) error { return nil }
func (s *stubRepo) ListPublic(ctx context.Context, f event.ListFilter) ([]*domain.Event, int, error) {
	return []*domain.Event{}, 0, nil
}

// Added: Missing method to satisfy event.EventRepo interface
func (s *stubRepo) ListPublicAfter(
	ctx context.Context,
	f event.ListFilter,
	afterRank float64,
	afterStart time.Time,
	afterID string,
) ([]*domain.Event, int, string, error) {
	return []*domain.Event{}, 0, "", nil
}

func (s *stubRepo) ListByOwner(ctx context.Context, o string, p, ps int) ([]*domain.Event, int, error) {
	return []*domain.Event{}, 0, nil
}

func (s *stubRepo) ListPublicTimeKeyset(
	ctx context.Context,
	f event.ListFilter,
	hasCursor bool,
	afterStart time.Time,
	afterID string,
) ([]*domain.Event, error) {
	return []*domain.Event{}, nil
}

func (s *stubRepo) ListPublicRelevanceKeyset(
	ctx context.Context,
	f event.ListFilter,
	hasCursor bool,
	afterRank float64,
	afterStart time.Time,
	afterID string,
) ([]*domain.Event, []float64, error) {
	return []*domain.Event{}, []float64{}, nil
}

// Mock Publisher for Router Test
type stubPub struct{}

func (s stubPub) PublishEvent(ctx context.Context, routingKey string, payload any) error { return nil }
func TestRouter_Routing(t *testing.T) {
	auth := authmw.NewAuth("secret", "issuer")

	clock := stubClock{}
	// Updated: Use stubPub instead of nil to satisfy EventPublisher interface
	svc := event.New(&stubRepo{}, clock, stubPub{})
	h := handlers.NewEventsHandler(svc, clock)
	z := handlers.NewHealthHandler()

	r := New(h, auth, z)

	t.Run("public_route_returns_200", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/event/v1/events", nil)
		rr := httptest.NewRecorder()

		r.ServeHTTP(rr, req)

		// This should no longer be 500 (panic)
		assert.Equal(t, http.StatusOK, rr.Code)
	})

	t.Run("protected_route_returns_401_without_token", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/event/v1/events", nil)
		rr := httptest.NewRecorder()

		r.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusUnauthorized, rr.Code)
	})
}
