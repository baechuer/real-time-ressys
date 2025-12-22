package logger

import (
	"bytes"
	"os"
	"strings"
	"sync"
	"testing"

	zlog "github.com/rs/zerolog/log"
)

var envMu sync.Mutex

func withEnv(t *testing.T, kv map[string]string) {
	t.Helper()

	envMu.Lock()
	t.Cleanup(envMu.Unlock)

	// save + set
	prev := map[string]*string{}
	for k, v := range kv {
		if old, ok := os.LookupEnv(k); ok {
			tmp := old
			prev[k] = &tmp
		} else {
			prev[k] = nil
		}
		_ = os.Setenv(k, v)
	}

	t.Cleanup(func() {
		for k, old := range prev {
			if old == nil {
				_ = os.Unsetenv(k)
			} else {
				_ = os.Setenv(k, *old)
			}
		}
	})
}

func TestInitWithWriter_Defaults_ToInfoAndConsole(t *testing.T) {
	withEnv(t, map[string]string{
		"LOG_LEVEL":  "",
		"LOG_FORMAT": "",
	})

	var buf bytes.Buffer
	InitWithWriter(&buf)

	if Logger.GetLevel().String() != "info" {
		t.Fatalf("expected level=info, got %s", Logger.GetLevel().String())
	}
	if zlog.Logger.GetLevel().String() != "info" {
		t.Fatalf("expected global level=info, got %s", zlog.Logger.GetLevel().String())
	}

	Logger.Info().Msg("hello")
	out := buf.String()
	if out == "" {
		t.Fatalf("expected output")
	}
	if strings.HasPrefix(strings.TrimSpace(out), "{") {
		t.Fatalf("expected console output, got json-like: %q", out)
	}
	if !strings.Contains(out, "hello") {
		t.Fatalf("expected message in output, got: %q", out)
	}
}

func TestInitWithWriter_InvalidLogLevel_FallsBackToInfo(t *testing.T) {
	withEnv(t, map[string]string{
		"LOG_LEVEL":  "not-a-level",
		"LOG_FORMAT": "console",
	})

	var buf bytes.Buffer
	InitWithWriter(&buf)

	if Logger.GetLevel().String() != "info" {
		t.Fatalf("expected level=info fallback, got %s", Logger.GetLevel().String())
	}

	Logger.Debug().Msg("debug-should-not-print")
	Logger.Info().Msg("info-should-print")
	out := buf.String()

	if strings.Contains(out, "debug-should-not-print") {
		t.Fatalf("did not expect debug output at info level, got: %q", out)
	}
	if !strings.Contains(out, "info-should-print") {
		t.Fatalf("expected info output, got: %q", out)
	}
}

func TestInitWithWriter_JSONFormat_OutputsJSON(t *testing.T) {
	withEnv(t, map[string]string{
		"LOG_LEVEL":  "info",
		"LOG_FORMAT": "json",
	})

	var buf bytes.Buffer
	InitWithWriter(&buf)

	Logger.Info().Str("k", "v").Msg("hello")
	out := strings.TrimSpace(buf.String())

	if out == "" {
		t.Fatalf("expected output")
	}
	if !strings.HasPrefix(out, "{") || !strings.HasSuffix(out, "}") {
		t.Fatalf("expected json object line, got: %q", out)
	}
	if !strings.Contains(out, `"message":"hello"`) && !strings.Contains(out, `"msg":"hello"`) {
		t.Fatalf("expected msg/message field, got: %q", out)
	}
	if !strings.Contains(out, `"k":"v"`) {
		t.Fatalf("expected k field, got: %q", out)
	}
}

func TestInitWithWriter_ConsoleFormat_OutputsNonJSON(t *testing.T) {
	withEnv(t, map[string]string{
		"LOG_LEVEL":  "info",
		"LOG_FORMAT": "console",
	})

	var buf bytes.Buffer
	InitWithWriter(&buf)

	Logger.Info().Msg("hello")
	out := strings.TrimSpace(buf.String())

	if out == "" {
		t.Fatalf("expected output")
	}
	if strings.HasPrefix(out, "{") {
		t.Fatalf("expected console output, got json-like: %q", out)
	}
	if !strings.Contains(out, "hello") {
		t.Fatalf("expected message in output, got: %q", out)
	}
}

func TestInit_SetsGlobalLoggerToo(t *testing.T) {
	withEnv(t, map[string]string{
		"LOG_LEVEL":  "info",
		"LOG_FORMAT": "console",
	})

	Init()

	if zlog.Logger.GetLevel().String() != Logger.GetLevel().String() {
		t.Fatalf("expected global logger level to match package logger level; global=%s pkg=%s",
			zlog.Logger.GetLevel().String(), Logger.GetLevel().String())
	}
}
