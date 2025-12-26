package config

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestLoad(t *testing.T) {
	// Helper: Ensure environment is clean for each subtest
	cleanup := func() {
		os.Unsetenv("APP_ENV")
		os.Unsetenv("HTTP_ADDR")
		os.Unsetenv("DATABASE_URL")
		os.Unsetenv("JWT_SECRET")
		os.Unsetenv("JWT_ISSUER")
		os.Unsetenv("RABBIT_URL")
		os.Unsetenv("RABBIT_EXCHANGE")
	}

	t.Run("should_return_error_if_database_url_is_missing", func(t *testing.T) {
		cleanup()
		cfg, err := Load()
		assert.Nil(t, cfg)
		assert.Error(t, err)
		// Match the exact string in your current config.go
		assert.Contains(t, err.Error(), "missing DATABASE_URL")
	})

	t.Run("should_return_error_if_jwt_secret_is_missing", func(t *testing.T) {
		cleanup()
		os.Setenv("DATABASE_URL", "postgres://localhost:5432/db")
		defer cleanup()

		cfg, err := Load()
		assert.Nil(t, cfg)
		assert.Error(t, err)
		// Match the exact string in your current config.go
		assert.Contains(t, err.Error(), "missing JWT_SECRET")
	})

	t.Run("should_load_successfully_with_valid_env", func(t *testing.T) {
		cleanup()
		os.Setenv("DATABASE_URL", "postgres://localhost:5432/db")
		os.Setenv("JWT_SECRET", "super-secret")
		os.Setenv("HTTP_ADDR", ":9090")
		defer cleanup()

		cfg, err := Load()
		assert.NoError(t, err)
		assert.NotNil(t, cfg)
		assert.Equal(t, ":9090", cfg.HTTPAddr)
		assert.Equal(t, "super-secret", cfg.JWTSecret)
		// Your new config.go defaults JWTIssuer to empty string ""
		assert.Equal(t, "", cfg.JWTIssuer)
		// Verify RabbitMQ default exchange
		assert.Equal(t, "city.events", cfg.RabbitExchange)
	})

	t.Run("should_fail_in_non_dev_env_if_rabbit_url_missing", func(t *testing.T) {
		cleanup()
		os.Setenv("APP_ENV", "prod")
		os.Setenv("DATABASE_URL", "postgres://localhost")
		os.Setenv("JWT_SECRET", "secret")
		// Purposefully not setting RABBIT_URL
		defer cleanup()

		cfg, err := Load()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "missing RABBIT_URL (required when APP_ENV != dev)")
		assert.Nil(t, cfg)
	})
}

func TestGetEnv(t *testing.T) {
	t.Run("should_trim_whitespace", func(t *testing.T) {
		os.Setenv("TEST_KEY", "  value_with_spaces  ")
		defer os.Unsetenv("TEST_KEY")

		result := getEnv("TEST_KEY", "default")
		assert.Equal(t, "value_with_spaces", result)
	})

	t.Run("should_return_default_if_empty", func(t *testing.T) {
		os.Setenv("TEST_KEY", "")
		defer os.Unsetenv("TEST_KEY")

		result := getEnv("TEST_KEY", "fallback")
		assert.Equal(t, "fallback", result)
	})
}

func TestGetDuration(t *testing.T) {
	t.Run("should_parse_valid_duration", func(t *testing.T) {
		os.Setenv("DUR_KEY", "5s")
		defer os.Unsetenv("DUR_KEY")

		d := getDuration("DUR_KEY", 0)
		assert.Equal(t, 5*time.Second, d)
	})

	t.Run("should_return_default_on_invalid_duration", func(t *testing.T) {
		os.Setenv("DUR_KEY", "invalid")
		defer os.Unsetenv("DUR_KEY")

		d := getDuration("DUR_KEY", 10*time.Second)
		assert.Equal(t, 10*time.Second, d)
	})
}
