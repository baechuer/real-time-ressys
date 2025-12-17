package retry

import (
	"context"
	"errors"
	"testing"
	"time"

	appErrors "github.com/baechuer/real-time-ressys/services/email-service/app/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIsRetryable_RetryableError(t *testing.T) {
	err := appErrors.NewRetryableError("retryable error", nil)
	assert.True(t, IsRetryable(err))
}

func TestIsRetryable_EmailProviderError(t *testing.T) {
	err := appErrors.NewEmailProviderError("provider error", errors.New("network error"))
	assert.True(t, IsRetryable(err))
}

func TestIsRetryable_InvalidInputError(t *testing.T) {
	err := appErrors.NewInvalidInput("invalid input")
	assert.False(t, IsRetryable(err))
}

func TestIsRetryable_PermanentFailureError(t *testing.T) {
	err := appErrors.NewPermanentFailure("permanent failure", errors.New("auth failed"))
	assert.False(t, IsRetryable(err))
}

func TestIsRetryable_NilError(t *testing.T) {
	assert.False(t, IsRetryable(nil))
}

func TestIsRetryable_UnknownError(t *testing.T) {
	err := errors.New("unknown error")
	// Default behavior: assume retryable for unknown errors
	assert.True(t, IsRetryable(err))
}

func TestCalculateDelay(t *testing.T) {
	config := &Config{
		InitialDelay: 1 * time.Second,
		MaxDelay:     30 * time.Second,
	}

	// First attempt (attempt 0)
	delay := CalculateDelay(0, config)
	assert.Equal(t, 1*time.Second, delay)

	// Second attempt (attempt 1)
	delay = CalculateDelay(1, config)
	assert.Equal(t, 2*time.Second, delay)

	// Third attempt (attempt 2)
	delay = CalculateDelay(2, config)
	assert.Equal(t, 4*time.Second, delay)

	// Fourth attempt (attempt 3)
	delay = CalculateDelay(3, config)
	assert.Equal(t, 8*time.Second, delay)
}

func TestCalculateDelay_MaxDelay(t *testing.T) {
	config := &Config{
		InitialDelay: 1 * time.Second,
		MaxDelay:     5 * time.Second,
	}

	// Attempt that would exceed max delay
	delay := CalculateDelay(10, config)
	assert.Equal(t, config.MaxDelay, delay)
}

func TestRetry_SuccessOnFirstAttempt(t *testing.T) {
	config := &Config{
		MaxRetries:   3,
		InitialDelay: 100 * time.Millisecond,
		MaxDelay:     1 * time.Second,
	}

	ctx := context.Background()
	attempts := 0

	err := Retry(ctx, config, func() error {
		attempts++
		return nil
	})

	require.NoError(t, err)
	assert.Equal(t, 1, attempts)
}

func TestRetry_SuccessOnRetry(t *testing.T) {
	config := &Config{
		MaxRetries:   3,
		InitialDelay: 10 * time.Millisecond,
		MaxDelay:     1 * time.Second,
	}

	ctx := context.Background()
	attempts := 0

	err := Retry(ctx, config, func() error {
		attempts++
		if attempts < 2 {
			return appErrors.NewRetryableError("temporary error", nil)
		}
		return nil
	})

	require.NoError(t, err)
	assert.Equal(t, 2, attempts)
}

func TestRetry_MaxRetriesExceeded(t *testing.T) {
	config := &Config{
		MaxRetries:   2,
		InitialDelay: 10 * time.Millisecond,
		MaxDelay:     1 * time.Second,
	}

	ctx := context.Background()
	attempts := 0

	err := Retry(ctx, config, func() error {
		attempts++
		return appErrors.NewRetryableError("always fails", nil)
	})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "max retries exceeded")
	assert.Equal(t, 3, attempts) // Initial + 2 retries
}

func TestRetry_NonRetryableError(t *testing.T) {
	config := &Config{
		MaxRetries:   3,
		InitialDelay: 10 * time.Millisecond,
		MaxDelay:     1 * time.Second,
	}

	ctx := context.Background()
	attempts := 0

	err := Retry(ctx, config, func() error {
		attempts++
		return appErrors.NewInvalidInput("invalid input")
	})

	assert.Error(t, err)
	assert.Equal(t, 1, attempts, "should not retry non-retryable errors")
}

func TestRetry_ContextCancellation(t *testing.T) {
	config := &Config{
		MaxRetries:   5,
		InitialDelay: 100 * time.Millisecond,
		MaxDelay:     1 * time.Second,
	}

	ctx, cancel := context.WithCancel(context.Background())
	attempts := 0

	// Cancel context after first attempt
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	err := Retry(ctx, config, func() error {
		attempts++
		if attempts == 1 {
			return appErrors.NewRetryableError("temporary error", nil)
		}
		return nil
	})

	assert.Error(t, err)
	assert.Equal(t, context.Canceled, err)
	assert.Equal(t, 1, attempts, "should stop on context cancellation")
}

func TestLoadConfig_Defaults(t *testing.T) {
	config := LoadConfig()

	assert.Equal(t, 3, config.MaxRetries)
	assert.Equal(t, 1*time.Second, config.InitialDelay)
	assert.Equal(t, 30*time.Second, config.MaxDelay)
}

func TestRetry_ExponentialBackoff(t *testing.T) {
	config := &Config{
		MaxRetries:   3,
		InitialDelay: 10 * time.Millisecond,
		MaxDelay:     1 * time.Second,
	}

	ctx := context.Background()
	attempts := 0
	lastAttemptTime := time.Now()

	err := Retry(ctx, config, func() error {
		attempts++
		if attempts > 1 {
			// Check that delay increased
			elapsed := time.Since(lastAttemptTime)
			assert.GreaterOrEqual(t, elapsed, 10*time.Millisecond)
		}
		lastAttemptTime = time.Now()

		if attempts < 3 {
			return appErrors.NewRetryableError("temporary error", nil)
		}
		return nil
	})

	require.NoError(t, err)
	assert.Equal(t, 3, attempts)
}
