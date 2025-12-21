package config

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"strings"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
)

func NewDB(dsn string, debug bool) (*sql.DB, error) {

	if dsn == "" {
		return nil, fmt.Errorf("empty DB DSN")
	}
	// 🔎 DEBUG: parse DSN exactly as Go sees it
	u, err := url.Parse(dsn)
	if err != nil {
		return nil, fmt.Errorf("invalid DB DSN parse error: %w", err)
	}

	user := ""
	passLen := -1
	passBytes := []byte(nil)

	if u.User != nil {
		user = u.User.Username()
		if p, ok := u.User.Password(); ok {
			passLen = len(p)
			passBytes = []byte(p)
		}
	}

	fmt.Printf(
		"DEBUG DSN parsed: scheme=%q host=%q user=%q passLen=%d passBytes=%v db=%q rawLen=%d\n",
		u.Scheme,
		u.Host,
		user,
		passLen,
		passBytes,
		strings.Trim(u.Path, "/"),
		len(dsn),
	)

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
