package security

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type HS256Verifier struct {
	secret []byte
}

func NewHS256Verifier(secret string) *HS256Verifier {
	return &HS256Verifier{secret: []byte(secret)}
}

type accessClaims struct {
	UserID string `json:"uid"`
	Role   string `json:"role"`
	Ver    int64  `json:"ver"`
	jwt.RegisteredClaims
}

func (v *HS256Verifier) VerifyAccessToken(token string) (TokenClaims, error) {
	parsed, err := jwt.ParseWithClaims(token, &accessClaims{}, func(t *jwt.Token) (any, error) {
		// prevent alg confusion
		if t.Method == nil || t.Method.Alg() != jwt.SigningMethodHS256.Alg() {
			return nil, ErrTokenInvalid
		}
		return v.secret, nil
	}, jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}))

	if err != nil {
		// jwt/v5 commonly supports errors.Is(err, jwt.ErrTokenExpired)
		if errors.Is(err, jwt.ErrTokenExpired) {
			return TokenClaims{}, ErrTokenExpired
		}
		return TokenClaims{}, ErrTokenInvalid
	}

	claims, ok := parsed.Claims.(*accessClaims)
	if !ok || !parsed.Valid {
		return TokenClaims{}, ErrTokenInvalid
	}

	exp := time.Time{}
	if claims.ExpiresAt != nil {
		exp = claims.ExpiresAt.Time
	}

	return TokenClaims{
		UserID:  claims.UserID,
		Role:    claims.Role,
		Ver:     claims.Ver,
		Exp:     exp,
		Issuer:  claims.Issuer,
		Subject: claims.Subject,
	}, nil
}
