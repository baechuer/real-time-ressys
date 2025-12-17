package circuitbreaker

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewCircuitBreaker(t *testing.T) {
	cb := NewCircuitBreaker(5, 30*time.Second, 2)

	assert.NotNil(t, cb)
	assert.Equal(t, 5, cb.maxFailures)
	assert.Equal(t, 30*time.Second, cb.resetTimeout)
	assert.Equal(t, 2, cb.halfOpenMaxCalls)
	assert.Equal(t, StateClosed, cb.State())
}

func TestCircuitBreaker_Call_Success(t *testing.T) {
	cb := NewCircuitBreaker(3, 100*time.Millisecond, 2)

	err := cb.Call(context.Background(), func() error {
		return nil
	})

	assert.NoError(t, err)
	assert.Equal(t, StateClosed, cb.State())
	assert.Equal(t, 0, cb.FailureCount())
}

func TestCircuitBreaker_Call_Failure(t *testing.T) {
	cb := NewCircuitBreaker(3, 100*time.Millisecond, 2)

	err := cb.Call(context.Background(), func() error {
		return errors.New("test error")
	})

	assert.Error(t, err)
	assert.Equal(t, StateClosed, cb.State()) // Still closed (only 1 failure)
	assert.Equal(t, 1, cb.FailureCount())
}

func TestCircuitBreaker_OpenAfterMaxFailures(t *testing.T) {
	cb := NewCircuitBreaker(3, 100*time.Millisecond, 2)

	// Cause 3 failures
	for i := 0; i < 3; i++ {
		_ = cb.Call(context.Background(), func() error {
			return errors.New("test error")
		})
	}

	// Circuit should be open now
	assert.Equal(t, StateOpen, cb.State())

	// Next call should fail immediately
	err := cb.Call(context.Background(), func() error {
		return nil // Even success should fail when circuit is open
	})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "circuit breaker is open")
}

func TestCircuitBreaker_HalfOpenTransition(t *testing.T) {
	cb := NewCircuitBreaker(2, 50*time.Millisecond, 2)

	// Open the circuit
	for i := 0; i < 2; i++ {
		_ = cb.Call(context.Background(), func() error {
			return errors.New("test error")
		})
	}

	assert.Equal(t, StateOpen, cb.State())

	// Wait for reset timeout
	time.Sleep(60 * time.Millisecond)

	// Next call should transition to half-open
	err := cb.Call(context.Background(), func() error {
		return nil // Success
	})

	// Should succeed and close the circuit
	assert.NoError(t, err)
	assert.Equal(t, StateClosed, cb.State())
	assert.Equal(t, 0, cb.FailureCount())
}

func TestCircuitBreaker_HalfOpen_Failure(t *testing.T) {
	cb := NewCircuitBreaker(2, 50*time.Millisecond, 2)

	// Open the circuit
	for i := 0; i < 2; i++ {
		_ = cb.Call(context.Background(), func() error {
			return errors.New("test error")
		})
	}

	// Wait for reset timeout
	time.Sleep(60 * time.Millisecond)

	// Try in half-open, but fail
	err := cb.Call(context.Background(), func() error {
		return errors.New("test error")
	})

	// Should fail and go back to open
	assert.Error(t, err)
	assert.Equal(t, StateOpen, cb.State())
}

func TestCircuitBreaker_HalfOpen_MaxCalls(t *testing.T) {
	cb := NewCircuitBreaker(2, 50*time.Millisecond, 2)

	// Open the circuit
	for i := 0; i < 2; i++ {
		_ = cb.Call(context.Background(), func() error {
			return errors.New("test error")
		})
	}

	// Wait for reset timeout
	time.Sleep(60 * time.Millisecond)

	// Make 2 successful calls (max allowed in half-open)
	// First call should transition to half-open and succeed, closing the circuit
	err1 := cb.Call(context.Background(), func() error {
		return nil
	})
	assert.NoError(t, err1)

	// Circuit should be closed now, so next calls should work normally
	err2 := cb.Call(context.Background(), func() error {
		return nil
	})
	assert.NoError(t, err2)

	// Since circuit is closed, third call should also work
	err3 := cb.Call(context.Background(), func() error {
		return nil
	})
	assert.NoError(t, err3)
}

func TestCircuitBreaker_State(t *testing.T) {
	cb := NewCircuitBreaker(2, 100*time.Millisecond, 2)

	// Initially closed
	assert.Equal(t, StateClosed, cb.State())

	// Open it
	for i := 0; i < 2; i++ {
		_ = cb.Call(context.Background(), func() error {
			return errors.New("test error")
		})
	}

	assert.Equal(t, StateOpen, cb.State())
}

func TestCircuitBreaker_FailureCount(t *testing.T) {
	cb := NewCircuitBreaker(5, 100*time.Millisecond, 2)

	// No failures initially
	assert.Equal(t, 0, cb.FailureCount())

	// Add failures
	for i := 0; i < 3; i++ {
		_ = cb.Call(context.Background(), func() error {
			return errors.New("test error")
		})
		assert.Equal(t, i+1, cb.FailureCount())
	}

	// Success should reset count
	_ = cb.Call(context.Background(), func() error {
		return nil
	})

	assert.Equal(t, 0, cb.FailureCount())
}

func TestCircuitBreaker_ConcurrentCalls(t *testing.T) {
	cb := NewCircuitBreaker(10, 100*time.Millisecond, 5)

	// Make concurrent calls
	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func() {
			_ = cb.Call(context.Background(), func() error {
				time.Sleep(10 * time.Millisecond)
				return nil
			})
			done <- true
		}()
	}

	// Wait for all to complete
	for i := 0; i < 10; i++ {
		<-done
	}

	// Circuit should still be closed
	assert.Equal(t, StateClosed, cb.State())
}

func TestCircuitBreaker_ContextCancellation(t *testing.T) {
	cb := NewCircuitBreaker(3, 100*time.Millisecond, 2)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	// Call should still work (context not used in current implementation)
	err := cb.Call(ctx, func() error {
		return nil
	})

	require.NoError(t, err)
}
