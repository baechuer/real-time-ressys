package memory

import (
	"context"
	"sync"
	"time"

	"github.com/baechuer/real-time-ressys/services/auth-service/internal/application/auth"
	"github.com/baechuer/real-time-ressys/services/auth-service/internal/domain"
)

type OneTimeTokenStore struct {
	mu sync.RWMutex
	// kind|token -> userID
	data map[string]string
}

func NewOneTimeTokenStore() *OneTimeTokenStore {
	return &OneTimeTokenStore{data: make(map[string]string)}
}

func key(kind auth.OneTimeTokenKind, token string) string { return string(kind) + "|" + token }

func (s *OneTimeTokenStore) Save(ctx context.Context, kind auth.OneTimeTokenKind, token string, userID string, ttl time.Duration) error {
	_ = ttl // ignore ttl in MVP
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data[key(kind, token)] = userID
	return nil
}

func (s *OneTimeTokenStore) Consume(ctx context.Context, kind auth.OneTimeTokenKind, token string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	k := key(kind, token)
	uid, ok := s.data[k]
	if !ok {
		if kind == auth.TokenVerifyEmail {
			return "", domain.ErrVerifyTokenNotFound()
		}
		return "", domain.ErrResetTokenNotFound()
	}
	delete(s.data, k)
	return uid, nil
}

func (s *OneTimeTokenStore) Peek(ctx context.Context, kind auth.OneTimeTokenKind, token string) (string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	k := key(kind, token)
	uid, ok := s.data[k]
	if !ok {
		if kind == auth.TokenVerifyEmail {
			return "", domain.ErrVerifyTokenNotFound()
		}
		return "", domain.ErrResetTokenNotFound()
	}
	return uid, nil
}
