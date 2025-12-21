package redis

import (
	"context"
	"fmt"
	"time"

	goredis "github.com/redis/go-redis/v9"
)

// FixedWindowLimiter implements a fixed-window rate limiter using Redis:
// INCR key; if count == 1 then EXPIRE key window
// key should already include "identity" + "route" and window bucket.
type FixedWindowLimiter struct {
	rdb *goredis.Client
}

func NewFixedWindowLimiter(c *Client) *FixedWindowLimiter {
	if c == nil {
		return &FixedWindowLimiter{rdb: nil}
	}
	return &FixedWindowLimiter{rdb: c.rdb}
}

type Decision struct {
	Allowed    bool
	Limit      int
	Remaining  int
	RetryAfter time.Duration // 0 if allowed
	ResetAt    time.Time     // window end (best-effort)
	Count      int
}

// AllowFixedWindow returns whether request is allowed for given key+window.
// window must be >= 1s, limit >= 1.
func (l *FixedWindowLimiter) AllowFixedWindow(ctx context.Context, key string, limit int, window time.Duration) (Decision, error) {
	if limit <= 0 {
		return Decision{Allowed: true, Limit: limit, Remaining: limit}, nil
	}
	if window <= 0 {
		window = time.Minute
	}
	if l.rdb == nil {
		// Redis disabled => allow (fail-open). If you prefer fail-closed, change here.
		return Decision{Allowed: true, Limit: limit, Remaining: limit}, nil
	}

	// Lua to ensure atomic INCR + set expire on first hit
	// returns: {count, ttl_ms}
	const lua = `
local c = redis.call("INCR", KEYS[1])
if c == 1 then
  redis.call("PEXPIRE", KEYS[1], ARGV[1])
end
local ttl = redis.call("PTTL", KEYS[1])
return {c, ttl}
`
	ttlms := window.Milliseconds()
	if ttlms <= 0 {
		ttlms = int64(window / time.Millisecond)
		if ttlms <= 0 {
			ttlms = 60000
		}
	}

	res, err := l.rdb.Eval(ctx, lua, []string{key}, ttlms).Result()
	if err != nil {
		return Decision{}, fmt.Errorf("ratelimit redis eval: %w", err)
	}

	arr, ok := res.([]any)
	if !ok || len(arr) != 2 {
		return Decision{}, fmt.Errorf("ratelimit redis eval: unexpected result type")
	}

	count := int(arr[0].(int64))
	ttlGot := time.Duration(arr[1].(int64)) * time.Millisecond

	remaining := limit - count
	allowed := count <= limit

	d := Decision{
		Allowed:    allowed,
		Limit:      limit,
		Remaining:  maxInt(0, remaining),
		Count:      count,
		RetryAfter: 0,
		ResetAt:    time.Now().Add(ttlGot),
	}

	if !allowed {
		if ttlGot > 0 {
			d.RetryAfter = ttlGot
		} else {
			d.RetryAfter = window
		}
	}

	return d, nil
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
