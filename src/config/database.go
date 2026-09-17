package config

import (
	"database/sql"
	_ "embed"
	"fmt"
	"log"
	"time"

	_ "github.com/lib/pq"
)

//go:embed init.sql
var initSQL string

var DB *sql.DB

func ConnectDatabase() {
	dsn := fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		Env.DBHost,
		Env.DBPort,
		Env.DBUser,
		Env.DBPassword,
		Env.DBName,
		Env.DBSSLMode,
	)

	var err error
	DB, err = sql.Open("postgres", dsn)
	if err != nil {
		log.Fatalf("Failed to open database connection: %v", err)
	}

	DB.SetMaxOpenConns(25)
	DB.SetMaxIdleConns(10)
	DB.SetConnMaxLifetime(5 * time.Minute)

	err = DB.Ping()
	if err != nil {
		log.Fatalf("Failed to ping database: %v", err)
	}

	log.Println("Database connected successfully")

	// Ensure all tables, indexes, and initial seeds exist automatically
	if err := RunMigrations(); err != nil {
		log.Printf("Warning during automatic database migration: %v", err)
	}
}

// RunMigrations applies init.sql idempotently (IF NOT EXISTS & ON CONFLICT DO NOTHING)
func RunMigrations() error {
	if initSQL == "" {
		return nil
	}
	log.Println("Checking and running database migrations (init.sql)...")
	_, err := DB.Exec(initSQL)
	if err != nil {
		return fmt.Errorf("failed executing migration script: %w", err)
	}
	log.Println("Database schema and initial seeds are up to date")
	return nil
}

func CloseDatabase() {
	if DB != nil {
		err := DB.Close()
		if err != nil {
			log.Printf("Error closing database connection: %v", err)
		} else {
			log.Println("Database connection closed")
		}
	}
}
