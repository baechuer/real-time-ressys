package logger

import (
	"os"

	"github.com/rs/zerolog"
)

var Logger zerolog.Logger

// Init initializes the global logger
func Init() {
	level := os.Getenv("LOG_LEVEL")
	if level == "" {
		level = "info"
	}

	logLevel, err := zerolog.ParseLevel(level)
	if err != nil {
		logLevel = zerolog.InfoLevel
	}

	format := os.Getenv("LOG_FORMAT")
	if format == "console" {
		Logger = zerolog.New(os.Stdout).With().
			Timestamp().
			Logger().
			Level(logLevel)
	} else {
		Logger = zerolog.New(os.Stdout).With().
			Timestamp().
			Logger().
			Level(logLevel).
			Output(zerolog.ConsoleWriter{Out: os.Stderr})
	}
}

// WithRequestID adds request ID to logger
func WithRequestID(requestID string) zerolog.Logger {
	return Logger.With().Str("request_id", requestID).Logger()
}

// GetLoggerFromContext retrieves logger from context (with request ID if available)
func GetLoggerFromContext(ctx interface{}) zerolog.Logger {
	// For now, return global logger
	// Can be enhanced to extract from context if needed
	return Logger
}

