package domain

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// Helper: 之前你已经定义了，这里保持一致
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
		assert.True(t, strings.HasPrefix(e.ID, ""), "UUID should be generated")
	})

	t.Run("fail_on_empty_fields", func(t *testing.T) {
		_, err := NewDraft(" ", "", "desc", "city", "cat", start, end, 0, now)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "validation_error")
	})

	t.Run("fail_on_long_title", func(t *testing.T) {
		longTitle := strings.Repeat("A", 121)
		_, err := NewDraft("u1", longTitle, "desc", "city", "cat", start, end, 0, now)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "title is required and must be <= 120 chars")
	})

	t.Run("fail_on_invalid_time_range", func(t *testing.T) {
		// End before start
		_, err := NewDraft("u1", "t", "d", "c", "cat", end, start, 0, now)
		assert.Error(t, err)
	})
}

func TestEvent_StatusTransitions(t *testing.T) {
	now := mustTime(t, "2025-12-25T10:00:00Z")
	start := now.Add(time.Hour)
	end := now.Add(2 * time.Hour)

	t.Run("draft_to_published_success", func(t *testing.T) {
		e, _ := NewDraft("u1", "t", "d", "c", "cat", start, end, 0, now)
		err := e.Publish(now)
		assert.NoError(t, err)
		assert.Equal(t, StatusPublished, e.Status)
		assert.NotNil(t, e.PublishedAt)
	})

	t.Run("cannot_cancel_already_canceled", func(t *testing.T) {
		e, _ := NewDraft("u1", "t", "d", "c", "cat", start, end, 0, now)
		_ = e.Cancel(now)
		err := e.Cancel(now)
		assert.Error(t, err)
		assert.IsType(t, &AppError{}, err)
		assert.Equal(t, CodeInvalidState, err.(*AppError).Code)
	})
}

func TestEvent_ApplyUpdate_Rules(t *testing.T) {
	now := mustTime(t, "2025-12-25T10:00:00Z")
	start := now.Add(time.Hour)
	end := now.Add(2 * time.Hour)
	e, _ := NewDraft("u1", "Old Title", "d", "c", "cat", start, end, 0, now)

	t.Run("partial_update_success", func(t *testing.T) {
		newTitle := "New Awesome Title"
		err := e.ApplyUpdate(&newTitle, nil, nil, nil, nil, nil, nil, now.Add(time.Minute))
		assert.NoError(t, err)
		assert.Equal(t, "New Awesome Title", e.Title)
		assert.Equal(t, now.Add(time.Minute).UTC(), e.UpdatedAt)
	})

	t.Run("fail_update_if_ended", func(t *testing.T) {
		pastNow := end.Add(time.Hour) // Simulate time has passed the event end
		newTitle := "Late Update"
		err := e.ApplyUpdate(&newTitle, nil, nil, nil, nil, nil, nil, pastNow)
		assert.Error(t, err)
		assert.Equal(t, CodeInvalidState, err.(*AppError).Code)
	})

	t.Run("validate_time_range_during_update", func(t *testing.T) {
		newStart := end.Add(time.Hour) // Start after existing end
		err := e.ApplyUpdate(nil, nil, nil, nil, &newStart, nil, nil, now)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "end_time must be after start_time")
	})
}

func TestAppError_Formatting(t *testing.T) {
	t.Run("error_with_meta", func(t *testing.T) {
		err := ErrValidationMeta("invalid fields", map[string]string{"field": "required"})
		expected := "validation_error: invalid fields (map[field:required])"
		assert.Equal(t, expected, err.Error())
	})
}
