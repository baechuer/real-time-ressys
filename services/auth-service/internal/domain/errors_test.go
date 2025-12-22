package domain

import (
	"errors"
	"testing"
)

func TestError_ErrorString_NoCause(t *testing.T) {
	err := New(KindAuth, "invalid_credentials", "invalid email or password")

	msg := err.Error()
	if msg == "" {
		t.Fatal("expected non-empty error string")
	}
}

func TestError_ErrorString_WithCause(t *testing.T) {
	root := errors.New("root cause")
	err := Wrap(KindInternal, "hash_failed", "hash failed", root)

	if !errors.Is(err, root) {
		t.Fatalf("expected errors.Is to match cause")
	}
}

func TestError_Unwrap(t *testing.T) {
	root := errors.New("root")
	err := Wrap(KindInternal, "internal_error", "internal", root)

	if errors.Unwrap(err) != root {
		t.Fatalf("unwrap did not return cause")
	}
}

func TestWithMeta_AttachesMeta(t *testing.T) {
	err := ErrMissingField("email")

	if err.Meta == nil {
		t.Fatalf("expected meta to be set")
	}
	if err.Meta["field"] != "email" {
		t.Fatalf("unexpected meta value: %+v", err.Meta)
	}
}

func TestIs_MatchesCode(t *testing.T) {
	err := ErrInvalidCredentials()

	if !Is(err, "invalid_credentials") {
		t.Fatalf("expected code match")
	}
	if Is(err, "something_else") {
		t.Fatalf("unexpected code match")
	}
}

func TestIs_NonDomainError(t *testing.T) {
	err := errors.New("plain error")

	if Is(err, "invalid_credentials") {
		t.Fatalf("should not match non-domain error")
	}
}
func TestValidationErrors(t *testing.T) {
	err := ErrInvalidField("email", "bad format")
	if err.Kind != KindValidation || err.Code != "invalid_field" {
		t.Fatalf("unexpected error: %+v", err)
	}
}

func TestAuthErrors(t *testing.T) {
	err := ErrTokenMissing()
	if err.Kind != KindAuth || err.Code != "token_missing" {
		t.Fatalf("unexpected error: %+v", err)
	}
}

func TestForbiddenErrors(t *testing.T) {
	err := ErrForbidden()
	if err.Kind != KindForbidden {
		t.Fatalf("unexpected kind")
	}
}

func TestNotFoundErrors(t *testing.T) {
	err := ErrUserNotFound()
	if err.Kind != KindNotFound {
		t.Fatalf("unexpected kind")
	}
}

func TestConflictErrors(t *testing.T) {
	err := ErrEmailAlreadyExists()
	if err.Kind != KindConflict {
		t.Fatalf("unexpected kind")
	}
}

func TestRateLimitedError(t *testing.T) {
	err := ErrRateLimited("login")
	if err.Kind != KindRateLimited {
		t.Fatalf("unexpected kind")
	}
	if err.Meta["scope"] != "login" {
		t.Fatalf("unexpected meta")
	}
}

func TestInternalErrors(t *testing.T) {
	root := errors.New("boom")
	err := ErrDBUnavailable(root)

	if err.Kind != KindInfrastructure {
		t.Fatalf("unexpected kind")
	}
	if !errors.Is(err, root) {
		t.Fatalf("expected wrapped cause")
	}
}
