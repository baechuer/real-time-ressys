package security

import (
	"strconv"
	"strings"
	"time"

	"github.com/baechuer/real-time-ressys/services/auth-service/internal/application/auth"
	"github.com/baechuer/real-time-ressys/services/auth-service/internal/domain"
)

type StubSigner struct{}

func NewStubSigner() *StubSigner { return &StubSigner{} }

// Format: "stub.<userID>.<role>.<expUnix>"
func (s *StubSigner) SignAccessToken(userID string, role string, ttl time.Duration) (string, error) {
	exp := time.Now().Add(ttl).Unix()
	return "stub." + userID + "." + role + "." + strconv.FormatInt(exp, 10), nil
}

func (s *StubSigner) VerifyAccessToken(token string) (auth.TokenClaims, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 4 || parts[0] != "stub" {
		return auth.TokenClaims{}, domain.ErrTokenInvalid()
	}

	userID := parts[1]
	role := parts[2]

	expUnix, err := strconv.ParseInt(parts[3], 10, 64)
	if err != nil {
		return auth.TokenClaims{}, domain.ErrTokenInvalid()
	}

	exp := time.Unix(expUnix, 0)
	if time.Now().After(exp) {
		return auth.TokenClaims{}, domain.ErrTokenExpired()
	}

	return auth.TokenClaims{
		UserID: userID,
		Role:   role,
		Exp:    exp,
	}, nil
}
