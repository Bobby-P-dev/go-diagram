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
	log.Println("Checking and running database migrations (init.sql)...")
	if _, err := connection.ExecContext(ctx, initSQL); err != nil {
		return fmt.Errorf("failed executing migration script: %w", err)
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
