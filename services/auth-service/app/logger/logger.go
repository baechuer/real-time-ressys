package logger

import (
	"os"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

var Logger zerolog.Logger

func Init() {
	// Set log level from environment
	logLevel := os.Getenv("LOG_LEVEL")
	if logLevel == "" {
		logLevel = "info"
	}

	level, err := zerolog.ParseLevel(logLevel)
	if err != nil {
		level = zerolog.InfoLevel
	}

	// Configure output format
	output := os.Getenv("LOG_FORMAT")
	if output == "json" {
		// JSON format for production
		Logger = zerolog.New(os.Stdout).With().
			Timestamp().
			Logger().
			Level(level)
	} else {
		// Pretty console format for development
		Logger = zerolog.New(zerolog.ConsoleWriter{
			Out:        os.Stdout,
			TimeFormat: time.RFC3339,
		}).With().
			Timestamp().
			Logger().
			Level(level)
	}

	// Set as global logger
	log.Logger = Logger
}

// WithRequestID adds request ID to logger context
func WithRequestID(requestID string) zerolog.Logger {
	return Logger.With().Str("request_id", requestID).Logger()
}

// WithUserID adds user ID to logger context
func WithUserID(userID int64) zerolog.Logger {
	return Logger.With().Int64("user_id", userID).Logger()
}

