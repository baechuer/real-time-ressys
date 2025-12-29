package postgres

import (
	"math/rand"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestComputeNextRetry_Bounds(t *testing.T) {
	rand.Seed(1)

	d0 := computeNextRetry(-1)
	require.GreaterOrEqual(t, d0, 4*time.Second)
	require.LessOrEqual(t, d0, 6*time.Second)

	d10 := computeNextRetry(10)
	require.GreaterOrEqual(t, d10, 850*time.Second)
	require.LessOrEqual(t, d10, 1250*time.Second)

	d20 := computeNextRetry(20)
	require.GreaterOrEqual(t, d20, 1500*time.Second)
	require.LessOrEqual(t, d20, 2100*time.Second)
}
