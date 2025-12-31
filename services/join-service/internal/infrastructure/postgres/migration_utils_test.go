//go:build integration
// +build integration

package postgres_test

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func WipeDB(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// 1. Drop all tables in public schema
	_, err := pool.Exec(ctx, `
		DO $$ 
		DECLARE 
			r RECORD; 
		BEGIN 
			FOR r IN (SELECT tablename FROM pg_tables WHERE schemaname = 'public') LOOP 
				EXECUTE 'DROP TABLE IF EXISTS ' || quote_ident(r.tablename) || ' CASCADE'; 
			END LOOP; 
		END $$;
	`)
	if err != nil {
		t.Fatalf("wipe db (tables): %v", err)
	}

	// 2. Drop all custom types (ENUMs)
	// We join pg_type and pg_namespace to find types in 'public' schema
	_, err = pool.Exec(ctx, `
		DO $$
		DECLARE
			r RECORD;
		BEGIN
			FOR r IN (
				SELECT t.typname 
				FROM pg_type t 
				JOIN pg_namespace n ON t.typnamespace = n.oid 
				WHERE n.nspname = 'public' AND t.typtype = 'e'
			) LOOP
				EXECUTE 'DROP TYPE IF EXISTS ' || quote_ident(r.typname) || ' CASCADE';
			END LOOP;
		END $$;
	`)
	if err != nil {
		t.Fatalf("wipe db (types): %v", err)
	}
}

func ApplyMigrations(t *testing.T, pool *pgxpool.Pool, migrationsDir string) {
	t.Helper()
	absDir, _ := filepath.Abs(migrationsDir)
	entries, err := os.ReadDir(migrationsDir)
	if err != nil {
		t.Fatalf("read migrations dir %q (abs: %q): %v", migrationsDir, absDir, err)
	}

	var files []os.DirEntry
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".sql") {
			files = append(files, e)
		}
	}

	if len(files) == 0 {
		t.Fatalf("no migration files found in %q", absDir)
	}

	sort.Slice(files, func(i, j int) bool {
		return files[i].Name() < files[j].Name()
	})

	for _, f := range files {
		content, err := os.ReadFile(filepath.Join(migrationsDir, f.Name()))
		if err != nil {
			t.Fatalf("read migration %s: %v", f.Name(), err)
		}

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if _, err := pool.Exec(ctx, string(content)); err != nil {
			t.Fatalf("apply migration %s: %v", f.Name(), err)
		}
	}
}
