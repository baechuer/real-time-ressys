package dto

import (
	"testing"
	"time"

	"github.com/baechuer/real-time-ressys/services/event-service/internal/domain"
	"github.com/stretchr/testify/assert"
)

func TestToEventResp(t *testing.T) {
	// Base times for reference
	now := time.Now().UTC()
	futureStart := now.Add(2 * time.Hour)
	futureEnd := now.Add(4 * time.Hour)
	pastStart := now.Add(-4 * time.Hour)
	pastEnd := now.Add(-2 * time.Hour)

	t.Run("successfully_maps_all_fields", func(t *testing.T) {
		e := &domain.Event{
			ID:          "evt_1",
			OwnerID:     "user_1",
			Title:       "Test Event",
			Description: "Test Desc",
			City:        "Sydney",
			Category:    "Tech",
			StartTime:   futureStart,
			EndTime:     futureEnd,
			Capacity:    50,
			Status:      domain.StatusPublished,
			CreatedAt:   now,
			UpdatedAt:   now,
		}

		resp := ToEventResp(e, now)

		assert.Equal(t, e.ID, resp.ID)
		assert.Equal(t, e.Title, resp.Title)
		assert.Equal(t, "published", resp.Status)
		assert.False(t, resp.Ended)
		assert.True(t, resp.Joinable)
	})

	t.Run("joinable_logic_rules", func(t *testing.T) {
		tests := []struct {
			name     string
			status   domain.EventStatus
			start    time.Time
			end      time.Time
			expected bool
		}{
			{"draft_is_not_joinable", domain.StatusDraft, futureStart, futureEnd, false},
			{"canceled_is_not_joinable", domain.StatusCanceled, futureStart, futureEnd, false},
			{"ended_is_not_joinable", domain.StatusPublished, pastStart, pastEnd, false},
			{"published_and_active_is_joinable", domain.StatusPublished, futureStart, futureEnd, true},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				e := &domain.Event{
					Status:    tt.status,
					StartTime: tt.start,
					EndTime:   tt.end,
				}
				resp := ToEventResp(e, now)
				assert.Equal(t, tt.expected, resp.Joinable, "Joinable logic failed for: "+tt.name)
			})
		}
	})

	t.Run("ended_logic_rules", func(t *testing.T) {
		e := &domain.Event{EndTime: pastEnd}
		resp := ToEventResp(e, now)
		assert.True(t, resp.Ended)

		e.EndTime = futureEnd
		resp = ToEventResp(e, now)
		assert.False(t, resp.Ended)
	})
}
