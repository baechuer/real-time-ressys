package postgres

import (
	"context"
	"database/sql"
	"time"

	"github.com/baechuer/real-time-ressys/services/event-service/internal/application/event"
	"github.com/baechuer/real-time-ressys/services/event-service/internal/domain"
)

type txRepo struct {
	tx *sql.Tx
}

const insertOutboxSQL = `
INSERT INTO event_outbox (
  message_id, routing_key, body, created_at
) VALUES ($1, $2, $3::jsonb, $4)
`

const selectEventForUpdateSQL = `
SELECT id, owner_id, title, description, city, category,
       start_time, end_time, capacity, status,
       published_at, canceled_at, created_at, updated_at
FROM events WHERE id = $1
FOR UPDATE
`

func (r *txRepo) GetByIDForUpdate(ctx context.Context, id string) (*domain.Event, error) {
	row := r.tx.QueryRowContext(ctx, selectEventForUpdateSQL, id)

	var e domain.Event
	var status string
	err := row.Scan(
		&e.ID, &e.OwnerID, &e.Title, &e.Description, &e.City, &e.Category,
		&e.StartTime, &e.EndTime, &e.Capacity, &status,
		&e.PublishedAt, &e.CanceledAt, &e.CreatedAt, &e.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	e.Status = domain.EventStatus(status)
	return &e, nil
}

func (r *txRepo) Update(ctx context.Context, e *domain.Event) error {
	_, err := r.tx.ExecContext(ctx, updateEventSQL,
		e.ID, e.Title, e.Description, e.City, e.Category,
		e.StartTime, e.EndTime, e.Capacity, string(e.Status),
		e.PublishedAt, e.CanceledAt, e.UpdatedAt,
	)
	return err
}

func (r *txRepo) InsertOutbox(ctx context.Context, msg event.OutboxMessage) error {
	// Store JSON as text cast to jsonb for lib/pq compatibility.
	_, err := r.tx.ExecContext(ctx, insertOutboxSQL,
		msg.MessageID,
		msg.RoutingKey,
		string(msg.Body),
		msg.CreatedAt.UTC(),
	)
	return err
}

// --- outbox worker helpers (non-tx) ---

type outboxRow struct {
	ID         int64
	MessageID  string
	RoutingKey string
	Body       []byte
}

const selectOutboxBatchForUpdateSQL = `
SELECT id, message_id, routing_key, body
FROM event_outbox
WHERE sent_at IS NULL
ORDER BY id ASC
LIMIT $1
FOR UPDATE SKIP LOCKED
`

const markOutboxSentSQL = `
UPDATE event_outbox
SET sent_at = $2,
    attempts = attempts + 1,
    last_error = NULL
WHERE id = $1
`

const markOutboxFailedSQL = `
UPDATE event_outbox
SET attempts = attempts + 1,
    last_error = $2
WHERE id = $1
`

// StartOutboxWorker starts a polling worker that publishes pending outbox rows to RabbitMQ.
// This is intentionally simple (polling + SKIP LOCKED) and safe under concurrency.
func (r *Repo) StartOutboxWorker(ctx context.Context, pub event.EventPublisher) {
	go func() {
		ticker := time.NewTicker(1 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				r.processOutboxBatch(ctx, pub, 50)
			}
		}
	}()
}

func (r *Repo) processOutboxBatch(ctx context.Context, pub event.EventPublisher, limit int) {
	if limit <= 0 {
		limit = 50
	}

	// Short operation window to avoid pile-ups.
	opCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	tx, err := r.db.BeginTx(opCtx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return
	}
	defer func() { _ = tx.Rollback() }()

	rows, err := tx.QueryContext(opCtx, selectOutboxBatchForUpdateSQL, limit)
	if err != nil {
		return
	}
	defer rows.Close()

	var batch []outboxRow
	for rows.Next() {
		var r outboxRow
		if err := rows.Scan(&r.ID, &r.MessageID, &r.RoutingKey, &r.Body); err != nil {
			return
		}
		batch = append(batch, r)
	}
	if err := rows.Err(); err != nil {
		return
	}
	if len(batch) == 0 {
		_ = tx.Commit()
		return
	}

	now := time.Now().UTC()

	for _, m := range batch {
		// publish each message and mark state inside the same tx holding the lock.
		if err := pub.PublishEvent(opCtx, m.RoutingKey, m.MessageID, m.Body); err != nil {
			_, _ = tx.ExecContext(opCtx, markOutboxFailedSQL, m.ID, err.Error())
			continue
		}
		_, _ = tx.ExecContext(opCtx, markOutboxSentSQL, m.ID, now)
	}

	_ = tx.Commit()
}
