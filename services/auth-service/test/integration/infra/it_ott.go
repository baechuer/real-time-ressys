//go:build integration

package infra

import (
	"context"
	"sync"
	"time"

	"github.com/baechuer/real-time-ressys/services/auth-service/internal/application/auth"
)

type inMemoryOTT struct {
	mu sync.Mutex
	m  map[string]ottVal
}

type ottVal struct {
	userID string
	exp    time.Time
}

func NewInMemoryOTT() *inMemoryOTT {
	return &inMemoryOTT{m: make(map[string]ottVal)}
}

// Save matches auth.OneTimeTokenStore.
// If your interface later changes the signature, paste the exact interface and I'll adjust.
func (s *inMemoryOTT) Save(ctx context.Context, kind auth.OneTimeTokenKind, token string, userID string, ttl time.Duration) error {
	_ = ctx
	s.mu.Lock()
	defer s.mu.Unlock()

	key := string(kind) + ":" + token
	s.m[key] = ottVal{userID: userID, exp: time.Now().Add(ttl)}
	return nil
}

// Put kept as a compatibility alias (won't hurt even if not in interface).
func (s *inMemoryOTT) Put(ctx context.Context, kind auth.OneTimeTokenKind, token string, userID string, ttl time.Duration) error {
	return s.Save(ctx, kind, token, userID, ttl)
}

// Peek returns the userID without consuming the token.
// If token not found or expired -> return "", nil.
func (s *inMemoryOTT) Peek(ctx context.Context, kind auth.OneTimeTokenKind, token string) (string, error) {
	_ = ctx
	s.mu.Lock()
	defer s.mu.Unlock()

	key := string(kind) + ":" + token
	v, ok := s.m[key]
	if !ok {
		return "", nil
	}
	if time.Now().After(v.exp) {
		delete(s.m, key)
		return "", nil
	}
	return v.userID, nil
}

// Consume returns the userID and deletes the token.
// If token not found or expired -> return "", nil.
func (s *inMemoryOTT) Consume(ctx context.Context, kind auth.OneTimeTokenKind, token string) (string, error) {
	_ = ctx
	s.mu.Lock()
	defer s.mu.Unlock()

	key := string(kind) + ":" + token
	v, ok := s.m[key]
	if !ok {
		return "", nil
	}
	delete(s.m, key)

	if time.Now().After(v.exp) {
		return "", nil
	}
	return v.userID, nil
}
