package config

import (
	"fmt"
	"os"
	"strings"

	"github.com/joho/godotenv"
)

type Config struct {
	HTTPAddr    string
	DatabaseURL string

	JWTSecret string
	JWTIssuer string
}

func Load() (*Config, error) {
	_ = godotenv.Load()

	cfg := &Config{}

	cfg.HTTPAddr = getEnv("HTTP_ADDR", ":8081")
	cfg.DatabaseURL = getEnv("DATABASE_URL", "")
	cfg.JWTSecret = getEnv("JWT_SECRET", "")
	cfg.JWTIssuer = getEnv("JWT_ISSUER", "cityevents-auth")

	if cfg.DatabaseURL == "" {
		return nil, fmt.Errorf("missing required env var: DATABASE_URL")
	}
	if cfg.JWTSecret == "" {
		return nil, fmt.Errorf("missing required env var: JWT_SECRET")
	}

	if os.Getenv("APP_ENV") == "prod" && cfg.JWTSecret == "default_secret" {
		return nil, fmt.Errorf("security risk: cannot use default JWT_SECRET in production")
	}

	return cfg, nil
}

func getEnv(k, def string) string {
	if v := strings.TrimSpace(os.Getenv(k)); v != "" {
		return v
	}
	return def
}
