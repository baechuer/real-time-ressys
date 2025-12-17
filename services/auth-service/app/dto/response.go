package dto

// AuthResponse represents the response after successful authentication
type AuthResponse struct {
	AccessToken  string       `json:"access_token"`
	RefreshToken string       `json:"refresh_token"`
	User         UserResponse `json:"user"`
}

// UserResponse represents user data in API responses (excludes sensitive fields)
type UserResponse struct {
	ID              int64  `json:"id"`
	Username        string `json:"username"`
	Email           string `json:"email"`
	IsEmailVerified bool   `json:"is_email_verified"`
	CreatedAt       string `json:"created_at"`
	RoleID          int    `json:"role_id"`
}

// RegisterResponse represents the response after successful registration
type RegisterResponse struct {
	Message string `json:"message"`
}

// ErrorResponse represents an error response
type ErrorResponse struct {
	Error   string `json:"error"`
	Code    string `json:"code,omitempty"`
	Details string `json:"details,omitempty"`
}
