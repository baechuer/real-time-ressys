package config

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
)

func NewDB(dsn string, debug bool) (*sql.DB, error) {

	if dsn == "" {
		return nil, fmt.Errorf("empty DB DSN")
	}
	// ---------------- actual connection ----------------
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, err
	}

	// sensible defaults (env-ify later if you want)
	db.SetMaxOpenConns(20)
	db.SetMaxIdleConns(10)
	db.SetConnMaxIdleTime(5 * time.Minute)
	db.SetConnMaxLifetime(60 * time.Minute)

	// verify connectivity early (fail fast)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, err
	}

	if debug {
		// prove we're connected to expected server/user/db (no secrets)
		var who, dbname, addr, ver string
		_ = db.QueryRowContext(ctx, "SELECT current_user").Scan(&who)
		_ = db.QueryRowContext(ctx, "SELECT current_database()").Scan(&dbname)
		_ = db.QueryRowContext(ctx, "SELECT inet_server_addr()::text").Scan(&addr)
		_ = db.QueryRowContext(ctx, "SHOW server_version").Scan(&ver)

		fmt.Printf("DB CONNECTED: user=%s db=%s server_addr=%s version=%s\n", who, dbname, addr, ver)
	}

	return db, nil
}
