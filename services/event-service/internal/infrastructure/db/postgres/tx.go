package postgres

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/baechuer/real-time-ressys/services/event-service/internal/application/event"
)

func (r *Repo) WithTx(ctx context.Context, fn func(tr event.TxEventRepo) error) error {
	tx, err := r.db.BeginTx(ctx, &sql.TxOptions{
		Isolation: sql.LevelReadCommitted,
		ReadOnly:  false,
	})
	if err != nil {
		return err
	}

	tr := &txRepo{tx: tx}

	defer func() {
		// Safety: in case fn panics, rollback to avoid leaked tx.
		if p := recover(); p != nil {
			_ = tx.Rollback()
			panic(p)
		}
	}()

	if err := fn(tr); err != nil {
		_ = tx.Rollback()
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit tx: %w", err)
	}
	return nil
}
