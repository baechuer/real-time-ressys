package domain

import (
	"errors"
	"fmt"
)

// ErrKind is used to map domain errors to HTTP status codes consistently.
type ErrKind string

const (
	KindValidation     ErrKind = "validation"     // 400
	KindAuth           ErrKind = "auth"           // 401
	KindForbidden      ErrKind = "forbidden"      // 403
	KindNotFound       ErrKind = "not_found"      // 404
	KindConflict       ErrKind = "conflict"       // 409
	KindRateLimited    ErrKind = "rate_limited"   // 429
	KindInfrastructure ErrKind = "infrastructure" // 503/500
	KindInternal       ErrKind = "internal"       // 500
)

// Error is a structured domain error.
// - Kind: high-level category for HTTP mapping
// - Code: stable machine code (do not change casually)
// - Message: safe summary for clients (avoid leaking sensitive details)
// - Meta: optional details (field, reason, etc.)
// - Cause: wrapped internal error for logging/diagnostics
type Error struct {
	Kind    ErrKind
	Code    string
	Message string
	Meta    map[string]string
	Cause   error
}

func (e *Error) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("%s (%s): %s: %v", e.Kind, e.Code, e.Message, e.Cause)
	}
	return fmt.Sprintf("%s (%s): %s", e.Kind, e.Code, e.Message)
}

func (e *Error) Unwrap() error { return e.Cause }

func New(kind ErrKind, code, msg string) *Error {
	return &Error{Kind: kind, Code: code, Message: msg}
}

func Wrap(kind ErrKind, code, msg string, cause error) *Error {
	return &Error{Kind: kind, Code: code, Message: msg, Cause: cause}
}

func WithMeta(err *Error, meta map[string]string) *Error {
	err.Meta = meta
	return err
}

func Is(err error, code string) bool {
	var de *Error
	if errors.As(err, &de) {
		return de.Code == code
	}
	return false
}

// ----------------------
// Validation errors (400)
// ----------------------

func ErrInvalidJSON(cause error) *Error {
	return Wrap(KindValidation, "invalid_json", "invalid JSON body", cause)
}

func ErrMissingField(field string) *Error {
	return WithMeta(New(KindValidation, "missing_field", "missing required field"), map[string]string{
		"field": field,
	})
}

func ErrInvalidField(field, reason string) *Error {
	return WithMeta(New(KindValidation, "invalid_field", "invalid field"), map[string]string{
		"field":  field,
		"reason": reason,
	})
}

func ErrWeakPassword(reason string) *Error {
	return WithMeta(New(KindValidation, "weak_password", "password does not meet requirements"), map[string]string{
		"reason": reason,
	})
}

// ----------------------
// Auth errors (401)
// ----------------------

// IMPORTANT: use this for login failures to avoid user enumeration.
func ErrInvalidCredentials() *Error {
	return New(KindAuth, "invalid_credentials", "invalid email or password")
}

func ErrTokenMissing() *Error {
	return New(KindAuth, "token_missing", "no token provided")
}

func ErrTokenInvalid() *Error {
	return New(KindAuth, "token_invalid", "invalid token")
}

func ErrTokenExpired() *Error {
	return New(KindAuth, "token_expired", "token is expired")
}

func ErrRefreshTokenInvalid() *Error {
	return New(KindAuth, "refresh_token_invalid", "invalid refresh token")
}

func ErrRefreshTokenExpired() *Error {
	return New(KindAuth, "refresh_token_expired", "refresh token is expired")
}

// Detect refresh token rotation abuse (old token reused).
func ErrRefreshTokenReused() *Error {
	return New(KindConflict, "refresh_token_reused", "refresh token reuse detected")
}

// ----------------------
// Forbidden (403)
// ----------------------

func ErrForbidden() *Error {
	return New(KindForbidden, "forbidden", "forbidden")
}

func ErrInsufficientRole(required string) *Error {
	return WithMeta(New(KindForbidden, "insufficient_role", "insufficient role"), map[string]string{
		"required": required,
	})
}

// ----------------------
// Not Found (404)
// ----------------------

func ErrUserNotFound() *Error {
	return New(KindNotFound, "user_not_found", "user not found")
}

func ErrResetTokenNotFound() *Error {
	return New(KindNotFound, "reset_token_not_found", "reset token not found")
}

func ErrVerifyTokenNotFound() *Error {
	return New(KindNotFound, "verify_token_not_found", "verification token not found")
}

// ----------------------
// Conflict (409)
// ----------------------

func ErrEmailAlreadyExists() *Error {
	return New(KindConflict, "email_already_exists", "email already registered")
}

func ErrUsernameAlreadyExists() *Error {
	return New(KindConflict, "username_already_exists", "username already registered")
}

func ErrAccountLocked() *Error {
	return New(KindForbidden, "account_locked", "account locked")
}

func ErrEmailNotVerified() *Error {
	return New(KindForbidden, "email_not_verified", "email not verified")
}

// ----------------------
// Moderation / RBAC errors
// ----------------------

// A user cannot ban/unban themselves.
func ErrCannotModerateSelf() *Error {
	return New(KindForbidden, "cannot_moderate_self", "cannot moderate self")
}

// Moderator cannot ban/unban admin users.
func ErrCannotModerateAdmin() *Error {
	return New(KindForbidden, "cannot_moderate_admin", "cannot moderate admin user")
}

// Admin cannot perform this action on themselves.
func ErrCannotAffectSelf() *Error {
	return New(KindForbidden, "cannot_affect_self", "cannot perform this action on self")
}
func ErrLastAdminProtected() *Error {
	return New(KindForbidden, "last_admin_protected", "cannot remove last admin")
}

// ----------------------
// Rate limit (429)
// ----------------------

func ErrRateLimited(scope string) *Error {
	return WithMeta(New(KindRateLimited, "rate_limited", "too many requests"), map[string]string{
		"scope": scope,
	})
}

// ----------------------
// Infrastructure / internal (5xx)
// ----------------------

func ErrDBUnavailable(cause error) *Error {
	return Wrap(KindInfrastructure, "db_unavailable", "database unavailable", cause)
}

func ErrRedisUnavailable(cause error) *Error {
	return Wrap(KindInfrastructure, "redis_unavailable", "cache unavailable", cause)
}

func ErrRabbitUnavailable(cause error) *Error {
	return Wrap(KindInfrastructure, "rabbit_unavailable", "message broker unavailable", cause)
}

func ErrHashFailed(cause error) *Error {
	return Wrap(KindInternal, "hash_failed", "password hashing failed", cause)
}

func ErrTokenSignFailed(cause error) *Error {
	return Wrap(KindInternal, "token_sign_failed", "token signing failed", cause)
}

func ErrRandomFailed(cause error) *Error {
	return Wrap(KindInternal, "random_failed", "random generation failed", cause)
}

func ErrInternal(cause error) *Error {
	return Wrap(KindInternal, "internal_error", "internal error", cause)
}
func ErrNotImplemented() *Error {
	return New(KindInternal, "not_implemented", "not implemented")
}

// Admin cannot demote the last remaining admin.
// (Enforce later when admin counting is implemented.)
func ErrCannotDemoteLastAdmin() *Error {
	return New(KindConflict, "cannot_demote_last_admin", "cannot demote the last admin")
}
func ErrInvalidRole(role string) *Error {
	return WithMeta(
		New(KindValidation, "invalid_role", "invalid role"),
		map[string]string{"role": role},
	)
}
