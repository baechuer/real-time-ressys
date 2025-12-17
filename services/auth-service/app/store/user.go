package store

import (
	"context"
	"database/sql"

	"github.com/baechuer/real-time-ressys/services/auth-service/app/models"
)

type UsersStore struct {
	db *sql.DB
}

func (s *UsersStore) GetAll(ctx context.Context) ([]models.User, error) {
	query := `SELECT id, username, email, password_hash,
	is_email_verified, role_id, created_at FROM users`
	var users []models.User
	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var user models.User
		err := rows.Scan(
			&user.ID,
			&user.Username,
			&user.Email,
			&user.PasswordHash,
			&user.IsEmailVerified,
			&user.RoleID,
			&user.CreatedAt,
		)
		if err != nil {
			return nil, err
		}
		users = append(users, user)
	}
	return users, nil
}

func (s *UsersStore) GetByID(ctx context.Context, id string) (*models.User, error) {
	query := `SELECT id, username, email, password_hash,
	is_email_verified, role_id, created_at FROM users WHERE id = $1`
	var user models.User
	err := s.db.QueryRowContext(ctx, query, id).Scan(
		&user.ID,
		&user.Username,
		&user.Email,
		&user.PasswordHash,
		&user.IsEmailVerified,
		&user.RoleID,
		&user.CreatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, sql.ErrNoRows
		}
		return nil, err
	}
	return &user, nil
}

func (s *UsersStore) GetByEmail(ctx context.Context, email string) (*models.User, error) {
	query := `SELECT id, username, email, password_hash,
	is_email_verified, role_id, created_at FROM users WHERE email = $1`
	var user models.User
	err := s.db.QueryRowContext(ctx, query, email).Scan(
		&user.ID,
		&user.Username,
		&user.Email,
		&user.PasswordHash,
		&user.IsEmailVerified,
		&user.RoleID,
		&user.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (s *UsersStore) Create(ctx context.Context, user *models.User) error {
	query := `
	INSERT INTO users (username, email, password_hash, is_email_verified)
	VALUES ($1, $2, $3, $4)
	RETURNING id, created_at
	`

	err := s.db.QueryRowContext(ctx, query,
		user.Username,
		user.Email,
		user.PasswordHash,
		user.IsEmailVerified,
	).Scan(&user.ID, &user.CreatedAt)

	if err != nil {
		return err
	}

	return nil
}

func (s *UsersStore) Update(ctx context.Context, user *models.User) error {
	query := `UPDATE users SET username = $1, email = $2, password_hash = $3, is_email_verified = $4 WHERE id = $5`
	_, err := s.db.ExecContext(ctx, query,
		user.Username,
		user.Email,
		user.PasswordHash,
		user.IsEmailVerified,
		user.ID,
	)
	if err != nil {
		return err
	}
	return nil
}

func (s *UsersStore) Delete(ctx context.Context, id string) error {
	query := `DELETE FROM users WHERE id = $1`
	_, err := s.db.ExecContext(ctx, query, id)
	if err != nil {
		return err
	}
	return nil
}
func (s *UsersStore) UpdateRole(ctx context.Context, id int64, roleID int) error {
	query := `UPDATE users SET role_id = $1 WHERE id = $2`
	_, err := s.db.ExecContext(ctx, query, roleID, id)
	if err != nil {
		return err
	}
	return nil
}
