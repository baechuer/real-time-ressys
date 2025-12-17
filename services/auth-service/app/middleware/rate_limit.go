package middleware

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

// RouteLimit defines a token bucket capacity and window for a route.
type RouteLimit struct {
	Name     string        // logical route name for the key
	Capacity int           // max tokens in the bucket
	Window   time.Duration // window over which capacity refills linearly
}

// PrincipalFunc extracts the rate-limit principal (e.g., user id or IP).
type PrincipalFunc func(*http.Request) string

// PrincipalIP extracts the client IP (best-effort).
func PrincipalIP() PrincipalFunc {
	return func(r *http.Request) string {
		if xf := r.Header.Get("X-Forwarded-For"); xf != "" {
			parts := strings.Split(xf, ",")
			if len(parts) > 0 {
				return "ip:" + strings.TrimSpace(parts[0])
			}
		}
		host, _, err := net.SplitHostPort(r.RemoteAddr)
		if err == nil && host != "" {
			return "ip:" + host
		}
		return "ip:unknown"
	}
}

// PrincipalUserOrIP prefers user id from context, falls back to IP.
func PrincipalUserOrIP() PrincipalFunc {
	return func(r *http.Request) string {
		if uid, ok := UserIDFromContext(r.Context()); ok {
			return fmt.Sprintf("user:%d", uid)
		}
		return PrincipalIP()(r)
	}
}

// RateLimit applies a Redis token-bucket limiter using a Lua script for atomicity.
func RateLimit(rdb *redis.Client, limit RouteLimit, principal PrincipalFunc) func(http.Handler) http.Handler {
	if principal == nil {
		principal = PrincipalIP()
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			now := time.Now().UnixMilli()
			key := fmt.Sprintf("rl:%s:%s", limit.Name, principal(r))

			allowed, tokens, retryAfter := takeToken(rdb, key, now, limit)
			if !allowed {
				if retryAfter > 0 {
					w.Header().Set("Retry-After", fmt.Sprintf("%d", retryAfter))
				}
				http.Error(w, "too many requests", http.StatusTooManyRequests)
				return
			}

			// Optional visibility header for debugging
			w.Header().Set("X-RateLimit-Remaining", fmt.Sprintf("%.0f", tokens))
			next.ServeHTTP(w, r)
		})
	}
}

// Lua script performs token-bucket operations atomically.
var rateLimitScript = redis.NewScript(`
local key = KEYS[1]
local now = tonumber(ARGV[1])
local capacity = tonumber(ARGV[2])
local window = tonumber(ARGV[3]) -- in ms

local bucket = redis.call("HMGET", key, "tokens", "ts")
local tokens = tonumber(bucket[1])
local ts = tonumber(bucket[2])

if tokens == nil or ts == nil then
  tokens = capacity
  ts = now
end

local delta = now - ts
if delta < 0 then delta = 0 end

local refill = (delta * capacity) / window
tokens = math.min(capacity, tokens + refill)
ts = now

local allowed = 0
if tokens >= 1 then
  tokens = tokens - 1
  allowed = 1
end

redis.call("HMSET", key, "tokens", tokens, "ts", ts)
redis.call("PEXPIRE", key, window)

local retryAfterMs = 0
if allowed == 0 then
  local need = 1 - tokens
  if need < 0 then need = 0 end
  retryAfterMs = math.ceil(need * window / capacity)
end

return {allowed, tokens, retryAfterMs}
`)

func takeToken(rdb *redis.Client, key string, nowMs int64, limit RouteLimit) (allowed bool, remaining float64, retryAfterSec int64) {
	res, err := rateLimitScript.Run(context.Background(), rdb, []string{key}, nowMs, limit.Capacity, limit.Window.Milliseconds()).Result()
	if err != nil {
		// Fail-open on Redis errors to avoid cascading auth issues; you can choose to fail-closed if desired.
		return true, float64(limit.Capacity), 0
	}
	arr, ok := res.([]interface{})
	if !ok || len(arr) != 3 {
		return true, float64(limit.Capacity), 0
	}
	allowed = arr[0].(int64) == 1
	remaining, _ = toFloat(arr[1])
	retryMs, _ := toFloat(arr[2])
	if retryMs > 0 {
		retryAfterSec = int64((retryMs + 999) / 1000) // ceil
	}
	return
}

func toFloat(v interface{}) (float64, bool) {
	switch t := v.(type) {
	case int64:
		return float64(t), true
	case float64:
		return t, true
	case string:
		f, err := strconv.ParseFloat(t, 64)
		if err == nil {
			return f, true
		}
	}
	return 0, false
}
