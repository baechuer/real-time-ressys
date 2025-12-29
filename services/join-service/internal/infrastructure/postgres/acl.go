package postgres

import (
	"context"
	"errors"

	"github.com/baechuer/real-time-ressys/services/join-service/internal/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func (r *Repository) GetEventOwnerID(ctx context.Context, eventID uuid.UUID) (uuid.UUID, error) {
	var owner uuid.UUID
	err := r.pool.QueryRow(ctx, `SELECT owner_id FROM events WHERE id = $1`, eventID).Scan(&owner)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return uuid.UUID{}, domain.ErrEventNotFound
		}
		return uuid.UUID{}, err
	}
	return owner, nil
}
