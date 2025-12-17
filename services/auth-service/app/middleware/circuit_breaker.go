package middleware

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
	defer cb.mu.Unlock()

	// Check if we should transition states
	cb.updateState()

	// Handle different states
	switch cb.state {
	case StateOpen:
		// Circuit is open - fail fast
		return errors.New("circuit breaker is open - service unavailable")

	case StateHalfOpen:
		// Half-open - allow limited requests
		if cb.halfOpenCalls >= cb.halfOpenMaxCalls {
			return errors.New("circuit breaker is half-open - too many concurrent requests")
		}
		cb.halfOpenCalls++

	case StateClosed:
		// Normal operation - proceed
	}

	// Execute the function
	err := fn()

	cb.mu.Lock()
	defer cb.mu.Unlock()

	if err != nil {
		// Request failed
		cb.recordFailure()
		return err
	}

	// Request succeeded
	cb.recordSuccess()
	return nil
}

// updateState transitions the circuit breaker state based on current conditions
func (cb *CircuitBreaker) updateState() {
	now := time.Now()

	switch cb.state {
	case StateClosed:
		// If we have too many failures, open the circuit
		if cb.failureCount >= cb.maxFailures {
			cb.state = StateOpen
			cb.lastFailTime = now
		}

	case StateOpen:
		// If enough time has passed, try half-open
		if now.Sub(cb.lastFailTime) >= cb.resetTimeout {
			cb.state = StateHalfOpen
			cb.halfOpenCalls = 0
			cb.failureCount = 0
		}

	case StateHalfOpen:
		// Stay in half-open until we succeed or fail enough
		// State transitions handled in recordSuccess/recordFailure
	}
}

// recordFailure records a failed request
func (cb *CircuitBreaker) recordFailure() {
	cb.failureCount++
	cb.lastFailTime = time.Now()

	if cb.state == StateHalfOpen {
		// Failed in half-open - go back to open
		cb.state = StateOpen
		cb.halfOpenCalls = 0
	}
}

// recordSuccess records a successful request
func (cb *CircuitBreaker) recordSuccess() {
	if cb.state == StateHalfOpen {
		// Success in half-open - close the circuit
		cb.state = StateClosed
		cb.failureCount = 0
		cb.halfOpenCalls = 0
	} else if cb.state == StateClosed {
		// Reset failure count on success
		cb.failureCount = 0
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
