package postgres

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/baechuer/real-time-ressys/services/auth-service/internal/domain"
)

type fakeSeederHasher struct {
	mu    sync.Mutex
	err   error
	calls int
}

func (h *fakeSeederHasher) Hash(pw string) (string, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.calls++
	if h.err != nil {
		return "", h.err
	}
	return "HASH(" + pw + ")", nil
}

type fakeSeederRepo struct {
	mu      sync.Mutex
	created []domain.User
	errOnce error
	errCnt  int
}

func (r *fakeSeederRepo) Create(ctx context.Context, u domain.User) (domain.User, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.errOnce != nil && r.errCnt == 0 {
		r.errCnt++
		return domain.User{}, r.errOnce // simulate duplicate/any failure once
	}
	r.created = append(r.created, u)
	return u, nil
}

func TestSeedUsers_Creates3Users_WithVerifiedTrue(t *testing.T) {
	t.Parallel()

	repo := &fakeSeederRepo{}
	hasher := &fakeSeederHasher{}

	SeedUsers(context.Background(), repo, hasher)

	if hasher.calls != 3 {
		t.Fatalf("expected hasher called 3 times, got %d", hasher.calls)
	}

	if len(repo.created) != 3 {
		t.Fatalf("expected 3 users created, got %d", len(repo.created))
	}

	// validate each seed user invariants
	for _, u := range repo.created {
		if u.ID == "" {
			t.Fatalf("expected non-empty id")
		}
		if u.Email == "" {
			t.Fatalf("expected non-empty email")
		}
		if u.PasswordHash == "" {
			t.Fatalf("expected non-empty hash")
		}
		if u.Role == "" {
			t.Fatalf("expected non-empty role")
		}
		if !u.EmailVerified {
			t.Fatalf("expected EmailVerified=true")
		}
		if u.Locked {
			t.Fatalf("expected Locked=false")
		}
	}
}

func TestSeedUsers_IgnoresCreateErrors_RestStillSeeded(t *testing.T) {
	t.Parallel()

	repo := &fakeSeederRepo{errOnce: errors.New("duplicate")}
	hasher := &fakeSeederHasher{}

	SeedUsers(context.Background(), repo, hasher)

	// one create failed (ignored), so only 2 persisted in our fake
	if len(repo.created) != 2 {
		t.Fatalf("expected 2 successful creates after one error, got %d", len(repo.created))
	}
}

func TestSeedUsers_HashFail_SkipsThatUser(t *testing.T) {
	t.Parallel()

	repo := &fakeSeederRepo{}
	hasher := &fakeSeederHasher{err: errors.New("hash fail")}

	SeedUsers(context.Background(), repo, hasher)

	// all three hash fail => no users created
	if len(repo.created) != 0 {
		t.Fatalf("expected 0 created when hash always fails, got %d", len(repo.created))
	}
}
