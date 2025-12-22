package config

import (
	"os"
	"testing"
	"time"
)

func setEnv(t *testing.T, k, v string) {
	t.Helper()
	old, ok := os.LookupEnv(k)
	os.Setenv(k, v)
	t.Cleanup(func() {
		if ok {
			os.Setenv(k, old)
		} else {
			os.Unsetenv(k)
		}
	})
}

func baseRequiredEnv(t *testing.T) {
	t.Helper()
	setEnv(t, "JWT_SECRET", "secret")
	setEnv(t, "DB_ADDR", "postgres://user:pass@localhost:5432/app")
	setEnv(t, "REDIS_ADDR", "localhost:6379")
	setEnv(t, "RABBIT_URL", "amqp://guest:guest@localhost:5672/")
	setEnv(t, "VERIFY_EMAIL_BASE_URL", "https://x/verify?token=")
	setEnv(t, "PASSWORD_RESET_BASE_URL", "https://x/reset?token=")
}

func TestLoad_MissingJWTSecret(t *testing.T) {
	baseRequiredEnv(t)
	os.Unsetenv("JWT_SECRET")

	_, err := Load()
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestLoad_InvalidVerifyEmailURL(t *testing.T) {
	baseRequiredEnv(t)
	setEnv(t, "VERIFY_EMAIL_BASE_URL", "https://x/verify")

	_, err := Load()
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestLoad_InvalidPasswordResetURL(t *testing.T) {
	baseRequiredEnv(t)
	setEnv(t, "PASSWORD_RESET_BASE_URL", "https://x/reset")

	_, err := Load()
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestLoad_InvalidDBAddr(t *testing.T) {
	baseRequiredEnv(t)
	setEnv(t, "DB_ADDR", "mysql://localhost/db")

	_, err := Load()
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestLoad_DurationsParsed(t *testing.T) {
	baseRequiredEnv(t)
	setEnv(t, "ACCESS_TOKEN_TTL", "1h")
	setEnv(t, "REFRESH_TOKEN_TTL", "48h")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.AccessTokenTTL != time.Hour {
		t.Fatalf("unexpected access ttl: %v", cfg.AccessTokenTTL)
	}
	if cfg.RefreshTokenTTL != 48*time.Hour {
		t.Fatalf("unexpected refresh ttl: %v", cfg.RefreshTokenTTL)
	}
}

func TestLoad_DefaultsApplied(t *testing.T) {
	baseRequiredEnv(t)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.HTTPAddr != ":8080" {
		t.Fatalf("unexpected http addr: %q", cfg.HTTPAddr)
	}
	if cfg.RedisDB != 0 {
		t.Fatalf("unexpected redis db: %d", cfg.RedisDB)
	}
}

func TestValidatePostgresDSN(t *testing.T) {
	cases := []struct {
		dsn string
		ok  bool
	}{
		{"postgres://user:pass@localhost:5432/app", true},
		{"postgresql://localhost/app", true},
		{"mysql://localhost/app", false},
		{"postgres://localhost", false},
	}

	for _, c := range cases {
		err := validatePostgresDSN(c.dsn)
		if c.ok && err != nil {
			t.Fatalf("expected ok for %q", c.dsn)
		}
		if !c.ok && err == nil {
			t.Fatalf("expected error for %q", c.dsn)
		}
	}
}
