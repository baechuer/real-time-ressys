package config

import (
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	AppEnv string
	Port   int

	// Postgres (pgxpool DSN)
	DBDSN string

	// JWT verification (must match auth-service signing config)
	JWTSecret string
	JWTIssuer string

	// Redis
	RedisAddr string
	RedisPass string
	RedisDB   int

	// Cache
	CacheUserTTL time.Duration

	// Rate limit
	RLEnabled bool
	RLLimit   int
	RLWindow  time.Duration

	// RabbitMQ
	RabbitURL      string
	RabbitExchange string

	// Logging
	LogLevel string

	// Optional toggles
	OutboxEnabled bool
}

func Load() (*Config, error) {
	_ = godotenv.Load()

	cfg := &Config{}
	cfg.AppEnv = getEnv("APP_ENV", "dev")
	cfg.Port = getInt("PORT", 8080)

	// --- Postgres: prefer DATABASE_URL if present, else build from POSTGRES_*
	dbURL := strings.TrimSpace(os.Getenv("DATABASE_URL"))
	if dbURL != "" {
		cfg.DBDSN = dbURL
	} else {
		addr := getEnv("POSTGRES_ADDR", "")
		user := getEnv("POSTGRES_USER", "")
		pass := getEnv("POSTGRES_PASSWORD", "")
		db := getEnv("POSTGRES_DB", "")
		sslmode := getEnv("POSTGRES_SSLMODE", "disable")
		cfg.DBDSN = buildPostgresURL(addr, user, pass, db, sslmode)
	}

	// --- JWT
	cfg.JWTSecret = getEnv("JWT_SECRET", "")
	cfg.JWTIssuer = getEnv("JWT_ISSUER", "")

	// --- Redis
	cfg.RedisAddr = getEnv("REDIS_ADDR", "127.0.0.1:6379")
	cfg.RedisPass = getEnv("REDIS_PASSWORD", "")
	cfg.RedisDB = getInt("REDIS_DB", 0)

	// --- Cache
	cfg.CacheUserTTL = getDuration("CACHE_USER_TTL", 10*time.Minute)

	// --- Rate limit (your .env uses RL_REQUESTS_LIMIT + RL_WINDOW_SECONDS)
	cfg.RLEnabled = getBool("RL_ENABLED", true)
	cfg.RLLimit = getInt("RL_REQUESTS_LIMIT", 100)
	cfg.RLWindow = time.Duration(getInt("RL_WINDOW_SECONDS", 60)) * time.Second

	// --- RabbitMQ (your .env uses RABBITMQ_URL/RABBITMQ_EXCHANGE)
	// Also accept RABBIT_URL / RABBIT_EXCHANGE for compatibility with other services.
	cfg.RabbitURL = firstNonEmpty(
		strings.TrimSpace(os.Getenv("RABBITMQ_URL")),
		strings.TrimSpace(os.Getenv("RABBIT_URL")),
		"amqp://guest:guest@localhost:5672/",
	)
	cfg.RabbitExchange = firstNonEmpty(
		strings.TrimSpace(os.Getenv("RABBITMQ_EXCHANGE")),
		strings.TrimSpace(os.Getenv("RABBIT_EXCHANGE")),
		"city.events",
	)

	// --- Logging
	cfg.LogLevel = getEnv("LOG_LEVEL", "info")

	// --- Optional toggles
	cfg.OutboxEnabled = getBool("OUTBOX_ENABLED", true)

	// --- Validation (fail fast, no more “Administrator fallback”)
	if cfg.DBDSN == "" {
		return nil, fmt.Errorf("missing database config: provide DATABASE_URL or POSTGRES_ADDR/POSTGRES_USER/POSTGRES_PASSWORD/POSTGRES_DB")
	}
	if cfg.JWTSecret == "" {
		return nil, fmt.Errorf("missing JWT_SECRET")
	}
	// Rabbit: dev can be empty; non-dev require (align with your event-service policy)
	if cfg.AppEnv != "dev" && cfg.RabbitURL == "" {
		return nil, fmt.Errorf("missing RABBITMQ_URL (required when APP_ENV != dev)")
	}

	return cfg, nil
}

// buildPostgresURL builds a safe postgres URL DSN (handles special characters).
func buildPostgresURL(addr, user, pass, db, sslmode string) string {
	// If any critical fields missing, return empty and let validation handle it.
	if strings.TrimSpace(addr) == "" || strings.TrimSpace(user) == "" || strings.TrimSpace(db) == "" {
		return ""
	}

	u := &url.URL{
		Scheme: "postgres",
		Host:   strings.TrimSpace(addr),
		Path:   "/" + strings.TrimPrefix(strings.TrimSpace(db), "/"),
	}
	if pass != "" {
		u.User = url.UserPassword(user, pass)
	} else {
		u.User = url.User(user)
	}

	q := url.Values{}
	if strings.TrimSpace(sslmode) != "" {
		q.Set("sslmode", strings.TrimSpace(sslmode))
	}
	u.RawQuery = q.Encode()
	return u.String()
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

func getEnv(k, def string) string {
	if v := strings.TrimSpace(os.Getenv(k)); v != "" {
		return v
	}
	return def
}

func getInt(k string, def int) int {
	v := strings.TrimSpace(os.Getenv(k))
	if v == "" {
		return def
	}
	i, err := strconv.Atoi(v)
	if err != nil {
		return def
	}
	return i
}

func getBool(k string, def bool) bool {
	v := strings.TrimSpace(os.Getenv(k))
	if v == "" {
		return def
	}
	switch strings.ToLower(v) {
	case "1", "true", "t", "yes", "y", "on":
		return true
	case "0", "false", "f", "no", "n", "off":
		return false
	default:
		// prefer failing fast over silent misconfig
		panic(fmt.Errorf("invalid boolean env %s=%q", k, v))
	}
}

func getDuration(k string, def time.Duration) time.Duration {
	v := strings.TrimSpace(os.Getenv(k))
	if v == "" {
		return def
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		return def
	}
	return d
}
