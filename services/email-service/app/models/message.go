package models

// EmailVerificationMessage represents the email verification message from auth-service
// This format matches auth-service/app/services/publisher.go exactly
type EmailVerificationMessage struct {
	Type            string `json:"type"`             // "email_verification"
	Email           string `json:"email"`
	VerificationURL string `json:"verification_url"`
}

// PasswordResetMessage represents the password reset message from auth-service
// This format matches auth-service/app/services/publisher.go exactly
type PasswordResetMessage struct {
	Type     string `json:"type"`      // "password_reset"
	Email    string `json:"email"`
	ResetURL string `json:"reset_url"`
}

