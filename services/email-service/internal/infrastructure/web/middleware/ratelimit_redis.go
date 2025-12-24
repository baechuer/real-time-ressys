package middleware

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/gomodule/redigo/redis"
	"github.com/rs/zerolog"
)

type RedisRateLimitConfig struct {
	Enabled bool

	// Per-IP limit per endpoint
	IPLimit  int
	IPWindow time.Duration

	// Per-token limit per endpoint
	TokenLimit  int
	TokenWindow time.Duration

	// Prefix for keys
	KeyPrefix string // e.g. "rl:email"
}

type RedisRateLimiter struct {
	pool    *redis.Pool
	cfg     RedisRateLimitConfig
	lg      zerolog.Logger
	script  allowScript
	onError func(error)
}

func NewRedisRateLimiter(pool *redis.Pool, cfg RedisRateLimitConfig, lg zerolog.Logger) *RedisRateLimiter {
	if cfg.KeyPrefix == "" {
		cfg.KeyPrefix = "rl:email"
	}
	return &RedisRateLimiter{
		pool:   pool,
		cfg:    cfg,
		lg:     lg.With().Str("component", "rl_redis").Logger(),
		script: defaultAllowScript,
	}
}

// WrapJSONTokenEndpoint applies:
// 1) per-IP limit
// 2) per-token limit (token extracted from JSON body field "token")
func (rl *RedisRateLimiter) WrapJSONTokenEndpoint(endpoint string, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !rl.cfg.Enabled || rl.pool == nil {
			next(w, r)
			return
		}

		ip := clientIP(r)
		if ip == "" {
			ip = "unknown"
		}

		// Read body once, extract token, then restore body for downstream handler
		rawBody, token, err := readBodyAndExtractToken(r)
		if err != nil {
			http.Error(w, "bad json", http.StatusBadRequest)
			return
		}
		r.Body = io.NopCloser(bytes.NewReader(rawBody))

		// 1) Per-IP limit
		if rl.cfg.IPLimit > 0 && rl.cfg.IPWindow > 0 {
			key := fmt.Sprintf("%s:ip:%s:%s", rl.cfg.KeyPrefix, ip, endpoint)
			allowed, retryAfter, cur, e := rl.allow(r.Context(), key, rl.cfg.IPWindow, rl.cfg.IPLimit)
			if e != nil {
				rl.lg.Error().Err(e).Str("key", key).Msg("redis rl ip check failed")
				if rl.onError != nil {
					rl.onError(e)
				}
				http.Error(w, "rate limit backend error", http.StatusBadGateway)
				return
			}
			if !allowed {
				w.Header().Set("Retry-After", fmt.Sprintf("%d", int(retryAfter.Seconds())))
				http.Error(w, fmt.Sprintf("rate limited (ip). count=%d limit=%d", cur, rl.cfg.IPLimit), http.StatusTooManyRequests)
				return
			}
		}

		// 2) Per-token limit (only if token exists)
		if token != "" && rl.cfg.TokenLimit > 0 && rl.cfg.TokenWindow > 0 {
			key := fmt.Sprintf("%s:token:%s:%s", rl.cfg.KeyPrefix, token, endpoint)
			allowed, retryAfter, cur, e := rl.allow(r.Context(), key, rl.cfg.TokenWindow, rl.cfg.TokenLimit)
			if e != nil {
				rl.lg.Error().Err(e).Str("key", key).Msg("redis rl token check failed")
				if rl.onError != nil {
					rl.onError(e)
				}
				http.Error(w, "rate limit backend error", http.StatusBadGateway)
				return
			}
			if !allowed {
				w.Header().Set("Retry-After", fmt.Sprintf("%d", int(retryAfter.Seconds())))
				http.Error(w, fmt.Sprintf("rate limited (token). count=%d limit=%d", cur, rl.cfg.TokenLimit), http.StatusTooManyRequests)
				return
			}
		}

		// restore body again (some handlers read twice defensively)
		r.Body = io.NopCloser(bytes.NewReader(rawBody))
		next(w, r)
	}
}

var luaAllow = redis.NewScript(1, `
local current = redis.call("INCR", KEYS[1])
if current == 1 then
  redis.call("PEXPIRE", KEYS[1], ARGV[1])
end
local limit = tonumber(ARGV[2])
if current > limit then
  return {0, current}
end
return {1, current}
`)

// returns allowed, retryAfter (approx window), currentCount
func (rl *RedisRateLimiter) allow(ctx context.Context, key string, window time.Duration, limit int) (bool, time.Duration, int, error) {
	conn, err := rl.pool.GetContext(ctx)
	if err != nil {
		return false, 0, 0, err
	}
	defer conn.Close()

	// ARGV[1]=window_ms, ARGV[2]=limit
	res, err := rl.script.Do(conn, key, int64(window/time.Millisecond), limit)
	if err != nil {
		return false, 0, 0, err
	}
	var ok int
	var cur int
	if _, err := redis.Scan(res, &ok, &cur); err != nil {
		return false, 0, 0, err
	}

	if ok == 1 {
		return true, 0, cur, nil
	}
	// Not allowed; we can approximate Retry-After as window (simple + OK)
	return false, window, cur, nil
}

func readBodyAndExtractToken(r *http.Request) ([]byte, string, error) {
	if r.Body == nil {
		return []byte{}, "", nil
	}
	// prevent huge bodies
	const max = 64 * 1024
	b, err := io.ReadAll(io.LimitReader(r.Body, max))
	if err != nil {
		return nil, "", err
	}
	// empty body is allowed for some endpoints; treat as no token
	if len(bytes.TrimSpace(b)) == 0 {
		return b, "", nil
	}

	// parse minimal json to extract "token"
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		return nil, "", err
	}
	t, _ := m["token"].(string)
	t = strings.TrimSpace(t)
	return b, t, nil
}

func clientIP(r *http.Request) string {
	// if behind reverse proxy, prefer X-Forwarded-For
	xff := strings.TrimSpace(r.Header.Get("X-Forwarded-For"))
	if xff != "" {
		parts := strings.Split(xff, ",")
		if len(parts) > 0 {
			return strings.TrimSpace(parts[0])
		}
	}
	xri := strings.TrimSpace(r.Header.Get("X-Real-IP"))
	if xri != "" {
		return xri
	}
	host, _, err := net.SplitHostPort(strings.TrimSpace(r.RemoteAddr))
	if err == nil && host != "" {
		return host
	}
	// if already a bare IP
	if ip := strings.TrimSpace(r.RemoteAddr); ip != "" {
		return ip
	}
	return ""
}

type allowScript interface {
	Do(conn redis.Conn, key string, windowMS int64, limit int) ([]any, error)
}

type redigoAllowScript struct {
	script *redis.Script
}

func (s redigoAllowScript) Do(conn redis.Conn, key string, windowMS int64, limit int) ([]any, error) {
	return redis.Values(s.script.Do(conn, key, windowMS, limit))
}

var defaultAllowScript allowScript = redigoAllowScript{
	script: redis.NewScript(1, `
local current = redis.call("INCR", KEYS[1])
if current == 1 then
  redis.call("PEXPIRE", KEYS[1], ARGV[1])
end
local limit = tonumber(ARGV[2])
if current > limit then
  return {0, current}
end
return {1, current}
`),
}
