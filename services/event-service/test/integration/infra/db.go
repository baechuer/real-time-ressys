package infra

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
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

func ApplyMigrations(db *sql.DB, migrationsDir string) error {
	absDir, _ := filepath.Abs(migrationsDir)
	files, err := os.ReadDir(migrationsDir)
	if err != nil {
		return fmt.Errorf("read migrations dir %q (abs: %q): %w", migrationsDir, absDir, err)
	}

	// Sort by name to ensure order (e.g. 001, 002...)
	sort.Slice(files, func(i, j int) bool {
		return files[i].Name() < files[j].Name()
	})

	var appliedCount int
	for _, f := range files {
		if f.IsDir() || !strings.HasSuffix(f.Name(), ".sql") {
			continue
		}

		content, err := os.ReadFile(filepath.Join(migrationsDir, f.Name()))
		if err != nil {
			return fmt.Errorf("read migration %s: %w", f.Name(), err)
		}

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if _, err := db.ExecContext(ctx, string(content)); err != nil {
			return fmt.Errorf("apply migration %s: %w", f.Name(), err)
		}
		appliedCount++
	}

	if appliedCount == 0 {
		return fmt.Errorf("no migration files found in %q (abs: %q)", migrationsDir, absDir)
	}

	return nil
}

func WipeDB(db *sql.DB) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	// Drop public schema and recreate it to remove all tables, types, functions
	if _, err := db.ExecContext(ctx, "DROP SCHEMA public CASCADE; CREATE SCHEMA public;"); err != nil {
		return err
	}
	return nil
}
