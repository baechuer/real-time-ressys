package logger

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/rs/zerolog"
	zlog "github.com/rs/zerolog/log"
)

func setenv(t *testing.T, k, v string) {
	t.Helper()
	old, ok := os.LookupEnv(k)
	if err := os.Setenv(k, v); err != nil {
		t.Fatalf("setenv %s: %v", k, err)
	}
	t.Cleanup(func() {
		if ok {
			_ = os.Setenv(k, old)
		} else {
			_ = os.Unsetenv(k)
		}
	})
}

func TestInit_DefaultsToConsoleInfo(t *testing.T) {
	_ = os.Unsetenv("LOG_LEVEL")
	_ = os.Unsetenv("LOG_FORMAT")

	var buf bytes.Buffer
	InitWithWriter(&buf)

	// Should set global logger to our package logger
	if zlog.Logger.GetLevel() != Logger.GetLevel() {
		t.Fatalf("global logger level not set; got %v want %v", zlog.Logger.GetLevel(), Logger.GetLevel())
	}
	if Logger.GetLevel() != zerolog.InfoLevel {
		t.Fatalf("expected info level, got %v", Logger.GetLevel())
	}

	Logger.Info().Msg("hello")
	out := buf.String()

	// ConsoleWriter output usually contains "INF" and the message.
	// Don't assert too tightly (timestamp etc. may vary).
	if !strings.Contains(out, "hello") {
		t.Fatalf("expected output to contain message, got: %q", out)
	}
}

func TestInit_JSONFormat(t *testing.T) {
	setenv(t, "LOG_FORMAT", "json")
	setenv(t, "LOG_LEVEL", "debug")

	var buf bytes.Buffer
	InitWithWriter(&buf)

	if Logger.GetLevel() != zerolog.DebugLevel {
		t.Fatalf("expected debug level, got %v", Logger.GetLevel())
	}

	Logger.Info().Msg("hello")
	out := buf.String()

	// JSON logger should include `"message":"hello"` (zerolog uses "message")
	if !strings.Contains(out, `"message":"hello"`) {
		t.Fatalf("expected json output, got: %q", out)
	}
}

func TestInit_InvalidLevel_FallsBackToInfo(t *testing.T) {
	setenv(t, "LOG_LEVEL", "not-a-level")
	setenv(t, "LOG_FORMAT", "json")

	var buf bytes.Buffer
	InitWithWriter(&buf)

	if Logger.GetLevel() != zerolog.InfoLevel {
		t.Fatalf("expected fallback to info, got %v", Logger.GetLevel())
	}
}
