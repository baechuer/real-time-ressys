package config

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestLoad(t *testing.T) {
	cleanup := func() {
		os.Unsetenv("APP_ENV")
		os.Unsetenv("HTTP_ADDR")
		os.Unsetenv("DATABASE_URL")
		os.Unsetenv("JWT_SECRET")
		os.Unsetenv("JWT_ISSUER")
		os.Unsetenv("RABBIT_URL")
		os.Unsetenv("HTTP_READ_TIMEOUT")
	}

	t.Run("should_return_error_if_database_url_is_missing", func(t *testing.T) {
		cleanup()
		cfg, err := Load()
		assert.Nil(t, cfg)
		assert.Error(t, err)
		assert.Equal(t, "missing DATABASE_URL", err.Error())
	})

	t.Run("should_return_error_if_jwt_secret_is_missing", func(t *testing.T) {
		cleanup()
		os.Setenv("DATABASE_URL", "postgres://localhost:5432/db")
		cfg, err := Load()
		assert.Nil(t, cfg)
		assert.Error(t, err)
		assert.Equal(t, "missing JWT_SECRET", err.Error())
	})

	t.Run("should_load_successfully_with_valid_env", func(t *testing.T) {
		cleanup()
		os.Setenv("DATABASE_URL", "postgres://localhost:5432/db")
		os.Setenv("JWT_SECRET", "super-secret")
		os.Setenv("APP_ENV", "dev")

		cfg, err := Load()
		assert.NoError(t, err)
		assert.NotNil(t, cfg)
		assert.Equal(t, "dev", cfg.AppEnv)
		assert.Equal(t, "city.events", cfg.RabbitExchange) // 验证默认值
	})

	t.Run("should_fail_in_prod_if_rabbit_url_is_missing", func(t *testing.T) {
		cleanup()
		os.Setenv("APP_ENV", "prod")
		os.Setenv("DATABASE_URL", "postgres://localhost")
		os.Setenv("JWT_SECRET", "secret")

		cfg, err := Load()
		assert.Nil(t, cfg)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "missing RABBIT_URL")
	})
}

func TestGetEnv(t *testing.T) {
	t.Run("should_trim_whitespace", func(t *testing.T) {
		os.Setenv("TEST_KEY", "  value_with_spaces  ")
		defer os.Unsetenv("TEST_KEY")

		result := getEnv("TEST_KEY", "default")
		assert.Equal(t, "value_with_spaces", result)
	})
}

func TestGetDuration(t *testing.T) {
	t.Run("should_parse_valid_duration", func(t *testing.T) {
		os.Setenv("TEST_DUR", "5s")
		defer os.Unsetenv("TEST_DUR")

		result := getDuration("TEST_DUR", 10*time.Second)
		assert.Equal(t, 5*time.Second, result)
	})

	t.Run("should_return_default_on_invalid_duration", func(t *testing.T) {
		os.Setenv("TEST_DUR", "invalid")
		defer os.Unsetenv("TEST_DUR")

		result := getDuration("TEST_DUR", 10*time.Second)
		assert.Equal(t, 10*time.Second, result)
	})
}
