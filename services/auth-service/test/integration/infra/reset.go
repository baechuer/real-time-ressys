//go:build integration

package infra

import (
	"context"
	"database/sql"
	"fmt"

	goredis "github.com/redis/go-redis/v9"
)

func ResetAll(ctx context.Context, db *sql.DB, rdb *goredis.Client) error {
	if err := ResetPostgres(ctx, db); err != nil {
		return err
	}
	if err := ResetRedis(ctx, rdb); err != nil {
		return err
	}
	return nil
}

func ResetPostgres(ctx context.Context, db *sql.DB) error {
	// 清空 users（你如果有更多表，自己加）
	_, err := db.ExecContext(ctx, `TRUNCATE TABLE users RESTART IDENTITY CASCADE;`)
	if err != nil {
		// 如果 users 还没建好，先忽略，让 EnsureAuthSchema 建表
		return fmt.Errorf("reset postgres: %w", err)
	}
	return nil
}

func ResetRedis(ctx context.Context, rdb *goredis.Client) error {
	// integration tests: 直接 FLUSHDB
	return rdb.FlushDB(ctx).Err()
}
