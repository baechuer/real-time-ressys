package config

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLoad(t *testing.T) {
	// 辅助函数：清理测试相关的环境变量
	cleanup := func() {
		os.Unsetenv("HTTP_ADDR")
		os.Unsetenv("DATABASE_URL")
		os.Unsetenv("JWT_SECRET")
		os.Unsetenv("JWTIssuer")
		os.Unsetenv("APP_ENV")
	}

	t.Run("should_return_error_if_database_url_is_missing", func(t *testing.T) {
		cleanup()
		// 即使不设置任何东西，也要确保必填项报错
		cfg, err := Load()
		assert.Nil(t, cfg)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "missing required env var: DATABASE_URL")
	})

	t.Run("should_return_error_if_jwt_secret_is_missing", func(t *testing.T) {
		cleanup()
		os.Setenv("DATABASE_URL", "postgres://localhost:5432/db")
		defer cleanup()

		cfg, err := Load()
		assert.Nil(t, cfg)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "missing required env var: JWT_SECRET")
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
		assert.Equal(t, "cityevents-auth", cfg.JWTIssuer) // 验证默认值
	})

	t.Run("should_fail_in_prod_with_default_secret", func(t *testing.T) {
		cleanup()
		os.Setenv("APP_ENV", "prod")
		os.Setenv("DATABASE_URL", "postgres://localhost")
		os.Setenv("JWT_SECRET", "default_secret")
		defer cleanup()

		cfg, err := Load()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "security risk")
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
