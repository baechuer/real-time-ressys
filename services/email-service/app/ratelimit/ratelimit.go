package ratelimit

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// RateLimiter provides rate limiting functionality
type RateLimiter struct {
	client *redis.Client
}

// NewRateLimiter creates a new rate limiter
func NewRateLimiter(client *redis.Client) *RateLimiter {
	return &RateLimiter{
		client: client,
	}
}

// Check checks if an email address has exceeded the rate limit
// Returns error if rate limit exceeded, nil otherwise
func (rl *RateLimiter) Check(ctx context.Context, email string, maxRequests int, window time.Duration) error {
	if rl.client == nil {
		// If Redis is not available, allow request (fail open)
		// In production, you might want to fail closed
		return nil
	}

	key := fmt.Sprintf("ratelimit:email:%s", email)
	
	// Use Redis INCR with expiration
	// This is a sliding window rate limiter
	count, err := rl.client.Incr(ctx, key).Result()
	if err != nil {
		// If Redis fails, allow request (fail open)
		// Log error but don't block
		return nil
	}

	// Set expiration on first request
	if count == 1 {
		rl.client.Expire(ctx, key, window)
	}

	if count > int64(maxRequests) {
		return fmt.Errorf("rate limit exceeded: %d requests in %v", maxRequests, window)
	}

	return nil
}

// CheckPerIP checks if an IP address has exceeded the rate limit
func (rl *RateLimiter) CheckPerIP(ctx context.Context, ip string, maxRequests int, window time.Duration) error {
	if rl.client == nil {
		return nil
	}

	key := fmt.Sprintf("ratelimit:ip:%s", ip)
	
	count, err := rl.client.Incr(ctx, key).Result()
	if err != nil {
		return nil
	}

	if count == 1 {
		rl.client.Expire(ctx, key, window)
	}

	if count > int64(maxRequests) {
		return fmt.Errorf("rate limit exceeded for IP: %d requests in %v", maxRequests, window)
	}

	return nil
}

