//go:build integration

package bootstrap

import (
	"context"
	"errors"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/baechuer/real-time-ressys/services/auth-service/internal/config"
)

/*
⚠️ IMPORTANT (please read first):

The goal of this test file is NOT to “mock everything”.
Instead, it validates that:

- NewServer behaves correctly under critical failure / degradation paths
- No panics occur
- Resources are properly cleaned up
- Dev / prod behaviors match expectations

Therefore, we:
- Use the real NewServer()
- Trigger failures via environment variables and fake endpoints
*/

// --------------------------
// helpers
// --------------------------

func withEnv(t *testing.T, kv map[string]string) func() {
	t.Helper()

	old := make(map[string]string)
	for k := range kv {
		old[k] = os.Getenv(k)
		_ = os.Setenv(k, kv[k])
	}
	return func() {
		for k, v := range old {
			if v == "" {
				_ = os.Unsetenv(k)
			} else {
				_ = os.Setenv(k, v)
			}
		}
	}
}

// Minimal valid environment.
// DB / Redis / RabbitMQ will be overridden per test case.
func baseEnv(env string) map[string]string {
	return map[string]string{
		"ENV":        env,
		"HTTP_ADDR":  ":0",
		"JWT_SECRET": "test-secret",

		"ACCESS_TOKEN_TTL":         "15m",
		"REFRESH_TOKEN_TTL":        "24h",
		"VERIFY_EMAIL_TOKEN_TTL":   "1h",
		"PASSWORD_RESET_TOKEN_TTL": "1h",

		"VERIFY_EMAIL_BASE_URL":   "http://example.com/verify?token=",
		"PASSWORD_RESET_BASE_URL": "http://example.com/reset?token=",
	}
}

// --------------------------
// tests
// --------------------------

// 1️⃣ config.Load failure
func TestNewServer_ConfigLoadFails(t *testing.T) {
	restore := withEnv(t, map[string]string{})
	defer restore()

	// Deliberately omit required environment variables
	srv, cleanup, err := NewServer()

	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if srv != nil {
		t.Fatalf("expected server=nil")
	}
	if cleanup != nil {
		t.Fatalf("expected cleanup=nil")
	}
}

// 2️⃣ Database connection failure
func TestNewServer_DBConnectFails(t *testing.T) {
	restore := withEnv(t, func() map[string]string {
		env := baseEnv("dev")
		env["DB_ADDR"] = "postgres://invalid:5432/db"
		env["REDIS_ADDR"] = "localhost:6379"
		env["RABBIT_URL"] = "amqp://guest:guest@localhost:5672/"
		return env
	}())
	defer restore()

	srv, cleanup, err := NewServer()

	if err == nil {
		t.Fatalf("expected db connect error")
	}
	if srv != nil {
		t.Fatalf("expected server=nil")
	}
	if cleanup != nil {
		t.Fatalf("expected cleanup=nil")
	}
}

// 3️⃣ Redis unavailable → fallback to in-memory store (dev)
func TestNewServer_RedisUnavailable_FallbackMemory(t *testing.T) {
	restore := withEnv(t, func() map[string]string {
		env := baseEnv("dev")
		env["DB_ADDR"] = "postgres://user:pass@localhost:5432/postgres?sslmode=disable"
		env["REDIS_ADDR"] = "localhost:1" // invalid port
		env["RABBIT_URL"] = "amqp://guest:guest@localhost:5672/"
		return env
	}())
	defer restore()

	srv, cleanup, err := NewServer()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if srv == nil || cleanup == nil {
		t.Fatalf("expected server and cleanup")
	}

	// Ensure server is usable and shuts down cleanly
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	_ = srv.Shutdown(ctx)
	cleanup()
}

// 4️⃣ RabbitMQ unavailable
// dev → allow noop publisher
func TestNewServer_RabbitUnavailable_Dev_Allows(t *testing.T) {
	restore := withEnv(t, func() map[string]string {
		env := baseEnv("dev")
		env["DB_ADDR"] = "postgres://user:pass@localhost:5432/postgres?sslmode=disable"
		env["REDIS_ADDR"] = "localhost:6379"
		env["RABBIT_URL"] = "amqp://invalid"
		return env
	}())
	defer restore()

	srv, cleanup, err := NewServer()
	if err != nil {
		t.Fatalf("unexpected error in dev: %v", err)
	}
	if srv == nil || cleanup == nil {
		t.Fatalf("expected server and cleanup")
	}
	cleanup()
}

// prod → fail fast
func TestNewServer_RabbitUnavailable_Prod_Fails(t *testing.T) {
	restore := withEnv(t, func() map[string]string {
		env := baseEnv("prod")
		env["DB_ADDR"] = "postgres://user:pass@localhost:5432/postgres?sslmode=disable"
		env["REDIS_ADDR"] = "localhost:6379"
		env["RABBIT_URL"] = "amqp://invalid"
		return env
	}())
	defer restore()

	srv, cleanup, err := NewServer()
	if err == nil {
		t.Fatalf("expected error in prod when rabbit unavailable")
	}
	if srv != nil {
		t.Fatalf("expected server=nil")
	}
	if cleanup != nil {
		t.Fatalf("expected cleanup=nil")
	}
}

// 5️⃣ Cleanup must be idempotent (safe to call multiple times)
func TestNewServer_Cleanup_Idempotent(t *testing.T) {
	restore := withEnv(t, func() map[string]string {
		env := baseEnv("dev")
		env["DB_ADDR"] = "postgres://user:pass@localhost:5432/postgres?sslmode=disable"
		env["REDIS_ADDR"] = "localhost:6379"
		env["RABBIT_URL"] = "amqp://guest:guest@localhost:5672/"
		return env
	}())
	defer restore()

	srv, cleanup, err := NewServer()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Shutdown server first
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	_ = srv.Shutdown(ctx)

	// Cleanup should be safe to call multiple times
	cleanup()
	cleanup()
}

// --------------------------
// compile-time guards
// --------------------------

var _ = errors.New
var _ = http.ErrServerClosed
var _ = config.Config{}
