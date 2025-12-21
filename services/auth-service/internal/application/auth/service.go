package auth

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"time"

	"github.com/baechuer/real-time-ressys/services/auth-service/internal/domain"
)

type Service struct {
	users    UserRepo
	hasher   PasswordHasher
	signer   TokenSigner
	sessions SessionStore
	ott      OneTimeTokenStore
	pub      EventPublisher

	accessTTL  time.Duration
	refreshTTL time.Duration
	audit      func(action string, fields map[string]string)

	// URLs used to build links sent via email-service
	verifyEmailBaseURL   string // e.g. https://frontend/verify-email?token=
	passwordResetBaseURL string // e.g. https://frontend/reset-password?token=
	verifyEmailTTL       time.Duration
	passwordResetTTL     time.Duration
}

type Config struct {
	AccessTTL             time.Duration
	RefreshTTL            time.Duration
	VerifyEmailBaseURL    string
	PasswordResetBaseURL  string
	VerifyEmailTokenTTL   time.Duration
	PasswordResetTokenTTL time.Duration
}

func NewService(
	users UserRepo,
	hasher PasswordHasher,
	signer TokenSigner,
	sessions SessionStore,
	ott OneTimeTokenStore,
	pub EventPublisher,
	cfg Config,
) *Service {
	auditFn := func(string, map[string]string) {}
	verifyTTL := cfg.VerifyEmailTokenTTL
	if verifyTTL <= 0 {
		verifyTTL = 24 * time.Hour
	}
	resetTTL := cfg.PasswordResetTokenTTL
	if resetTTL <= 0 {
		resetTTL = 30 * time.Minute
	}
	return &Service{
		users:    users,
		hasher:   hasher,
		signer:   signer,
		sessions: sessions,
		ott:      ott,
		pub:      pub,
		audit:    auditFn,

		accessTTL:  cfg.AccessTTL,
		refreshTTL: cfg.RefreshTTL,

		verifyEmailBaseURL:   cfg.VerifyEmailBaseURL,
		passwordResetBaseURL: cfg.PasswordResetBaseURL,
		verifyEmailTTL:       verifyTTL,
		passwordResetTTL:     resetTTL,
	}
}

// AuthTokens is the common token output for handlers/DTO mapping.
type AuthTokens struct {
	AccessToken  string
	RefreshToken string // if you store refresh in HttpOnly cookie, you may ignore this in handler
	ExpiresIn    int64  // seconds
	TokenType    string // "Bearer"
}

type RegisterResult struct {
	User   domain.User
	Tokens AuthTokens
}

type LoginResult struct {
	User   domain.User
	Tokens AuthTokens
}

func (s *Service) WithAudit(fn func(action string, fields map[string]string)) *Service {
	if fn != nil {
		s.audit = fn
	}
	return s
}

// issueTokens issues an access token + refresh token for a user.
func (s *Service) issueTokens(ctx context.Context, userID, role string) (AuthTokens, error) {
	access, err := s.signer.SignAccessToken(userID, role, s.accessTTL)
	if err != nil {
		// TODO: ensure domain has this constructor
		return AuthTokens{}, domain.ErrTokenSignFailed(err)
	}

	refresh, err := s.sessions.CreateRefreshToken(ctx, userID, s.refreshTTL)
	if err != nil {
		return AuthTokens{}, err
	}

	return AuthTokens{
		AccessToken:  access,
		RefreshToken: refresh,
		TokenType:    "Bearer",
		ExpiresIn:    int64(s.accessTTL.Seconds()),
	}, nil
}

// newOpaqueToken returns a URL-safe opaque token.
func newOpaqueToken(bytesLen int) (string, error) {
	if bytesLen <= 0 {
		return "", errors.New("invalid token length")
	}
	b := make([]byte, bytesLen)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}
