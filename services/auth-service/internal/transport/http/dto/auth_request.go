package dto

import (
	"strings"

	"github.com/baechuer/real-time-ressys/services/auth-service/internal/domain"
)

// -------- Core auth --------

type RegisterRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func (r *RegisterRequest) Validate() error {
	if r.Email == "" {
		return domain.ErrMissingField("email")
	}
	if r.Password == "" {
		return domain.ErrMissingField("password")
	}
	if len(r.Password) < 12 {
		return domain.ErrWeakPassword("min length 12")
	}

	if !strings.Contains(r.Email, "@") {
		return domain.ErrInvalidField("email", "invalid format")

	}
	return nil
}

type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func (r *LoginRequest) Validate() error {
	if r.Email == "" {
		return domain.ErrMissingField("email")
	}
	if r.Password == "" {
		return domain.ErrMissingField("password")
	}
	return nil
}

// If refresh token is in HttpOnly cookie, this request can be empty.
// If you accept refresh token in JSON, keep RefreshToken and enforce validation.
type RefreshRequest struct {
	RefreshToken string `json:"refresh_token,omitempty"`
}

func (r *RefreshRequest) Validate() error {
	// If you REQUIRE body refresh token, uncomment:
	// if r.RefreshToken == "" { return domain.ErrMissingField("refresh_token") }
	return nil
}

type LogoutRequest struct{}

// -------- Email verification --------

type VerifyEmailRequest struct {
	Email string `json:"email"`
}

func (r *VerifyEmailRequest) Validate() error {
	r.Email = strings.TrimSpace(strings.ToLower(r.Email))
	if r.Email == "" {
		return domain.ErrMissingField("email")
	}
	if !strings.Contains(r.Email, "@") {
		return domain.ErrInvalidField("email", "invalid format")
	}
	return nil
}

type VerifyEmailConfirmRequest struct {
	Token string `json:"token"`
}

func (r *VerifyEmailConfirmRequest) Validate() error {
	if r.Token == "" {
		return domain.ErrMissingField("token")
	}
	return nil
}

// -------- Password reset --------

// Step A: request reset (server should always return 200 to avoid enumeration)
type PasswordResetRequest struct {
	Email string `json:"email"`
}

func (r *PasswordResetRequest) Validate() error {
	r.Email = strings.TrimSpace(strings.ToLower(r.Email))

	if r.Email == "" {
		return domain.ErrMissingField("email")
	}
	if !strings.Contains(r.Email, "@") {
		return domain.ErrInvalidField("email", "invalid format")
	}
	return nil
}

// Step B: confirm reset
type PasswordResetConfirmRequest struct {
	Token       string `json:"token"`
	NewPassword string `json:"new_password"`
}

func (r *PasswordResetConfirmRequest) Validate() error {
	if r.Token == "" {
		return domain.ErrMissingField("token")
	}
	if r.NewPassword == "" {
		return domain.ErrMissingField("new_password")
	}
	if len(r.NewPassword) < 12 {
		return domain.ErrWeakPassword("min length 12")
	}
	return nil
}

// Optional: validate reset token (GET /password/reset/validate?token=...)
type PasswordResetValidateQuery struct {
	Token string `json:"-"` // filled from query param, not JSON
}

func (q *PasswordResetValidateQuery) Validate() error {
	if q.Token == "" {
		return domain.ErrMissingField("token")
	}
	return nil
}

// -------- Password change (authenticated) --------

type PasswordChangeRequest struct {
	OldPassword string `json:"old_password"`
	NewPassword string `json:"new_password"`
}

func (r *PasswordChangeRequest) Validate() error {
	if r.OldPassword == "" {
		return domain.ErrMissingField("old_password")
	}
	if r.NewPassword == "" {
		return domain.ErrMissingField("new_password")
	}
	if len(r.NewPassword) < 12 {
		return domain.ErrWeakPassword("min length 12")
	}
	return nil
}

type SetUserRoleRequest struct {
	Role string `json:"role"`
}

func (r *SetUserRoleRequest) Validate() error {
	if r.Role == "" {
		return domain.ErrMissingField("role")
	}
	if !domain.IsValidRole(r.Role) {
		return domain.ErrInvalidField("role", "invalid role")
	}
	return nil
}

// -------- Sessions --------

type SessionsRevokeRequest struct{}

// -------- Optional --------

type MeStatusRequest struct{}
