package config

import (
	"log"
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

// Load loads environment variables from .env file
// It's safe to call multiple times - it won't error if .env doesn't exist
func Load() {
	err := godotenv.Load()
	if err != nil {
		// .env file is optional, so we just log a warning
		log.Println("Warning: .env file not found, using system environment variables")
	}
}

func GetString(key, fallback string) string {

	val, ok := os.LookupEnv(key)
	if !ok {
		return fallback
	}
	return val
}

func GetInt(key string, fallback int) int {
	val, ok := os.LookupEnv(key)
	if !ok {
		return fallback
	}
	valInt, err := strconv.Atoi(val)
	if err != nil {
		return fallback
	}
	return valInt
}
