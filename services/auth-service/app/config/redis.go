package config

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// NewRedisClient creates a Redis client using environment variables with sane defaults.
// Defaults target a local dev instance on :6379 without auth.
func NewRedisClient() (*redis.Client, error) {
	addr := GetString("REDIS_ADDR", "localhost:6379")
	password := GetString("REDIS_PASSWORD", "")
	db := GetInt("REDIS_DB", 0)

	client := redis.NewClient(&redis.Options{
		Addr:         addr,
		Password:     password,
		DB:           db,
		PoolSize:     GetInt("REDIS_POOL_SIZE", 10),
		MinIdleConns: GetInt("REDIS_MIN_IDLE_CONNS", 2),
		DialTimeout:  5 * time.Second,
		ReadTimeout:  3 * time.Second,
		WriteTimeout: 3 * time.Second,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("redis ping failed: %w", err)
	}
	return client, nil
}
