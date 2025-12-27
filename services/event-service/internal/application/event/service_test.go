package event

import (
	"context"
	"testing"
	"time"

	"github.com/baechuer/real-time-ressys/services/event-service/internal/domain"
	"github.com/stretchr/testify/assert"
)

// --- Mocks & Helpers ---

type fakeClock struct{ t time.Time }

func (c fakeClock) Now() time.Time { return c.t }

// Mock Cache
type mockCache struct {
	store map[string]any
}

func newMockCache() *mockCache { return &mockCache{store: make(map[string]any)} }

func (m *mockCache) Get(ctx context.Context, key string, dest any) (bool, error) {
	// For testing simplistic scenarios
	// In real tests with reflection, dest needs to be set.
	// Here we just return miss to keep tests passing without complex reflection logic for now.
	// Or we can stub it if needed.
	return false, nil
}
func (m *mockCache) Set(ctx context.Context, key string, val any, ttl time.Duration) error {
	m.store[key] = val
	return nil
}
func (m *mockCache) Delete(ctx context.Context, keys ...string) error {
	for _, k := range keys {
		delete(m.store, k)
	}
	return nil
}

type memRepo struct {
	byID map[string]*domain.Event
}

func newMemRepo() *memRepo { return &memRepo{byID: map[string]*domain.Event{}} }

func (m *memRepo) Create(ctx context.Context, e *domain.Event) error {
	m.byID[e.ID] = e
	return nil
}

func (m *memRepo) GetByID(ctx context.Context, id string) (*domain.Event, error) {
	e, ok := m.byID[id]
	if !ok {
		return nil, domain.ErrNotFound("event not found")
	}
	return e, nil
}

func (m *memRepo) Update(ctx context.Context, e *domain.Event) error {
	m.byID[e.ID] = e
	return nil
}

// FIXED: Satisfies Keyset pagination ListPublicTimeKeyset
func (m *memRepo) ListPublicTimeKeyset(
	ctx context.Context,
	f ListFilter,
	hasCursor bool,
	afterStart time.Time,
	afterID string,
) ([]*domain.Event, error) {
	return []*domain.Event{}, nil
}

// FIXED: Satisfies Keyset pagination ListPublicRelevanceKeyset
func (m *memRepo) ListPublicRelevanceKeyset(
	ctx context.Context,
	f ListFilter,
	hasCursor bool,
	afterRank float64,
	afterStart time.Time,
	afterID string,
) ([]*domain.Event, []float64, error) {
	return []*domain.Event{}, []float64{}, nil
}

func (m *memRepo) ListByOwner(ctx context.Context, ownerID string, page, pageSize int) ([]*domain.Event, int, error) {
	return []*domain.Event{}, 0, nil
}

func mustTime(t *testing.T, s string) time.Time {
	tt, _ := time.Parse(time.RFC3339, s)
	return tt.UTC()
}

// --- Test Cases ---

func TestService_Cancel_Success(t *testing.T) {
	now := mustTime(t, "2025-12-25T10:00:00Z")
	repo := newMemRepo()
	// Update New() signature
	svc := New(repo, fakeClock{t: now}, NoopPublisher{}, newMockCache(), 0, 0)

	eventID := "evt_123"
	ownerID := "user_A"
	repo.byID[eventID] = &domain.Event{
		ID:      eventID,
		OwnerID: ownerID,
		Status:  domain.StatusPublished,
	}

	t.Run("owner_can_cancel", func(t *testing.T) {
		ev, err := svc.Cancel(context.Background(), eventID, ownerID, "user")
		assert.NoError(t, err)
		assert.Equal(t, domain.StatusCanceled, ev.Status)
		assert.NotNil(t, ev.CanceledAt)
	})

	t.Run("admin_can_cancel_any_event", func(t *testing.T) {
		repo.byID[eventID].Status = domain.StatusPublished
		ev, err := svc.Cancel(context.Background(), eventID, "admin_user", "admin")
		assert.NoError(t, err)
		assert.Equal(t, domain.StatusCanceled, ev.Status)
	})
}

func TestService_Publish_Validation(t *testing.T) {
	now := mustTime(t, "2025-12-25T10:00:00Z")
	repo := newMemRepo()
	svc := New(repo, fakeClock{t: now}, NoopPublisher{}, newMockCache(), 0, 0)

	t.Run("cannot_publish_with_start_time_in_past", func(t *testing.T) {
		eventID := "evt_past"
		repo.byID[eventID] = &domain.Event{
			ID:        eventID,
			OwnerID:   "owner",
			Status:    domain.StatusDraft,
			StartTime: now.Add(-1 * time.Hour),
			EndTime:   now.Add(1 * time.Hour),
		}

		_, err := svc.Publish(context.Background(), eventID, "owner", "user")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "cannot publish event in the past")
	})
}

func TestService_ListPublic_CursorLogic(t *testing.T) {
	now := mustTime(t, "2025-12-25T10:00:00Z")
	repo := newMemRepo()
	svc := New(repo, fakeClock{t: now}, NoopPublisher{}, newMockCache(), 0, 0)

	t.Run("time_sort_without_cursor", func(t *testing.T) {
		f := ListFilter{Sort: "time"}
		res, err := svc.ListPublic(context.Background(), f)
		assert.NoError(t, err)
		assert.Equal(t, "", res.NextCursor)
	})

	t.Run("relevance_sort_requires_query", func(t *testing.T) {
		f := ListFilter{Sort: "relevance", Query: ""}
		_, err := svc.ListPublic(context.Background(), f)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "required when sort=relevance")
	})
}
