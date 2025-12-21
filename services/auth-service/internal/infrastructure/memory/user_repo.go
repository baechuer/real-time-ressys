package memory

import (
	"context"
	"sync"

	"github.com/baechuer/real-time-ressys/services/auth-service/internal/domain"
)

type UserRepo struct {
	mu      sync.RWMutex
	byID    map[string]domain.User
	byEmail map[string]string // email -> userID
}

func NewUserRepo() *UserRepo {
	return &UserRepo{
		byID:    make(map[string]domain.User),
		byEmail: make(map[string]string),
	}
}

func (r *UserRepo) GetByEmail(ctx context.Context, email string) (domain.User, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	id, ok := r.byEmail[email]
	if !ok {
		return domain.User{}, domain.ErrUserNotFound()
	}
	u := r.byID[id]
	return u, nil
}

func (r *UserRepo) GetByID(ctx context.Context, id string) (domain.User, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	u, ok := r.byID[id]
	if !ok {
		return domain.User{}, domain.ErrUserNotFound()
	}
	return u, nil
}

func (r *UserRepo) Create(ctx context.Context, u domain.User) (domain.User, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.byEmail[u.Email]; exists {
		return domain.User{}, domain.ErrEmailAlreadyExists()
	}

	// ID should already be set by service/handler; but be defensive.
	if u.ID == "" {
		return domain.User{}, domain.ErrInternal(nil)
	}

	r.byID[u.ID] = u
	r.byEmail[u.Email] = u.ID
	return u, nil
}

func (r *UserRepo) UpdatePasswordHash(ctx context.Context, userID string, newHash string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	u, ok := r.byID[userID]
	if !ok {
		return domain.ErrUserNotFound()
	}
	u.PasswordHash = newHash
	r.byID[userID] = u
	return nil
}

func (r *UserRepo) SetEmailVerified(ctx context.Context, userID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	u, ok := r.byID[userID]
	if !ok {
		return domain.ErrUserNotFound()
	}
	u.EmailVerified = true
	r.byID[userID] = u
	return nil
}

func (r *UserRepo) LockUser(ctx context.Context, userID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	u, ok := r.byID[userID]
	if !ok {
		return domain.ErrUserNotFound()
	}
	u.Locked = true
	r.byID[userID] = u
	return nil
}
func (r *UserRepo) UnlockUser(ctx context.Context, userID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	u, ok := r.byID[userID]
	if !ok {
		return domain.ErrUserNotFound()
	}

	u.Locked = false
	r.byID[userID] = u
	return nil
}
func (r *UserRepo) SetRole(ctx context.Context, userID string, role string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	u, ok := r.byID[userID]
	if !ok {
		return domain.ErrUserNotFound()
	}

	u.Role = role
	r.byID[userID] = u
	return nil
}
func (r *UserRepo) CountByRole(ctx context.Context, role string) (int, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	n := 0
	for _, u := range r.byID {
		if u.Role == role {
			n++
		}
	}
	return n, nil
}
