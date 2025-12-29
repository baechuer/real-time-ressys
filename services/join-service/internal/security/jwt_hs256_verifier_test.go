package security_test

import (
	"testing"
	"time"

	"github.com/baechuer/real-time-ressys/services/join-service/internal/security"
	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
)

func signHS256(t *testing.T, secret []byte, claims security.TokenClaims, exp time.Time) string {
	t.Helper()

	jc := jwt.MapClaims{
		"uid":  claims.UserID,
		"role": claims.Role,
		"ver":  claims.Ver,
		"iat":  time.Now().Unix(),
		"exp":  exp.Unix(),
		"iss":  "auth-service",
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, jc)
	s, err := tok.SignedString(secret)
	if err != nil {
		t.Fatalf("sign token: %v", err)
	}
	return s
}

func TestHS256Verifier_VerifyAccessToken(t *testing.T) {
	secret := []byte("supersecret")
	v := security.NewHS256Verifier(string(secret))

	t.Run("valid token", func(t *testing.T) {
		token := signHS256(t, secret, security.TokenClaims{
			UserID: "u1", Role: "user", Ver: 1,
		}, time.Now().Add(1*time.Hour))

		claims, err := v.VerifyAccessToken(token)
		assert.NoError(t, err)
		assert.Equal(t, "u1", claims.UserID)
		assert.Equal(t, "user", claims.Role)
		assert.Equal(t, int64(1), claims.Ver)
	})

	t.Run("expired token", func(t *testing.T) {
		token := signHS256(t, secret, security.TokenClaims{
			UserID: "u1", Role: "user", Ver: 1,
		}, time.Now().Add(-1*time.Minute))

		_, err := v.VerifyAccessToken(token)
		assert.ErrorIs(t, err, security.ErrTokenExpired)
	})

	t.Run("wrong signature", func(t *testing.T) {
		token := signHS256(t, []byte("othersecret"), security.TokenClaims{
			UserID: "u1", Role: "user", Ver: 1,
		}, time.Now().Add(1*time.Hour))

		_, err := v.VerifyAccessToken(token)
		assert.ErrorIs(t, err, security.ErrTokenInvalid)
	})

	t.Run("malformed token", func(t *testing.T) {
		_, err := v.VerifyAccessToken("not.a.jwt")
		assert.ErrorIs(t, err, security.ErrTokenInvalid)
	})

	t.Run("wrong algorithm", func(t *testing.T) {
		jc := jwt.MapClaims{
			"uid": "u1", "role": "user", "ver": 1,
			"exp": time.Now().Add(1 * time.Hour).Unix(),
		}
		tok := jwt.NewWithClaims(jwt.SigningMethodHS512, jc)
		s, _ := tok.SignedString(secret)

		_, err := v.VerifyAccessToken(s)
		assert.ErrorIs(t, err, security.ErrTokenInvalid)
	})
}
