//go:build integration

package infra

import (
	"context"
	"database/sql"
)

// EnsureAuthSchema：
// - 建表 users（若不存在）
// - 补齐 integration tests 会触发的字段（email_verified/locked/password_changed_at/token_version 等）
//
// 你真实项目如果有 migrations，最终建议：
// ✅ integration 环境里跑 migration
// 这里先保证你“能跑通 tests”，并且不会再手动 ALTER 一路补列。
func EnsureAuthSchema(ctx context.Context, db *sql.DB) error {
	// 1) create table if not exists (宽松字段集合)
	_, err := db.ExecContext(ctx, `
CREATE TABLE IF NOT EXISTS users (
  id TEXT PRIMARY KEY,
  email TEXT NOT NULL UNIQUE,
  password_hash TEXT NOT NULL,

  role TEXT NOT NULL DEFAULT 'user',
  token_version BIGINT NOT NULL DEFAULT 0,

  email_verified BOOLEAN NOT NULL DEFAULT FALSE,
  locked BOOLEAN NOT NULL DEFAULT FALSE,

  password_changed_at TIMESTAMPTZ NULL,

  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
`)
	if err != nil {
		return err
	}

	// 2) patch columns if missing (幂等)
	stmts := []string{
		`ALTER TABLE users ADD COLUMN IF NOT EXISTS role TEXT NOT NULL DEFAULT 'user';`,
		`ALTER TABLE users ADD COLUMN IF NOT EXISTS token_version BIGINT NOT NULL DEFAULT 0;`,
		`ALTER TABLE users ADD COLUMN IF NOT EXISTS email_verified BOOLEAN NOT NULL DEFAULT FALSE;`,
		`ALTER TABLE users ADD COLUMN IF NOT EXISTS locked BOOLEAN NOT NULL DEFAULT FALSE;`,
		`ALTER TABLE users ADD COLUMN IF NOT EXISTS password_changed_at TIMESTAMPTZ NULL;`,
		`ALTER TABLE users ADD COLUMN IF NOT EXISTS created_at TIMESTAMPTZ NOT NULL DEFAULT NOW();`,
		`ALTER TABLE users ADD COLUMN IF NOT EXISTS updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW();`,
		`ALTER TABLE users ADD COLUMN IF NOT EXISTS password_hash TEXT;`,
	}

	for _, s := range stmts {
		if _, e := db.ExecContext(ctx, s); e != nil {
			return e
		}
	}

	return nil
}
