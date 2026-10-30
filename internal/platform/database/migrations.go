package database

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"embed"
	"errors"
	"fmt"
)

//go:embed migrations/*.sql
var workspaceMigrations embed.FS

const migrationLock = 110004

func migrateWorkspace(ctx context.Context, tx *sql.Tx) error {
	if _, err := tx.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (
		version TEXT PRIMARY KEY, checksum TEXT NOT NULL, applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW())`); err != nil {
		return fmt.Errorf("create migration ledger: %w", err)
	}
	for _, version := range []string{"04_connected_workspace", "05_generation_runs", "06_diagram_generation", "07_diagram_mutations", "08_password_sessions"} {
		if err := applyWorkspaceMigration(ctx, tx, version); err != nil {
			return err
		}
	}
	return nil
}

func applyWorkspaceMigration(ctx context.Context, tx *sql.Tx, version string) error {
	source, err := workspaceMigrations.ReadFile("migrations/" + version + ".sql")
	if err != nil {
		return err
	}
	checksum := fmt.Sprintf("%x", sha256.Sum256(source))
	var applied string
	err = tx.QueryRowContext(ctx, "SELECT checksum FROM schema_migrations WHERE version = $1", version).Scan(&applied)
	if err == nil {
		if applied != checksum {
			return fmt.Errorf("migration %s checksum changed; add a new migration", version)
		}
		return nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("read migration ledger: %w", err)
	}
	if _, err := tx.ExecContext(ctx, string(source)); err != nil {
		return fmt.Errorf("apply %s: %w", version, err)
	}
	if _, err := tx.ExecContext(ctx, "INSERT INTO schema_migrations(version, checksum) VALUES ($1, $2)", version, checksum); err != nil {
		return fmt.Errorf("record %s: %w", version, err)
	}
	return nil
}

func backfillWorkspace(ctx context.Context, tx *sql.Tx) error {
	source, err := workspaceMigrations.ReadFile("migrations/04_connected_workspace_backfill.sql")
	if err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, "LOCK TABLE projects, chat_messages, diagram_versions IN SHARE ROW EXCLUSIVE MODE"); err != nil {
		return fmt.Errorf("lock legacy import sources: %w", err)
	}
	if _, err := tx.ExecContext(ctx, string(source)); err != nil {
		return fmt.Errorf("backfill connected workspace: %w", err)
	}
	return nil
}

func BackfillConnectedWorkspace(ctx context.Context, connection *sql.DB) error {
	if connection == nil {
		return fmt.Errorf("database connection is not configured")
	}
	tx, err := connection.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, "SELECT pg_advisory_xact_lock($1)", migrationLock); err != nil {
		return err
	}
	if err := backfillWorkspace(ctx, tx); err != nil {
		return err
	}
	return tx.Commit()
}
