package model

import "time"

type userRow struct {
	ID            string
	Email         string
	PasswordHash  string
	Role          string
	EmailVerified bool
	Locked        bool
	CreatedAt     time.Time
}
