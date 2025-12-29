package domain_test

import (
	"testing"

	"github.com/baechuer/real-time-ressys/services/join-service/internal/domain"
	"github.com/stretchr/testify/assert"
)

func TestWaitlistMax(t *testing.T) {
	tests := []struct {
		name     string
		capacity int
		expected int
	}{
		{"Unlimited capacity", 0, 0}, // 0 capacity = unlimited, so 0 waitlist cap (repo handles logic)
		{"Closed event", -1, 0},      // Closed
		{"Small event", 10, 20},      // Min cap hits (20)
		{"Medium event", 50, 50},     // 1x multiplier
		{"Large event", 200, 100},    // Hard cap hits (100)
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := domain.WaitlistMax(tt.capacity)
			assert.Equal(t, tt.expected, got)
		})
	}
}
