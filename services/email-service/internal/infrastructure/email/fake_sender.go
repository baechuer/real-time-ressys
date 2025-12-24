package email

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/rs/zerolog"
)

// FakeSender is a development/testing sender.
// It can simulate transient/permanent failures via env var.
//
// FAKE_FAIL_MODE:
// - "none" (default): always succeed
// - "transient": return Temporary() error (retriable)
// - "permanent": return Permanent() error (non-retriable)
type FakeSender struct {
	lg zerolog.Logger
}

func NewFakeSender(lg zerolog.Logger) *FakeSender {
	return &FakeSender{
		lg: lg.With().Str("component", "fake_sender").Logger(),
	}
}

func (s *FakeSender) SendVerifyEmail(ctx context.Context, toEmail, url string) error {
	s.lg.Info().
		Str("to", toEmail).
		Str("url", url).
		Msg("FAKE send verify email")

	return s.maybeFail("verify")
}

func (s *FakeSender) SendPasswordReset(ctx context.Context, toEmail, url string) error {
	s.lg.Info().
		Str("to", toEmail).
		Str("url", url).
		Msg("FAKE send password reset")

	return s.maybeFail("reset")
}

func (s *FakeSender) maybeFail(kind string) error {
	mode := strings.TrimSpace(strings.ToLower(os.Getenv("FAKE_FAIL_MODE")))
	if mode == "" || mode == "none" {
		return nil
	}

	// Small sleep makes logs easier to read and simulates IO
	time.Sleep(50 * time.Millisecond)

	switch mode {
	case "transient":
		return TemporaryError{msg: fmt.Sprintf("fake transient failure (%s)", kind)}
	case "permanent":
		return PermanentError{msg: fmt.Sprintf("fake permanent failure (%s)", kind)}
	default:
		return nil
	}
}

// TemporaryError marks a retriable failure (e.g., network timeout, SMTP 4xx, provider 5xx).
type TemporaryError struct{ msg string }

func (e TemporaryError) Error() string   { return e.msg }
func (e TemporaryError) Temporary() bool { return true } // used by consumer classification
func (e TemporaryError) Permanent() bool { return false }
func (e TemporaryError) Unwrap() error   { return nil }

// PermanentError marks a non-retriable failure (e.g., schema violation, hard bounce).
type PermanentError struct{ msg string }

func (e PermanentError) Error() string   { return e.msg }
func (e PermanentError) Permanent() bool { return true } // consumer sends to final DLQ
