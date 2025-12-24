package idempotency

import (
	"context"
	"fmt"
	"time"

	"github.com/gomodule/redigo/redis"
	"github.com/rs/zerolog"
)

type RedisStore struct {
	pool *redis.Pool
	lg   zerolog.Logger
}

func NewRedisPool(addr, password string, db int) *redis.Pool {
	return &redis.Pool{
		MaxIdle:     5,
		MaxActive:   20,
		IdleTimeout: 60 * time.Second,
		Wait:        true,
		Dial: func() (redis.Conn, error) {
			c, err := redis.Dial(
				"tcp",
				addr,
				redis.DialConnectTimeout(3*time.Second),
				redis.DialReadTimeout(3*time.Second),
				redis.DialWriteTimeout(3*time.Second),
			)
			if err != nil {
				return nil, err
			}
			if password != "" {
				if _, err := c.Do("AUTH", password); err != nil {
					_ = c.Close()
					return nil, err
				}
			}
			if db != 0 {
				if _, err := c.Do("SELECT", db); err != nil {
					_ = c.Close()
					return nil, err
				}
			}
			return c, nil
		},
		TestOnBorrow: func(c redis.Conn, t time.Time) error {
			// light ping if connection is older
			if time.Since(t) < 30*time.Second {
				return nil
			}
			_, err := c.Do("PING")
			return err
		},
	}
}

func NewRedisStore(pool *redis.Pool, lg zerolog.Logger) *RedisStore {
	return &RedisStore{
		pool: pool,
		lg:   lg.With().Str("component", "idem_store").Logger(),
	}
}

// MarkSentNX implements notify.IdempotencyStore
func (s *RedisStore) MarkSentNX(ctx context.Context, key string, ttl time.Duration) (bool, error) {
	if key == "" {
		return false, fmt.Errorf("empty key")
	}
	conn, err := s.pool.GetContext(ctx)
	if err != nil {
		return false, err
	}
	defer conn.Close()

	// SET key "1" NX EX <seconds>
	secs := int64(ttl / time.Second)
	if secs <= 0 {
		secs = 86400 // fallback 24h
	}

	reply, err := redis.String(conn.Do("SET", key, "1", "NX", "EX", secs))
	if err == redis.ErrNil {
		// key already exists
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return reply == "OK", nil
}

// Seen implements notify.IdempotencyStore
func (s *RedisStore) Seen(ctx context.Context, key string) (bool, error) {
	if key == "" {
		return false, fmt.Errorf("empty key")
	}
	conn, err := s.pool.GetContext(ctx)
	if err != nil {
		return false, err
	}
	defer conn.Close()

	// EXISTS key -> 1/0
	n, err := redis.Int(conn.Do("EXISTS", key))
	if err != nil {
		return false, err
	}
	return n == 1, nil
}

// MarkSent implements notify.IdempotencyStore
func (s *RedisStore) MarkSent(ctx context.Context, key string, ttl time.Duration) error {
	if key == "" {
		return fmt.Errorf("empty key")
	}
	conn, err := s.pool.GetContext(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()

	secs := int64(ttl / time.Second)
	if secs <= 0 {
		secs = 86400
	}

	// SET key "1" EX <seconds>
	_, err = conn.Do("SET", key, "1", "EX", secs)
	return err
}
