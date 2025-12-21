package domain

type User struct {
	ID            string
	Email         string
	PasswordHash  string
	Role          string
	EmailVerified bool
	Locked        bool
}
