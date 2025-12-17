package dto

type RequestPasswordResetRequest struct {
	// Empty - user ID comes from access token
}

// ForgotPasswordRequest represents a request to send password reset email (unauthenticated)
type ForgotPasswordRequest struct {
	Email string `json:"email" validate:"required,email,max=255"`
}

type ResetPasswordRequest struct {
	Token       string `json:"token" validate:"required"`
	NewPassword string `json:"new_password" validate:"required,min=8,max=128,password_strength"`
}
