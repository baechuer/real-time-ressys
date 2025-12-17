package retry

import (
	"context"
	"fmt"
	"math"
	"os"
	"strconv"
	"time"

	appErrors "github.com/baechuer/real-time-ressys/services/email-service/app/errors"
)

// Config holds retry configuration
type Config struct {
	MaxRetries    int
	InitialDelay time.Duration
	MaxDelay      time.Duration
}

// LoadConfig loads retry configuration from environment
func LoadConfig() *Config {
	maxRetries, _ := strconv.Atoi(os.Getenv("MAX_RETRIES"))
	if maxRetries == 0 {
		maxRetries = 3
	}

	initialDelay, _ := time.ParseDuration(os.Getenv("RETRY_INITIAL_DELAY"))
	if initialDelay == 0 {
		initialDelay = 1 * time.Second
	}

	maxDelay, _ := time.ParseDuration(os.Getenv("RETRY_MAX_DELAY"))
	if maxDelay == 0 {
		maxDelay = 30 * time.Second
	}

	return &Config{
		MaxRetries:    maxRetries,
		InitialDelay: initialDelay,
		MaxDelay:     maxDelay,
	}
}

// IsRetryable checks if an error is retryable
func IsRetryable(err error) bool {
	if err == nil {
		return false
	}

	appErr, ok := err.(*appErrors.AppError)
	if !ok {
		// Default: assume retryable for unknown errors
		return true
	}

	// Retryable errors
	switch appErr.Code {
	case appErrors.ErrCodeRetryable:
		return true
	case appErrors.ErrCodeEmailProvider:
		// Email provider errors are usually retryable (network, rate limits)
		return true
	case appErrors.ErrCodeInternal:
		// Internal errors might be retryable
		return true
	}

	// Non-retryable errors
	switch appErr.Code {
	case appErrors.ErrCodeInvalidInput:
		return false
	case appErrors.ErrCodePermanentFailure:
		return false
	}

	return false
}

// CalculateDelay calculates exponential backoff delay
func CalculateDelay(attempt int, config *Config) time.Duration {
	delay := time.Duration(float64(config.InitialDelay) * math.Pow(2, float64(attempt)))
	if delay > config.MaxDelay {
		delay = config.MaxDelay
	}
	return delay
}

// Retry executes a function with retry logic
func Retry(ctx context.Context, config *Config, fn func() error) error {
	var lastErr error

	for attempt := 0; attempt <= config.MaxRetries; attempt++ {
		if attempt > 0 {
			// Wait before retry
			delay := CalculateDelay(attempt-1, config)
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(delay):
			}
		}

		err := fn()
		if err == nil {
			return nil // Success
		}

		lastErr = err

		// Check if error is retryable
		if !IsRetryable(err) {
			return err // Don't retry non-retryable errors
		}

		// If this was the last attempt, return error
		if attempt == config.MaxRetries {
			break
		}
	}

	return fmt.Errorf("max retries exceeded: %w", lastErr)
}

