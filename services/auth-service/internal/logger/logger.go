package logger

import (
	"io"
	"os"
	"time"

	"github.com/rs/zerolog"
	zlog "github.com/rs/zerolog/log"
)

var Logger zerolog.Logger

func Init() {
	InitWithWriter(os.Stdout)
}

func InitWithWriter(w io.Writer) {
	logLevel := os.Getenv("LOG_LEVEL")
	if logLevel == "" {
		logLevel = "info"
	}
	level, err := zerolog.ParseLevel(logLevel)
	if err != nil {
		level = zerolog.InfoLevel
	}

	format := os.Getenv("LOG_FORMAT") // "json" or "console"
	if format == "" {
		format = "console"
	}

	if format == "json" {
		Logger = zerolog.New(w).With().Timestamp().Logger().Level(level)
	} else {
		Logger = zerolog.New(zerolog.ConsoleWriter{
			Out:        w,
			TimeFormat: time.RFC3339,
		}).With().Timestamp().Logger().Level(level)
	}

	// set global
	zlog.Logger = Logger
}
