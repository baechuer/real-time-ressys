package memory

import (
	"context"
	"sync"
	"time"

	"github.com/baechuer/real-time-ressys/services/auth-service/internal/domain"
)

type SessionStore struct {
	mu sync.RWMutex
	// refreshToken -> userID
	tokenToUser map[string]string
	// userID -> set(refreshToken)
	userTokens map[string]map[string]struct{}
}

func NewSessionStore() *SessionStore {
	return &SessionStore{
		tokenToUser: make(map[string]string),
		userTokens:  make(map[string]map[string]struct{}),
	}
}

func (s *SessionStore) CreateRefreshToken(ctx context.Context, userID string, ttl time.Duration) (string, error) {
	_ = ttl // in-memory MVP: ignore ttl
	tok, err := newOpaqueToken(32)
	if err != nil {
		return "", domain.ErrRandomFailed(err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.tokenToUser[tok] = userID
	if s.userTokens[userID] == nil {
		s.userTokens[userID] = make(map[string]struct{})
	}
	s.userTokens[userID][tok] = struct{}{}

	return tok, nil
}

func (s *SessionStore) GetUserIDByRefreshToken(ctx context.Context, token string) (string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	uid, ok := s.tokenToUser[token]
	if !ok {
		return "", domain.ErrRefreshTokenInvalid()
	}
	return uid, nil
}

func (s *SessionStore) RotateRefreshToken(ctx context.Context, oldToken string, ttl time.Duration) (string, error) {
	uid, err := s.GetUserIDByRefreshToken(ctx, oldToken)
	if err != nil {
		return "", err
	}

	// revoke old
	_ = s.RevokeRefreshToken(ctx, oldToken)

	// create new
	return s.CreateRefreshToken(ctx, uid, ttl)
}

func (s *SessionStore) RevokeRefreshToken(ctx context.Context, token string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	uid, ok := s.tokenToUser[token]
	if !ok {
		return nil // idempotent
	}
	delete(s.tokenToUser, token)
	if set := s.userTokens[uid]; set != nil {
		delete(set, token)
		if len(set) == 0 {
			delete(s.userTokens, uid)
		}
	}
	return nil
}

func (s *SessionStore) RevokeAll(ctx context.Context, userID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	set := s.userTokens[userID]
	for tok := range set {
		delete(s.tokenToUser, tok)
	}
	delete(s.userTokens, userID)
	return nil
}

// local helper (same as in service.go but duplicated to avoid package import cycle)
func newOpaqueToken(bytesLen int) (string, error) {
	return domainInternalOpaqueToken(bytesLen)
}
