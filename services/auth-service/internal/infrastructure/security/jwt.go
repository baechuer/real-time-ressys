package security

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/baechuer/real-time-ressys/services/auth-service/internal/application/auth"
	"github.com/baechuer/real-time-ressys/services/auth-service/internal/domain"
)

type JWTSigner struct {
	secret []byte
	issuer string
}

func NewJWTSigner(secret string, issuer string) *JWTSigner {
	return &JWTSigner{
		secret: []byte(secret),
		issuer: issuer,
	}
}

type accessClaims struct {
	UserID string `json:"uid"`
	Role   string `json:"role"`
	jwt.RegisteredClaims
}

func (s *JWTSigner) SignAccessToken(userID string, role string, ttl time.Duration) (string, error) {
	now := time.Now()
	claims := accessClaims{
		UserID: userID,
		Role:   role,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    s.issuer,
			Subject:   userID,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(ttl)),
		},
	}

	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := tok.SignedString(s.secret)
	if err != nil {
		return "", domain.ErrTokenSignFailed(err)
	}
	return signed, nil
}

func (s *JWTSigner) VerifyAccessToken(token string) (auth.TokenClaims, error) {
	parsed, err := jwt.ParseWithClaims(token, &accessClaims{}, func(t *jwt.Token) (any, error) {
		// prevent alg confusion
		if t.Method != jwt.SigningMethodHS256 {
			return nil, domain.ErrTokenInvalid()
		}
		return s.secret, nil
	}, jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}))
	if err != nil {
		// Map common jwt errors -> your domain errors
		if errorsIsJWTExpired(err) {
			return auth.TokenClaims{}, domain.ErrTokenExpired()
		}
		return auth.TokenClaims{}, domain.ErrTokenInvalid()
	}

	claims, ok := parsed.Claims.(*accessClaims)
	if !ok || !parsed.Valid {
		return auth.TokenClaims{}, domain.ErrTokenInvalid()
	}

	exp := time.Time{}
	if claims.ExpiresAt != nil {
		exp = claims.ExpiresAt.Time
	}

	return auth.TokenClaims{
		UserID: claims.UserID,
		Role:   claims.Role,
		Exp:    exp,
	}, nil
}

// local helper so you don't depend on jwt error types everywhere
func errorsIsJWTExpired(err error) bool {
	// jwt/v5 uses errors.Is(err, jwt.ErrTokenExpired) in many cases.
	return jwt.ErrTokenExpired != nil && errors.Is(err, jwt.ErrTokenExpired)
}
