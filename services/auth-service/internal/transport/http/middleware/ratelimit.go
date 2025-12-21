package middleware

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/baechuer/real-time-ressys/services/auth-service/internal/domain"
	"github.com/baechuer/real-time-ressys/services/auth-service/internal/infrastructure/redis"
)

type RateLimiter interface {
	AllowFixedWindow(ctx context.Context, key string, limit int, window time.Duration) (redis.Decision, error)
}

// To avoid leaking redis.Decision into the middleware layer,
// we abstract the decision via a tiny interface.
type rateDecision interface {
	isAllowed() bool
	limit() int
	remaining() int
	retryAfter() time.Duration
}

// Adapter notes:
// You can provide a thin wrapper in wire.go, or let redis.Decision
// implement these methods directly.
// For simplicity, the middleware uses type assertions to extract fields.
// If you prefer, you can change RateLimiter to return
// (allowed bool, remaining int, retry time.Duration, err error).
func extractDecision(v any) (allowed bool, limit int, remaining int, retry time.Duration) {
	// Compatible with future implementations exposing a Get() method.
	type d1 interface {
		Get() (bool, int, int, time.Duration)
	}
	if x, ok := v.(d1); ok {
		return x.Get()
	}

	// Compatible with a redis.Decision-like struct (matching field names).
	type decisionLike struct {
		Allowed    bool
		Limit      int
		Remaining  int
		RetryAfter time.Duration
	}
	if x, ok := v.(decisionLike); ok {
		return x.Allowed, x.Limit, x.Remaining, x.RetryAfter
	}

	// Fallback: allow the request.
	return true, 0, 0, 0
}

// FixedWindowConfig defines the configuration for a fixed-window rate limit.
type FixedWindowConfig struct {
	RouteKey string
	Limit    int
	Window   time.Duration
	// Identity strategy: "user_or_ip" by default.
}

func RateLimitFixedWindow(limiter RateLimiter, cfg FixedWindowConfig, writeErr WriteErrFunc) func(http.Handler) http.Handler {

	if cfg.Window <= 0 {
		cfg.Window = time.Minute
	}
	if cfg.RouteKey == "" {
		cfg.RouteKey = "unknown"
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if limiter == nil {
				next.ServeHTTP(w, r)
				return
			}

			identity := userOrIP(r)
			bucket := windowBucket(time.Now(), cfg.Window)
			key := fmt.Sprintf("rl:%s:%s:%d", cfg.RouteKey, identity, bucket)

			dec, err := limiter.AllowFixedWindow(r.Context(), key, cfg.Limit, cfg.Window)
			if err != nil {
				// Rate limiter failure:
				// Prefer fail-open to preserve availability, but log a warning.
				// If you want fail-closed, replace this with writeErr(...) and return.
				next.ServeHTTP(w, r)
				return
			}

			allowed, limit, remaining, retry := extractDecision(dec)
			if !allowed {
				// You may add headers in the response layer; here we return a domain error.
				// Recommended: add domain.ErrRateLimited(retry) or ErrRateLimited().
				writeErr(w, r, domain.ErrRateLimited(cfg.RouteKey))
				// Optional: include Retry-After header.
				if retry > 0 {
					w.Header().Set("Retry-After", fmt.Sprintf("%d", int(retry.Seconds())))
				}
				_ = limit
				_ = remaining
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func windowBucket(now time.Time, window time.Duration) int64 {
	sec := int64(window.Seconds())
	if sec <= 0 {
		sec = 60
	}
	return now.Unix() / sec
}

// userOrIP prefers the JWT userID if present; otherwise falls back to client IP.
func userOrIP(r *http.Request) string {
	if uid, ok := UserIDFromContext(r.Context()); ok && strings.TrimSpace(uid) != "" {
		return "u:" + uid
	}
	return "ip:" + clientIP(r)
}

func clientIP(r *http.Request) string {
	// If behind a reverse proxy, trust X-Forwarded-For ONLY if you control the proxy.
	xff := strings.TrimSpace(r.Header.Get("X-Forwarded-For"))
	if xff != "" {
		parts := strings.Split(xff, ",")
		ip := strings.TrimSpace(parts[0])
		if ip != "" {
			return ip
		}
	}

	host, _, err := net.SplitHostPort(strings.TrimSpace(r.RemoteAddr))
	if err == nil && host != "" {
		return host
	}
	return strings.TrimSpace(r.RemoteAddr)
}
