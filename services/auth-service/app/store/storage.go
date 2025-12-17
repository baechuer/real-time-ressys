package store

import (
	"context"
	"database/sql"

	"github.com/baechuer/real-time-ressys/services/auth-service/app/models"
)

type Storage struct {
	Users interface {
		GetAll(ctx context.Context) ([]models.User, error)
		GetByID(ctx context.Context, id string) (*models.User, error)
		GetByEmail(ctx context.Context, email string) (*models.User, error)
		Create(ctx context.Context, user *models.User) error
		Update(ctx context.Context, user *models.User) error
		Delete(ctx context.Context, id string) error
	}
}

func NewStorage(db *sql.DB) Storage {
	return Storage{
		Users: &UsersStore{db: db},
	}
}
