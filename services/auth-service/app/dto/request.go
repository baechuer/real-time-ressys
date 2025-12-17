package dto

// RegisterRequest represents the data needed to register a new user
type RegisterRequest struct {
	Email    string `json:"email" validate:"required,email,max=255"`
	Password string `json:"password" validate:"required,min=8,max=128,password_strength"`
	Username string `json:"username" validate:"required,min=3,max=50,username_format"`
}

// LoginRequest represents the data needed to login
type LoginRequest struct {
	Email    string `json:"email" validate:"required,email,max=255"`
	Password string `json:"password" validate:"required,min=8,max=128,password_strength"`
}
