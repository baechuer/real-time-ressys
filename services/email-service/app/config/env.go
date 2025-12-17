package config

import (
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

// Load loads environment variables from .env file
func Load() error {
	// Try to load .env file, but don't fail if it doesn't exist
	_ = godotenv.Load()
	return nil
}

// GetString returns environment variable as string with default
func GetString(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// GetInt returns environment variable as int with default
func GetInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}

// GetBool returns environment variable as bool
func GetBool(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		if boolValue, err := strconv.ParseBool(value); err == nil {
			return boolValue
		}
	}
	return defaultValue
}

