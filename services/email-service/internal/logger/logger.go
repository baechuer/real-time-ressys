package logger

import (
	"io"
	"os"
	"strings"
	"time"

	"github.com/rs/zerolog"
	zlog "github.com/rs/zerolog/log"
)

var Logger zerolog.Logger

func Init() {
	InitWithWriter(os.Stdout)
}

func InitWithWriter(w io.Writer) {
	// ---- level ----
	logLevel := strings.TrimSpace(os.Getenv("LOG_LEVEL"))
	if logLevel == "" {
		logLevel = "info"
	}
	level, err := zerolog.ParseLevel(logLevel)
	if err != nil {
		level = zerolog.InfoLevel
	}

	// ---- format ----
	format := strings.TrimSpace(os.Getenv("LOG_FORMAT")) // "json" or "console"
	if format == "" {
		format = "console"
	}

	// ---- time format ----
	timeFormat := strings.TrimSpace(os.Getenv("LOG_TIME_FORMAT"))
	if timeFormat == "" {
		// auth-service RFC3339
		timeFormat = time.RFC3339
	}

	// ---- base ----
	var base zerolog.Logger
	if format == "json" {
		base = zerolog.New(w)
	} else {
		// console writer (human readable)
		cw := zerolog.ConsoleWriter{
			Out:        w,
			TimeFormat: timeFormat,
		}
		if strings.TrimSpace(os.Getenv("LOG_COLOR")) == "0" {
			cw.NoColor = true
		}
		base = zerolog.New(cw)
	}

	// ---- enrich ----
	l := base.With().Timestamp().Logger().Level(level)

	// Optional: show caller like auth-service (if you enabled there)
	if strings.TrimSpace(os.Getenv("LOG_CALLER")) == "1" {
		l = l.With().Caller().Logger()
	}

	Logger = l
	zlog.Logger = Logger
}
