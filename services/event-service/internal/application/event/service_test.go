package event

import (
	"context"
	"testing"
	"time"

	"github.com/baechuer/real-time-ressys/services/event-service/internal/domain"
	"github.com/stretchr/testify/assert" // 建议使用 testify 简化断言
)

// --- Mocks & Helpers ---

type fakeClock struct{ t time.Time }

func (c fakeClock) Now() time.Time { return c.t }

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
func (m *memRepo) ListPublic(ctx context.Context, f ListFilter) ([]*domain.Event, int, error) {
	return []*domain.Event{}, 0, nil
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
	svc := New(repo, fakeClock{t: now})

	// 1. Setup a published event
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
		// Reset status
		repo.byID[eventID].Status = domain.StatusPublished

		ev, err := svc.Cancel(context.Background(), eventID, "admin_user", "admin")
		assert.NoError(t, err)
		assert.Equal(t, domain.StatusCanceled, ev.Status)
	})

	t.Run("wrong_user_cannot_cancel", func(t *testing.T) {
		repo.byID[eventID].Status = domain.StatusPublished

		_, err := svc.Cancel(context.Background(), eventID, "user_B", "user")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not allowed")
	})
}

func TestService_Publish_Validation(t *testing.T) {
	now := mustTime(t, "2025-12-25T10:00:00Z")
	repo := newMemRepo()
	svc := New(repo, fakeClock{t: now})

	t.Run("cannot_publish_with_start_time_in_past", func(t *testing.T) {
		eventID := "evt_past"
		repo.byID[eventID] = &domain.Event{
			ID:        eventID,
			OwnerID:   "owner",
			Status:    domain.StatusDraft,
			StartTime: now.Add(-1 * time.Hour), // 过去的时间
			EndTime:   now.Add(1 * time.Hour),
		}

		_, err := svc.Publish(context.Background(), eventID, "owner", "user")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "must be in the future")
	})

	t.Run("cannot_publish_if_already_canceled", func(t *testing.T) {
		eventID := "evt_canceled"
		repo.byID[eventID] = &domain.Event{
			ID:      eventID,
			OwnerID: "owner",
			Status:  domain.StatusCanceled,
		}

		_, err := svc.Publish(context.Background(), eventID, "owner", "user")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "already canceled")
	})
}

func TestService_GetPublic_Visibility(t *testing.T) {
	repo := newMemRepo()
	svc := New(repo, fakeClock{t: time.Now()})

	t.Run("hidden_if_draft", func(t *testing.T) {
		repo.byID["d1"] = &domain.Event{ID: "d1", Status: domain.StatusDraft}
		_, err := svc.GetPublic(context.Background(), "d1")
		assert.Error(t, err)
	})

	t.Run("visible_if_published", func(t *testing.T) {
		repo.byID["p1"] = &domain.Event{ID: "p1", Status: domain.StatusPublished}
		ev, err := svc.GetPublic(context.Background(), "p1")
		assert.NoError(t, err)
		assert.NotNil(t, ev)
	})
}

func TestListFilter_Normalize(t *testing.T) {
	t.Run("default_pagination", func(t *testing.T) {
		f := ListFilter{}
		err := f.Normalize()
		assert.NoError(t, err)
		assert.Equal(t, 1, f.Page)
		assert.Equal(t, 20, f.PageSize)
	})

	t.Run("enforce_max_pagesize", func(t *testing.T) {
		f := ListFilter{PageSize: 999}
		_ = f.Normalize()
		assert.Equal(t, 100, f.PageSize)
	})

	t.Run("invalid_time_range", func(t *testing.T) {
		from := time.Now()
		to := from.Add(-1 * time.Hour)
		f := ListFilter{From: &from, To: &to}
		err := f.Normalize()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "to must be >= from")
	})
}
