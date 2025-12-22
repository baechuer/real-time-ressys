package dto

import (
	"testing"

	"github.com/baechuer/real-time-ressys/services/auth-service/internal/domain"
)

func TestRegisterRequest_Validate(t *testing.T) {
	t.Run("missing email", func(t *testing.T) {
		r := &RegisterRequest{Email: "", Password: "123456789012"}
		err := r.Validate()
		if err == nil || !domain.Is(err, "missing_field") {
			t.Fatalf("expected missing_field(email), got: %v", err)
		}
	})

	t.Run("missing password", func(t *testing.T) {
		r := &RegisterRequest{Email: "a@b.com", Password: ""}
		err := r.Validate()
		if err == nil || !domain.Is(err, "missing_field") {
			t.Fatalf("expected missing_field(password), got: %v", err)
		}
	})

	t.Run("weak password (<12)", func(t *testing.T) {
		r := &RegisterRequest{Email: "a@b.com", Password: "short"}
		err := r.Validate()
		if err == nil || !domain.Is(err, "weak_password") {
			t.Fatalf("expected weak_password, got: %v", err)
		}
	})

	t.Run("invalid email format (no @)", func(t *testing.T) {
		r := &RegisterRequest{Email: "abc", Password: "123456789012"}
		err := r.Validate()
		if err == nil || !domain.Is(err, "invalid_field") {
			t.Fatalf("expected invalid_field(email), got: %v", err)
		}
	})

	t.Run("ok", func(t *testing.T) {
		r := &RegisterRequest{Email: "a@b.com", Password: "123456789012"}
		err := r.Validate()
		if err != nil {
			t.Fatalf("expected nil, got: %v", err)
		}
	})
}

func TestLoginRequest_Validate(t *testing.T) {
	t.Run("missing email", func(t *testing.T) {
		r := &LoginRequest{Email: "", Password: "x"}
		err := r.Validate()
		if err == nil || !domain.Is(err, "missing_field") {
			t.Fatalf("expected missing_field(email), got: %v", err)
		}
	})

	t.Run("missing password", func(t *testing.T) {
		r := &LoginRequest{Email: "a@b.com", Password: ""}
		err := r.Validate()
		if err == nil || !domain.Is(err, "missing_field") {
			t.Fatalf("expected missing_field(password), got: %v", err)
		}
	})

	t.Run("ok", func(t *testing.T) {
		r := &LoginRequest{Email: "a@b.com", Password: "x"}
		err := r.Validate()
		if err != nil {
			t.Fatalf("expected nil, got: %v", err)
		}
	})
}

func TestRefreshRequest_Validate(t *testing.T) {
	t.Run("always ok (cookie-based)", func(t *testing.T) {
		r := &RefreshRequest{}
		if err := r.Validate(); err != nil {
			t.Fatalf("expected nil, got: %v", err)
		}
	})
}

func TestVerifyEmailRequest_Validate(t *testing.T) {
	t.Run("trims + lowercases", func(t *testing.T) {
		r := &VerifyEmailRequest{Email: "  TeSt@Example.com "}
		if err := r.Validate(); err != nil {
			t.Fatalf("expected nil, got: %v", err)
		}
		if r.Email != "test@example.com" {
			t.Fatalf("expected normalized email, got: %q", r.Email)
		}
	})

	t.Run("missing email", func(t *testing.T) {
		r := &VerifyEmailRequest{Email: "   "}
		err := r.Validate()
		if err == nil || !domain.Is(err, "missing_field") {
			t.Fatalf("expected missing_field(email), got: %v", err)
		}
	})

	t.Run("invalid email format", func(t *testing.T) {
		r := &VerifyEmailRequest{Email: "abc"}
		err := r.Validate()
		if err == nil || !domain.Is(err, "invalid_field") {
			t.Fatalf("expected invalid_field(email), got: %v", err)
		}
	})
}

func TestVerifyEmailConfirmRequest_Validate(t *testing.T) {
	t.Run("missing token", func(t *testing.T) {
		r := &VerifyEmailConfirmRequest{Token: ""}
		err := r.Validate()
		if err == nil || !domain.Is(err, "missing_field") {
			t.Fatalf("expected missing_field(token), got: %v", err)
		}
	})

	t.Run("ok", func(t *testing.T) {
		r := &VerifyEmailConfirmRequest{Token: "t"}
		if err := r.Validate(); err != nil {
			t.Fatalf("expected nil, got: %v", err)
		}
	})
}

func TestPasswordResetRequest_Validate(t *testing.T) {
	t.Run("normalizes email", func(t *testing.T) {
		r := &PasswordResetRequest{Email: "  TeSt@Example.com "}
		if err := r.Validate(); err != nil {
			t.Fatalf("expected nil, got: %v", err)
		}
		if r.Email != "test@example.com" {
			t.Fatalf("expected normalized email, got: %q", r.Email)
		}
	})

	t.Run("missing email", func(t *testing.T) {
		r := &PasswordResetRequest{Email: " "}
		err := r.Validate()
		if err == nil || !domain.Is(err, "missing_field") {
			t.Fatalf("expected missing_field(email), got: %v", err)
		}
	})

	t.Run("invalid email format", func(t *testing.T) {
		r := &PasswordResetRequest{Email: "abc"}
		err := r.Validate()
		if err == nil || !domain.Is(err, "invalid_field") {
			t.Fatalf("expected invalid_field(email), got: %v", err)
		}
	})
}

func TestPasswordResetConfirmRequest_Validate(t *testing.T) {
	t.Run("missing token", func(t *testing.T) {
		r := &PasswordResetConfirmRequest{Token: "", NewPassword: "123456789012"}
		err := r.Validate()
		if err == nil || !domain.Is(err, "missing_field") {
			t.Fatalf("expected missing_field(token), got: %v", err)
		}
	})

	t.Run("missing new_password", func(t *testing.T) {
		r := &PasswordResetConfirmRequest{Token: "t", NewPassword: ""}
		err := r.Validate()
		if err == nil || !domain.Is(err, "missing_field") {
			t.Fatalf("expected missing_field(new_password), got: %v", err)
		}
	})

	t.Run("weak new_password", func(t *testing.T) {
		r := &PasswordResetConfirmRequest{Token: "t", NewPassword: "short"}
		err := r.Validate()
		if err == nil || !domain.Is(err, "weak_password") {
			t.Fatalf("expected weak_password, got: %v", err)
		}
	})

	t.Run("ok", func(t *testing.T) {
		r := &PasswordResetConfirmRequest{Token: "t", NewPassword: "123456789012"}
		if err := r.Validate(); err != nil {
			t.Fatalf("expected nil, got: %v", err)
		}
	})
}

func TestPasswordResetValidateQuery_Validate(t *testing.T) {
	t.Run("missing token", func(t *testing.T) {
		q := &PasswordResetValidateQuery{Token: ""}
		err := q.Validate()
		if err == nil || !domain.Is(err, "missing_field") {
			t.Fatalf("expected missing_field(token), got: %v", err)
		}
	})

	t.Run("ok", func(t *testing.T) {
		q := &PasswordResetValidateQuery{Token: "t"}
		if err := q.Validate(); err != nil {
			t.Fatalf("expected nil, got: %v", err)
		}
	})
}

func TestPasswordChangeRequest_Validate(t *testing.T) {
	t.Run("missing old_password", func(t *testing.T) {
		r := &PasswordChangeRequest{OldPassword: "", NewPassword: "123456789012"}
		err := r.Validate()
		if err == nil || !domain.Is(err, "missing_field") {
			t.Fatalf("expected missing_field(old_password), got: %v", err)
		}
	})

	t.Run("missing new_password", func(t *testing.T) {
		r := &PasswordChangeRequest{OldPassword: "x", NewPassword: ""}
		err := r.Validate()
		if err == nil || !domain.Is(err, "missing_field") {
			t.Fatalf("expected missing_field(new_password), got: %v", err)
		}
	})

	t.Run("weak new_password", func(t *testing.T) {
		r := &PasswordChangeRequest{OldPassword: "x", NewPassword: "short"}
		err := r.Validate()
		if err == nil || !domain.Is(err, "weak_password") {
			t.Fatalf("expected weak_password, got: %v", err)
		}
	})

	t.Run("ok", func(t *testing.T) {
		r := &PasswordChangeRequest{OldPassword: "x", NewPassword: "123456789012"}
		if err := r.Validate(); err != nil {
			t.Fatalf("expected nil, got: %v", err)
		}
	})
}

func TestSetUserRoleRequest_Validate(t *testing.T) {
	t.Run("missing role", func(t *testing.T) {
		r := &SetUserRoleRequest{Role: ""}
		err := r.Validate()
		if err == nil || !domain.Is(err, "missing_field") {
			t.Fatalf("expected missing_field(role), got: %v", err)
		}
	})

	t.Run("invalid role", func(t *testing.T) {
		r := &SetUserRoleRequest{Role: "superadmin"}
		err := r.Validate()
		if err == nil || !domain.Is(err, "invalid_field") {
			t.Fatalf("expected invalid_field(role), got: %v", err)
		}
	})

	t.Run("ok roles", func(t *testing.T) {
		for _, role := range []string{"user", "moderator", "admin"} {
			r := &SetUserRoleRequest{Role: role}
			if err := r.Validate(); err != nil {
				t.Fatalf("expected nil for role=%s, got: %v", role, err)
			}
		}
	})
}
