package postgres

import (
	"context"
	"fmt"
	"io/fs"
	"sort"
	"strings"

	"github.com/gliedabrennung/sedna/internal/common/logger"
	"github.com/jackc/pgx/v5/pgxpool"
)

const migrationLockKey int64 = 8891234509876

const migrationsTable = `
CREATE TABLE IF NOT EXISTS schema_migrations (
	version     TEXT        PRIMARY KEY,
	applied_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
)`

func Migrate(ctx context.Context, pool *pgxpool.Pool, src fs.FS) error {
	conn, err := pool.Acquire(ctx)
	if err != nil {
		return fmt.Errorf("migrate: acquire connection: %w", err)
	}
	defer conn.Release()

	if _, err := conn.Exec(ctx, `SELECT pg_advisory_lock($1)`, migrationLockKey); err != nil {
		return fmt.Errorf("migrate: acquire lock: %w", err)
	}
	defer func() {
		if _, err := conn.Exec(context.WithoutCancel(ctx),
			`SELECT pg_advisory_unlock($1)`, migrationLockKey); err != nil {
			logger.CtxErrorf(ctx, "release migration lock: %v", err)
		}
	}()

	if _, err := pool.Exec(ctx, migrationsTable); err != nil {
		return fmt.Errorf("migrate: create migrations table: %w", err)
	}

	applied, err := appliedVersions(ctx, pool)
	if err != nil {
		return err
	}

	files, err := migrationFiles(src)
	if err != nil {
		return err
	}

	count := 0
	for _, name := range files {
		if applied[name] {
			continue
		}

		body, err := fs.ReadFile(src, name)
		if err != nil {
			return fmt.Errorf("migrate: read %s: %w", name, err)
		}

		if err := applyOne(ctx, pool, name, string(body)); err != nil {
			return err
		}
		logger.CtxInfof(ctx, "applied migration %s", name)
		count++
	}

	if count == 0 {
		logger.CtxInfof(ctx, "database schema is up to date")
	}
	return nil
}

func applyOne(ctx context.Context, pool *pgxpool.Pool, name, body string) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("migrate: begin %s: %w", name, err)
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, body); err != nil {
		return fmt.Errorf("migrate: apply %s: %w", name, err)
	}
	if _, err := tx.Exec(ctx,
		`INSERT INTO schema_migrations (version) VALUES ($1)`, name); err != nil {
		return fmt.Errorf("migrate: record %s: %w", name, err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("migrate: commit %s: %w", name, err)
	}
	return nil
}

func appliedVersions(ctx context.Context, pool *pgxpool.Pool) (map[string]bool, error) {
	rows, err := pool.Query(ctx, `SELECT version FROM schema_migrations`)
	if err != nil {
		return nil, fmt.Errorf("migrate: read applied versions: %w", err)
	}
	defer rows.Close()

	applied := make(map[string]bool)
	for rows.Next() {
		var version string
		if err := rows.Scan(&version); err != nil {
			return nil, fmt.Errorf("migrate: scan version: %w", err)
		}
		applied[version] = true
	}
	return applied, rows.Err()
}

func migrationFiles(src fs.FS) ([]string, error) {
	entries, err := fs.ReadDir(src, ".")
	if err != nil {
		return nil, fmt.Errorf("migrate: list migrations: %w", err)
	}

	var files []string
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".sql") {
			continue
		}
		files = append(files, e.Name())
	}
	sort.Strings(files)
	return files, nil
}
