package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/baechuer/real-time-ressys/services/event-service/internal/application/event"
	"github.com/baechuer/real-time-ressys/services/event-service/internal/domain"
	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
)

// MockClock for stable testing
type mockClock struct{ t time.Time }

func (m mockClock) Now() time.Time { return m.t }

// TestCases
func TestEventsHandler_GetPublic(t *testing.T) {
	now := time.Now().UTC()
	repo := &mockRepo{}
	// Added a mock publisher (nil) as the 3rd argument for event.New
	svc := event.New(repo, mockClock{t: now}, nil)
	h := NewEventsHandler(svc, mockClock{t: now})

	t.Run("return_400_on_invalid_uuid", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/events/invalid-uuid", nil)

		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("event_id", "invalid-uuid")
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

		rr := httptest.NewRecorder()
		h.GetPublic(rr, req)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
		assert.Contains(t, rr.Body.String(), "validation_error")
	})
}

// Minimal mock repo for handler testing
type mockRepo struct{}

func (m *mockRepo) Create(ctx context.Context, e *domain.Event) error { return nil }
func (m *mockRepo) GetByID(ctx context.Context, id string) (*domain.Event, error) {
	if id == "not-found" {
		return nil, domain.ErrNotFound("not found")
	}
	return &domain.Event{ID: id, Status: domain.StatusPublished}, nil
}
func (m *mockRepo) Update(ctx context.Context, e *domain.Event) error { return nil }

// Keyset - time (Satisfying ports.go)
func (m *mockRepo) ListPublicTimeKeyset(
	ctx context.Context,
	f event.ListFilter,
	hasCursor bool,
	afterStart time.Time,
	afterID string,
) ([]*domain.Event, error) {
	return []*domain.Event{}, nil
}

// Keyset - relevance (Satisfying ports.go)
func (m *mockRepo) ListPublicRelevanceKeyset(
	ctx context.Context,
	f event.ListFilter,
	hasCursor bool,
	afterRank float64,
	afterStart time.Time,
	afterID string,
) ([]*domain.Event, []float64, error) {
	return []*domain.Event{}, []float64{}, nil
}

// Legacy ListPublic (If still in interface)
func (m *mockRepo) ListPublic(ctx context.Context, f event.ListFilter) ([]*domain.Event, int, error) {
	return nil, 0, nil
}

func (m *mockRepo) ListByOwner(ctx context.Context, ownerID string, page, pageSize int) ([]*domain.Event, int, error) {
	return nil, 0, nil
}
