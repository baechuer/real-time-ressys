package models

import "time"

type User struct {
	ID              int64
	Username        string
	Email           string
	PasswordHash    string
	RoleID          int // Add this
	CreatedAt       time.Time
	IsEmailVerified bool
}
