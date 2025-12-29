package security

import "time"

type TokenClaims struct {
	UserID  string
	Role    string
	Ver     int64
	Exp     time.Time
	Issuer  string
	Subject string
}
