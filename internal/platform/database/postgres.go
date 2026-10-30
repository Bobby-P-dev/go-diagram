package database

import (
	"context"
	"database/sql"
	_ "embed"
	"fmt"
	"log"
	"time"

	_ "github.com/lib/pq"
)

//go:embed init.sql
var initSQL string

type Config struct {
	Host     string
	Port     string
	User     string
	Password string
	DBName   string
	SSLMode  string
}

func Connect(ctx context.Context, cfg Config) (*sql.DB, error) {
	return connect(ctx, cfg, sql.Open)
}

func connect(ctx context.Context, cfg Config, open func(string, string) (*sql.DB, error)) (*sql.DB, error) {
	dsn := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=%s", cfg.Host, cfg.Port, cfg.User, cfg.Password, cfg.DBName, cfg.SSLMode)
	connection, err := open("postgres", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open database connection: %w", err)
	}
	connection.SetMaxOpenConns(25)
	connection.SetMaxIdleConns(10)
	connection.SetConnMaxLifetime(5 * time.Minute)
	if err := connection.PingContext(ctx); err != nil {
		_ = connection.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}
	log.Println("Database connected successfully")
	return connection, nil
}

func RunMigrations(ctx context.Context, connection *sql.DB) error {
	if connection == nil {
		return fmt.Errorf("database connection is not configured")
	}
	tx, err := connection.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin database migration: %w", err)
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, "SELECT pg_advisory_xact_lock($1)", migrationLock); err != nil {
		return fmt.Errorf("lock database migration: %w", err)
	}
	log.Println("Checking database baseline and connected workspace migrations...")
	if _, err := tx.ExecContext(ctx, initSQL); err != nil {
		return fmt.Errorf("failed executing migration script: %w", err)
	}
	if err := migrateWorkspace(ctx, tx); err != nil {
		return err
	}
	if err := backfillWorkspace(ctx, tx); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit database migrations: %w", err)
	}
	log.Println("Database schema and initial seeds are up to date")
	return nil
}

func Close(connection *sql.DB) {
	if connection == nil {
		return
	}
	if err := connection.Close(); err != nil {
		log.Printf("Error closing database connection: %v", err)
	} else {
		log.Println("Database connection closed")
	}
}
