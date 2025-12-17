package circuitbreaker

import (
	"context"
	"errors"
	"sync"
	"time"
)

// CircuitState represents the state of a circuit breaker
type CircuitState int

const (
	StateClosed   CircuitState = iota // Normal operation - requests pass through
	StateOpen                         // Circuit is open - requests fail immediately
	StateHalfOpen                     // Testing if service recovered - limited requests pass
)

// CircuitBreaker implements the circuit breaker pattern to prevent cascading failures
type CircuitBreaker struct {
	maxFailures      int           // Number of failures before opening circuit
	resetTimeout     time.Duration // Time to wait before attempting half-open
	halfOpenMaxCalls int           // Max calls allowed in half-open state

	mu            sync.RWMutex
	state         CircuitState
	failureCount  int
	lastFailTime  time.Time
	halfOpenCalls int
}

// NewCircuitBreaker creates a new circuit breaker with the specified configuration
func NewCircuitBreaker(maxFailures int, resetTimeout time.Duration, halfOpenMaxCalls int) *CircuitBreaker {
	return &CircuitBreaker{
		maxFailures:      maxFailures,
		resetTimeout:     resetTimeout,
		halfOpenMaxCalls: halfOpenMaxCalls,
		state:            StateClosed,
	}
}

// Call executes a function with circuit breaker protection
func (cb *CircuitBreaker) Call(ctx context.Context, fn func() error) error {
	cb.mu.Lock()

	// Check if we should transition states
	cb.updateState()

	// Handle different states
	switch cb.state {
	case StateOpen:
		cb.mu.Unlock()
		return errors.New("circuit breaker is open")
	case StateHalfOpen:
		if cb.halfOpenCalls >= cb.halfOpenMaxCalls {
			cb.mu.Unlock()
			return errors.New("circuit breaker half-open limit reached")
		}
		cb.halfOpenCalls++
		cb.mu.Unlock()
	default:
		cb.mu.Unlock()
	}

	// Execute the function
	err := fn()

	cb.mu.Lock()
	defer cb.mu.Unlock()

	if err != nil {
		// Record failure
		cb.recordFailure()
		return err
	}

	// Record success
	cb.recordSuccess()
	return nil
}

// updateState updates the circuit breaker state based on current conditions
func (cb *CircuitBreaker) updateState() {
	now := time.Now()

	switch cb.state {
	case StateClosed:
		// Check if we should open
		if cb.failureCount >= cb.maxFailures {
			cb.state = StateOpen
			cb.lastFailTime = now
		}
	case StateOpen:
		// Check if we should transition to half-open
		if now.Sub(cb.lastFailTime) >= cb.resetTimeout {
			cb.state = StateHalfOpen
			cb.halfOpenCalls = 0
		}
	case StateHalfOpen:
		// Stay in half-open until success or failure
		// Will transition based on recordSuccess/recordFailure
	}
}

// recordFailure records a failure and updates state
func (cb *CircuitBreaker) recordFailure() {
	cb.failureCount++
	cb.lastFailTime = time.Now()

	if cb.state == StateHalfOpen {
		// Failed in half-open, go back to open
		cb.state = StateOpen
		cb.halfOpenCalls = 0
	} else if cb.failureCount >= cb.maxFailures {
		// Too many failures, open circuit
		cb.state = StateOpen
	}
}

// recordSuccess records a success and updates state
func (cb *CircuitBreaker) recordSuccess() {
	cb.failureCount = 0

	if cb.state == StateHalfOpen {
		// Success in half-open, close circuit
		cb.state = StateClosed
		cb.halfOpenCalls = 0
	}
}

// State returns the current state of the circuit breaker
func (cb *CircuitBreaker) State() CircuitState {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.state
}

// FailureCount returns the current failure count
func (cb *CircuitBreaker) FailureCount() int {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.failureCount
}

