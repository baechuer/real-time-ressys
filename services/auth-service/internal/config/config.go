package config

import (
	"fmt"
	"os"
	"strings"
	"time"
)

type Config struct {
	//App
	Env string // dev / staging / prod
	//HTTP
	HTTPAddr string
	//Auth / Security
	JWTSecret       string
	AccessTokenTTL  time.Duration
	RefreshTokenTTL time.Duration

	// Infrastructure
	DBAddr    string
	RedisAddr string
	RabbitURL string

	HTTPReadTimeout  time.Duration
	HTTPWriteTimeout time.Duration
	HTTPIdleTimeout  time.Duration

	// One-time token flows (email verify / password reset)
	VerifyEmailBaseURL    string
	PasswordResetBaseURL  string
	VerifyEmailTokenTTL   time.Duration
	PasswordResetTokenTTL time.Duration
}

func Load() (*Config, error) {
	cfg := &Config{
		Env:      getEnv("ENV", "dev"),
		HTTPAddr: getEnv("HTTP_ADDR", ":8080"),
	}
	// required values
	cfg.JWTSecret = os.Getenv("JWT_SECRET")
	if cfg.JWTSecret == "" {
		return nil, fmt.Errorf("missing required env var: JWT_SECRET")
	}

	// optional with defaults
	// However, if the environmental variable is not set, we'll use a default
	ttl, err := getDuration("ACCESS_TOKEN_TTL", 15*time.Minute)
	if err != nil {
		return nil, err
	}
	cfg.AccessTokenTTL = ttl

	rtl, err := getDuration("REFRESH_TOKEN_TTL", 7*24*time.Hour)
	if err != nil {
		return nil, err
	}
	cfg.RefreshTokenTTL = rtl
	// One-time token URLs (sent via email-service)
	// Must include `token=` because service appends the token.
	cfg.VerifyEmailBaseURL = os.Getenv("VERIFY_EMAIL_BASE_URL")
	if cfg.VerifyEmailBaseURL == "" {
		return nil, fmt.Errorf("missing required env var: VERIFY_EMAIL_BASE_URL")
	}
	if !strings.Contains(cfg.VerifyEmailBaseURL, "token=") {
		return nil, fmt.Errorf("VERIFY_EMAIL_BASE_URL must contain `token=`")
	}

	cfg.PasswordResetBaseURL = os.Getenv("PASSWORD_RESET_BASE_URL")
	if cfg.PasswordResetBaseURL == "" {
		return nil, fmt.Errorf("missing required env var: PASSWORD_RESET_BASE_URL")
	}
	if !strings.Contains(cfg.PasswordResetBaseURL, "token=") {
		return nil, fmt.Errorf("PASSWORD_RESET_BASE_URL must contain `token=`")
	}

	// One-time token TTLs
	vet, err := getDuration("VERIFY_EMAIL_TOKEN_TTL", 24*time.Hour)
	if err != nil {
		return nil, err
	}
	cfg.VerifyEmailTokenTTL = vet

	prt, err := getDuration("PASSWORD_RESET_TOKEN_TTL", 30*time.Minute)
	if err != nil {
		return nil, err
	}
	cfg.PasswordResetTokenTTL = prt

	// Infrastructure dependencies.
	// These values are required at startup because the auth-service
	// cannot operate correctly without its backing services.
	// Fail fast here to avoid starting in a broken or partially-initialized state.

	cfg.DBAddr = os.Getenv("DB_ADDR")
	if cfg.DBAddr == "" {
		return nil, fmt.Errorf("missing required env var: DB_ADDR")
	}

	cfg.RedisAddr = os.Getenv("REDIS_ADDR")
	if cfg.RedisAddr == "" {
		return nil, fmt.Errorf("missing required env var: REDIS_ADDR")
	}

	cfg.RabbitURL = os.Getenv("RABBIT_URL")
	if cfg.RabbitURL == "" {
		return nil, fmt.Errorf("missing required env var: RABBIT_URL")
	}

	//Timeout values are optional and have a default value if not
	rt, err := getDuration("HTTP_READ_TIMEOUT", 10*time.Second)
	if err != nil {
		return nil, err
	}
	cfg.HTTPReadTimeout = rt

	wt, err := getDuration("HTTP_WRITE_TIMEOUT", 30*time.Second)
	if err != nil {
		return nil, err
	}
	cfg.HTTPWriteTimeout = wt

	it, err := getDuration("HTTP_IDLE_TIMEOUT", time.Minute)
	if err != nil {
		return nil, err
	}
	cfg.HTTPIdleTimeout = it

	return cfg, nil
}

func getEnv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func getDuration(key string, def time.Duration) (time.Duration, error) {
	v := os.Getenv(key)
	if v == "" {
		return def, nil
	}

	d, err := time.ParseDuration(v)
	if err != nil {
		return 0, fmt.Errorf("invalid duration for %s: %q: %w", key, v, err)
	}
	return d, nil
}
