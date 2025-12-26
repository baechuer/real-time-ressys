package domain

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func mustTime(t *testing.T, s string) time.Time {
	t.Helper()
	tt, err := time.Parse(time.RFC3339, s)
	if err != nil {
		t.Fatalf("bad time %q: %v", s, err)
	}
	return tt.UTC()
}

func TestNewDraft_Validation(t *testing.T) {
	now := mustTime(t, "2025-12-25T10:00:00Z")
	start := now.Add(1 * time.Hour)
	end := now.Add(2 * time.Hour)

	t.Run("valid_draft_creation", func(t *testing.T) {
		e, err := NewDraft("owner-1", "Pool Party", "Summer vibes", "Sydney", "Social", start, end, 50, now)
		assert.NoError(t, err)
		assert.NotNil(t, e)
		assert.Equal(t, StatusDraft, e.Status)
		assert.Equal(t, start.UTC(), e.StartTime)
		assert.NotEmpty(t, e.ID)
	})

	t.Run("fail_on_empty_owner", func(t *testing.T) {
		_, err := NewDraft("", "Title", "Desc", "City", "Cat", start, end, 0, now)
		assert.Error(t, err)
		assert.Equal(t, CodeValidation, err.(*AppError).Code)
	})

	t.Run("fail_on_invalid_capacity", func(t *testing.T) {
		_, err := NewDraft("u1", "t", "d", "c", "cat", start, end, -1, now)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "capacity must be >= 0")
	})
}

func TestEvent_Lifecycle_Transitions(t *testing.T) {
	now := mustTime(t, "2025-12-25T10:00:00Z")

	t.Run("publish_success", func(t *testing.T) {
		e, _ := NewDraft("u1", "t", "d", "c", "cat", now.Add(1*time.Hour), now.Add(2*time.Hour), 0, now)
		err := e.Publish(now)
		assert.NoError(t, err)
		assert.Equal(t, StatusPublished, e.Status)
		assert.NotNil(t, e.PublishedAt)
	})

	t.Run("cannot_publish_in_past", func(t *testing.T) {
		e, _ := NewDraft("u1", "t", "d", "c", "cat", now.Add(-1*time.Hour), now.Add(1*time.Hour), 0, now)
		err := e.Publish(now)
		assert.Error(t, err)
		assert.Equal(t, CodeValidation, err.(*AppError).Code)
	})

	t.Run("cancel_published_event_success", func(t *testing.T) {
		e, _ := NewDraft("u1", "t", "d", "c", "cat", now.Add(1*time.Hour), now.Add(2*time.Hour), 0, now)
		_ = e.Publish(now)
		err := e.Cancel(now)
		assert.NoError(t, err)
		assert.Equal(t, StatusCanceled, e.Status)
		assert.NotNil(t, e.CanceledAt)
	})

	t.Run("cannot_cancel_ended_event", func(t *testing.T) {
		e, _ := NewDraft("u1", "t", "d", "c", "cat", now.Add(-2*time.Hour), now.Add(-1*time.Hour), 0, now)
		err := e.Cancel(now)
		assert.Error(t, err)
		assert.Equal(t, CodeInvalidState, err.(*AppError).Code)
	})
}

func TestEvent_ApplyUpdate_Rules(t *testing.T) {
	now := mustTime(t, "2025-12-25T10:00:00Z")
	start := now.Add(1 * time.Hour)
	end := now.Add(2 * time.Hour)
	e, _ := NewDraft("u1", "Old", "d", "c", "cat", start, end, 0, now)

	t.Run("update_all_fields_success", func(t *testing.T) {
		newTitle := "New"
		newDesc := "New Desc"
		newCity := "Melbourne"
		newCat := "Music"
		newCap := 100
		newStart := start.Add(30 * time.Minute)
		newEnd := end.Add(30 * time.Minute)

		err := e.ApplyUpdate(&newTitle, &newDesc, &newCity, &newCat, &newStart, &newEnd, &newCap, now)
		assert.NoError(t, err)
		assert.Equal(t, "New", e.Title)
		assert.Equal(t, 100, e.Capacity)
		assert.Equal(t, newStart.UTC(), e.StartTime)
	})

	t.Run("enforce_logic_during_update", func(t *testing.T) {
		badEnd := e.StartTime.Add(-10 * time.Minute)
		err := e.ApplyUpdate(nil, nil, nil, nil, nil, &badEnd, nil, now)
		assert.Error(t, err)
	})
}
