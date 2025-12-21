package config

import (
	"os"
	"testing"
	"time"
)

// helper: set env and auto-restore after test
func setEnv(t *testing.T, key, value string) {
	t.Helper()

	old, existed := os.LookupEnv(key)
	if err := os.Setenv(key, value); err != nil {
		t.Fatalf("Setenv %s: %v", key, err)
	}

	t.Cleanup(func() {
		if existed {
			_ = os.Setenv(key, old)
		} else {
			_ = os.Unsetenv(key)
		}
	})
}

func unsetEnv(t *testing.T, key string) {
	t.Helper()

	old, existed := os.LookupEnv(key)
	_ = os.Unsetenv(key)

	t.Cleanup(func() {
		if existed {
			_ = os.Setenv(key, old)
		}
	})
}

func setRequiredEnv(t *testing.T) {
	t.Helper()
	setEnv(t, "JWT_SECRET", "secret")
	setEnv(t, "DB_ADDR", "postgres://localhost:5432/db")
	setEnv(t, "REDIS_ADDR", "localhost:6379")
	setEnv(t, "RABBIT_URL", "amqp://guest:guest@localhost:5672/")
}

func TestLoad_MissingJWTSecret_ReturnsError(t *testing.T) {
	// Make sure required env are unset
	unsetEnv(t, "JWT_SECRET")
	setEnv(t, "DB_ADDR", "x")
	setEnv(t, "REDIS_ADDR", "x")
	setEnv(t, "RABBIT_URL", "x")

	_, err := Load()
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if err.Error() != "missing required env var: JWT_SECRET" {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoad_MissingDBAddr_ReturnsError(t *testing.T) {
	unsetEnv(t, "DB_ADDR")
	setEnv(t, "JWT_SECRET", "secret")
	setEnv(t, "REDIS_ADDR", "x")
	setEnv(t, "RABBIT_URL", "x")

	_, err := Load()
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if err.Error() != "missing required env var: DB_ADDR" {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoad_MissingRedisAddr_ReturnsError(t *testing.T) {
	unsetEnv(t, "REDIS_ADDR")
	setEnv(t, "JWT_SECRET", "secret")
	setEnv(t, "DB_ADDR", "x")
	setEnv(t, "RABBIT_URL", "x")

	_, err := Load()
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if err.Error() != "missing required env var: REDIS_ADDR" {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoad_MissingRabbitURL_ReturnsError(t *testing.T) {
	unsetEnv(t, "RABBIT_URL")
	setEnv(t, "JWT_SECRET", "secret")
	setEnv(t, "DB_ADDR", "x")
	setEnv(t, "REDIS_ADDR", "x")

	_, err := Load()
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if err.Error() != "missing required env var: RABBIT_URL" {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoad_Defaults_WhenOptionalUnset(t *testing.T) {
	setRequiredEnv(t)

	// Ensure optional env are unset so defaults should apply
	unsetEnv(t, "ENV")
	unsetEnv(t, "HTTP_ADDR")
	unsetEnv(t, "ACCESS_TOKEN_TTL")
	unsetEnv(t, "REFRESH_TOKEN_TTL")
	unsetEnv(t, "HTTP_READ_TIMEOUT")
	unsetEnv(t, "HTTP_WRITE_TIMEOUT")
	unsetEnv(t, "HTTP_IDLE_TIMEOUT")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}

	// defaults from Load()
	if cfg.Env != "dev" {
		t.Fatalf("Env default mismatch: got %q want %q", cfg.Env, "dev")
	}
	if cfg.HTTPAddr != ":8080" {
		t.Fatalf("HTTPAddr default mismatch: got %q want %q", cfg.HTTPAddr, ":8080")
	}
	if cfg.AccessTokenTTL != 15*time.Minute {
		t.Fatalf("AccessTokenTTL default mismatch: got %v want %v", cfg.AccessTokenTTL, 15*time.Minute)
	}
	if cfg.RefreshTokenTTL != 7*24*time.Hour {
		t.Fatalf("RefreshTokenTTL default mismatch: got %v want %v", cfg.RefreshTokenTTL, 7*24*time.Hour)
	}
	if cfg.HTTPReadTimeout != 10*time.Second {
		t.Fatalf("HTTPReadTimeout default mismatch: got %v want %v", cfg.HTTPReadTimeout, 10*time.Second)
	}
	if cfg.HTTPWriteTimeout != 30*time.Second {
		t.Fatalf("HTTPWriteTimeout default mismatch: got %v want %v", cfg.HTTPWriteTimeout, 30*time.Second)
	}
	if cfg.HTTPIdleTimeout != time.Minute {
		t.Fatalf("HTTPIdleTimeout default mismatch: got %v want %v", cfg.HTTPIdleTimeout, time.Minute)
	}
}

func TestLoad_OverridesOptionalValues_FromEnv(t *testing.T) {
	setRequiredEnv(t)

	// override optionals
	setEnv(t, "ENV", "prod")
	setEnv(t, "HTTP_ADDR", ":9999")
	setEnv(t, "ACCESS_TOKEN_TTL", "1h")
	setEnv(t, "REFRESH_TOKEN_TTL", "48h")
	setEnv(t, "HTTP_READ_TIMEOUT", "2s")
	setEnv(t, "HTTP_WRITE_TIMEOUT", "3s")
	setEnv(t, "HTTP_IDLE_TIMEOUT", "4s")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}

	if cfg.Env != "prod" {
		t.Fatalf("Env override mismatch: got %q want %q", cfg.Env, "prod")
	}
	if cfg.HTTPAddr != ":9999" {
		t.Fatalf("HTTPAddr override mismatch: got %q want %q", cfg.HTTPAddr, ":9999")
	}
	if cfg.AccessTokenTTL != time.Hour {
		t.Fatalf("AccessTokenTTL override mismatch: got %v want %v", cfg.AccessTokenTTL, time.Hour)
	}
	if cfg.RefreshTokenTTL != 48*time.Hour {
		t.Fatalf("RefreshTokenTTL override mismatch: got %v want %v", cfg.RefreshTokenTTL, 48*time.Hour)
	}
	if cfg.HTTPReadTimeout != 2*time.Second {
		t.Fatalf("HTTPReadTimeout override mismatch: got %v want %v", cfg.HTTPReadTimeout, 2*time.Second)
	}
	if cfg.HTTPWriteTimeout != 3*time.Second {
		t.Fatalf("HTTPWriteTimeout override mismatch: got %v want %v", cfg.HTTPWriteTimeout, 3*time.Second)
	}
	if cfg.HTTPIdleTimeout != 4*time.Second {
		t.Fatalf("HTTPIdleTimeout override mismatch: got %v want %v", cfg.HTTPIdleTimeout, 4*time.Second)
	}
}

func TestLoad_InvalidDuration_ReturnsError(t *testing.T) {
	setRequiredEnv(t)

	// make one duration invalid
	setEnv(t, "ACCESS_TOKEN_TTL", "not-a-duration")

	_, err := Load()
	if err == nil {
		t.Fatalf("expected error, got nil")
	}

	// Your getDuration wraps error with a stable prefix; test prefix instead of full string if you want more robustness.
	wantPrefix := `invalid duration for ACCESS_TOKEN_TTL: "not-a-duration":`
	if len(err.Error()) < len(wantPrefix) || err.Error()[:len(wantPrefix)] != wantPrefix {
		t.Fatalf("unexpected error: %v", err)
	}
}
