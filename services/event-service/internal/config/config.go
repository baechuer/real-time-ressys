package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	AppEnv string

	HTTPAddr    string
	DatabaseURL string

	JWTSecret string
	JWTIssuer string

	// RabbitMQ
	RabbitURL      string
	RabbitExchange string

	// Redis & Caching
	RedisURL        string
	CacheTTLDetails time.Duration // GetPublic
	CacheTTLList    time.Duration // ListPublic (First Page)

	// Rate Limiting
	RLEnabled bool
	RLLimit   int
	RLWindow  time.Duration

	LogLevel  string
	LogFormat string

	HTTPReadTimeout  time.Duration
	HTTPWriteTimeout time.Duration
	HTTPIdleTimeout  time.Duration
}

func Load() (*Config, error) {
	// 加载 .env 文件到环境变量中
	_ = godotenv.Load()

	cfg := &Config{}

	cfg.AppEnv = getEnv("APP_ENV", "dev")
	cfg.HTTPAddr = getEnv("HTTP_ADDR", ":8081")
	cfg.DatabaseURL = getEnv("DATABASE_URL", "")

	cfg.JWTSecret = getEnv("JWT_SECRET", "")
	cfg.JWTIssuer = getEnv("JWT_ISSUER", "")

	cfg.RabbitURL = getEnv("RABBIT_URL", "")
	cfg.RabbitExchange = getEnv("RABBIT_EXCHANGE", "city.events")

	cfg.RedisURL = getEnv("REDIS_URL", "redis://localhost:6379/0")
	cfg.CacheTTLDetails = getDuration("CACHE_TTL_DETAILS", 5*time.Minute)
	cfg.CacheTTLList = getDuration("CACHE_TTL_LIST", 15*time.Second)

	// Rate Limiting Defaults: 100 reqs / 1 min
	cfg.RLEnabled = getEnv("RL_ENABLED", "true") == "true"
	cfg.RLLimit = getIntEnv("RL_IP_LIMIT", 100)
	cfg.RLWindow = getDuration("RL_IP_WINDOW", 1*time.Minute)

	cfg.LogLevel = getEnv("LOG_LEVEL", "info")
	cfg.LogFormat = getEnv("LOG_FORMAT", "console")

	cfg.HTTPReadTimeout = getDuration("HTTP_READ_TIMEOUT", 10*time.Second)
	cfg.HTTPWriteTimeout = getDuration("HTTP_WRITE_TIMEOUT", 20*time.Second)
	cfg.HTTPIdleTimeout = getDuration("HTTP_IDLE_TIMEOUT", 60*time.Second)

	// validation
	if cfg.DatabaseURL == "" {
		return nil, fmt.Errorf("missing DATABASE_URL")
	}
	if cfg.JWTSecret == "" {
		return nil, fmt.Errorf("missing JWT_SECRET")
	}

	// Rabbit: dev 可空；非 dev 强制
	if cfg.AppEnv != "dev" && cfg.RabbitURL == "" {
		return nil, fmt.Errorf("missing RABBIT_URL (required when APP_ENV != dev)")
	}

	return cfg, nil
}

func getEnv(k, def string) string {
	if v := strings.TrimSpace(os.Getenv(k)); v != "" {
		return v
	}
	return def
}

func getDuration(key string, def time.Duration) time.Duration {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return def
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		return def
	}
	return d
}

func getIntEnv(key string, def int) int {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return def
	}
	i, err := strconv.Atoi(v)
	if err != nil {
		return def
	}
	return i
}
