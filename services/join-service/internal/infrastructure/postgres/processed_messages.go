// PATH: services/join-service/internal/infrastructure/postgres/processed_messages.go
package postgres

import (
	"context"
	"strings"

	"github.com/jackc/pgx/v5"
)

// TryMarkProcessed inserts (message_id, handler_name) once.
// Returns:
//
//	ok=true  -> first time processed
//	ok=false -> duplicate delivery (already processed)
func (r *Repository) TryMarkProcessed(ctx context.Context, messageID, handlerName string) (ok bool, err error) {
	messageID = strings.TrimSpace(messageID)
	handlerName = strings.TrimSpace(handlerName)

	if messageID == "" {
		// Caller should not rely on this for correctness; message_id must exist for safe dedupe.
		return true, nil
	}
	if handlerName == "" {
		handlerName = "unknown"
	}

	tag, err := r.pool.Exec(ctx, `
		INSERT INTO processed_messages (message_id, handler_name)
		VALUES ($1, $2)
		ON CONFLICT DO NOTHING
	`, messageID, handlerName)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() == 1, nil
}

// TryMarkProcessedTx is the transactional variant.
// MUST be used when you want "dedupe fence + side effects" to be atomic.
func (r *Repository) TryMarkProcessedTx(ctx context.Context, tx pgx.Tx, messageID, handlerName string) (ok bool, err error) {
	messageID = strings.TrimSpace(messageID)
	handlerName = strings.TrimSpace(handlerName)

	if messageID == "" {
		return true, nil
	}
	if handlerName == "" {
		handlerName = "unknown"
	}

	tag, err := tx.Exec(ctx, `
		INSERT INTO processed_messages (message_id, handler_name)
		VALUES ($1, $2)
		ON CONFLICT DO NOTHING
	`, messageID, handlerName)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() == 1, nil
}

// ProcessOnce runs fn inside a DB transaction guarded by processed_messages "idempotency fence".
// - If duplicate (already processed): fn is NOT executed, and returns processed=false, err=nil.
// - If fn fails: tx rolls back => processed marker does NOT persist => message can be retried.
func (r *Repository) ProcessOnce(
	ctx context.Context,
	messageID, handlerName string,
	fn func(tx pgx.Tx) error,
) (processed bool, err error) {
	messageID = strings.TrimSpace(messageID)
	handlerName = strings.TrimSpace(handlerName)

	// If caller failed to provide message_id, we cannot safely dedupe.
	// Still run fn (best effort) instead of dropping.
	if messageID == "" {
		tx, txErr := r.pool.Begin(ctx)
		if txErr != nil {
			return false, txErr
		}
		defer func() { _ = tx.Rollback(ctx) }()

		if err := fn(tx); err != nil {
			return false, err
		}
		if err := tx.Commit(ctx); err != nil {
			return false, err
		}
		return true, nil
	}

	if handlerName == "" {
		handlerName = "unknown"
	}

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return false, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	first, err := r.TryMarkProcessedTx(ctx, tx, messageID, handlerName)
	if err != nil {
		return false, err
	}
	if !first {
		// Duplicate delivery: don't execute fn, don't mutate anything.
		return false, nil
	}

	if err := fn(tx); err != nil {
		return false, err
	}

	if err := tx.Commit(ctx); err != nil {
		return false, err
	}
	return true, nil
}
