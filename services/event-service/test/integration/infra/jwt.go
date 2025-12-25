package infra

import (
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type AccessClaims struct {
	UserID string `json:"uid"`
	Role   string `json:"role"`
	Ver    int64  `json:"ver"`
	jwt.RegisteredClaims
}

func MakeToken(secret, issuer, uid, role string, ver int64, ttl time.Duration) (string, error) {
	now := time.Now().UTC()
	claims := AccessClaims{
		UserID: uid,
		Role:   role,
		Ver:    ver,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    issuer,
			Subject:   uid,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(ttl)),
		},
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return tok.SignedString([]byte(secret))
}
