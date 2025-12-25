package infra

import (
	"context"
	"database/sql"
	"time"

	_ "github.com/lib/pq"
)

func OpenDB(dbURL string) (*sql.DB, error) {
	return sql.Open("postgres", dbURL)
}

func PingDB(db *sql.DB) error {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	return db.PingContext(ctx)
}

func ResetEvents(db *sql.DB) error {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	_, err := db.ExecContext(ctx, `TRUNCATE TABLE events`)
	return err
}
