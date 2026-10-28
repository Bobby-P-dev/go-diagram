package database

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestConnectContextAndPingFailure(test *testing.T) {
	mockDB, mock, err := sqlmock.New(sqlmock.MonitorPingsOption(true))
	if err != nil {
		test.Fatal(err)
	}
	defer mockDB.Close()

	test.Run("success", func(test *testing.T) {
		mock.ExpectPing()
		db, err := connect(context.Background(), Config{Host: "localhost"}, func(driverName, dataSourceName string) (*sql.DB, error) {
			return mockDB, nil
		})
		if err != nil || db == nil {
			test.Fatalf("expected success, got %v", err)
		}
	})

	test.Run("ping failure closes db", func(test *testing.T) {
		pingErr := errors.New("cannot ping database")
		mock.ExpectPing().WillReturnError(pingErr)
		mock.ExpectClose()
		_, err := connect(context.Background(), Config{Host: "localhost"}, func(driverName, dataSourceName string) (*sql.DB, error) {
			return mockDB, nil
		})
		if !errors.Is(err, pingErr) {
			test.Fatalf("expected ping error, got %v", err)
		}
	})
}

func TestRunMigrations(test *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		test.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()

	test.Run("nil connection returns error", func(test *testing.T) {
		if err := RunMigrations(context.Background(), nil); err == nil {
			test.Fatal("expected error for nil connection")
		}
	})

	test.Run("migration execution success", func(test *testing.T) {
		mock.ExpectExec(".*").WillReturnResult(sqlmock.NewResult(0, 0))

		if err := RunMigrations(context.Background(), db); err != nil {
			test.Errorf("expected no error, got %v", err)
		}
	})

	test.Run("migration execution error", func(test *testing.T) {
		mock.ExpectExec(".*").WillReturnError(errors.New("syntax error in SQL"))

		if err := RunMigrations(context.Background(), db); err == nil {
			test.Errorf("expected error, got nil")
		}
	})
}

func TestClose(test *testing.T) {
	db, _, err := sqlmock.New()
	if err != nil {
		test.Fatalf("failed to create sqlmock: %v", err)
	}

	Close(db)
	Close(nil) // Should not panic
}
