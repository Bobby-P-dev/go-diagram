package database

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"errors"
	"fmt"
	"regexp"
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
	test.Run("nil connection returns error", func(test *testing.T) {
		if err := RunMigrations(context.Background(), nil); err == nil {
			test.Fatal("expected error for nil connection")
		}
	})

	for _, scenario := range []string{"new migration", "already applied", "checksum mismatch", "baseline failure", "backfill failure", "commit failure"} {
		test.Run(scenario, func(test *testing.T) {
			db, mock, err := sqlmock.New()
			if err != nil {
				test.Fatal(err)
			}
			defer db.Close()
			mock.ExpectBegin()
			mock.ExpectExec(regexp.QuoteMeta("SELECT pg_advisory_xact_lock($1)")).WithArgs(migrationLock).WillReturnResult(sqlmock.NewResult(0, 0))
			baseline := mock.ExpectExec(regexp.QuoteMeta(initSQL))
			if scenario == "baseline failure" {
				baseline.WillReturnError(errors.New("baseline failed"))
			} else {
				baseline.WillReturnResult(sqlmock.NewResult(0, 0))
				mock.ExpectExec("CREATE TABLE IF NOT EXISTS schema_migrations").WillReturnResult(sqlmock.NewResult(0, 0))
				source, readErr := workspaceMigrations.ReadFile("migrations/04_connected_workspace.sql")
				if readErr != nil {
					test.Fatal(readErr)
				}
				checksum := fmt.Sprintf("%x", sha256.Sum256(source))
				ledger := mock.ExpectQuery(regexp.QuoteMeta("SELECT checksum FROM schema_migrations WHERE version = $1")).WithArgs("04_connected_workspace")
				switch scenario {
				case "new migration":
					ledger.WillReturnError(sql.ErrNoRows)
					mock.ExpectExec(regexp.QuoteMeta(string(source))).WillReturnResult(sqlmock.NewResult(0, 0))
					mock.ExpectExec("INSERT INTO schema_migrations").WithArgs("04_connected_workspace", checksum).WillReturnResult(sqlmock.NewResult(0, 1))
				case "checksum mismatch":
					ledger.WillReturnRows(sqlmock.NewRows([]string{"checksum"}).AddRow("changed"))
				default:
					ledger.WillReturnRows(sqlmock.NewRows([]string{"checksum"}).AddRow(checksum))
				}
				if scenario != "checksum mismatch" {
					nextSource, readErr := workspaceMigrations.ReadFile("migrations/05_generation_runs.sql")
					if readErr != nil {
						test.Fatal(readErr)
					}
					nextChecksum := fmt.Sprintf("%x", sha256.Sum256(nextSource))
					nextLedger := mock.ExpectQuery(regexp.QuoteMeta("SELECT checksum FROM schema_migrations WHERE version = $1")).WithArgs("05_generation_runs")
					if scenario == "new migration" {
						nextLedger.WillReturnError(sql.ErrNoRows)
						mock.ExpectExec(regexp.QuoteMeta(string(nextSource))).WillReturnResult(sqlmock.NewResult(0, 0))
						mock.ExpectExec("INSERT INTO schema_migrations").WithArgs("05_generation_runs", nextChecksum).WillReturnResult(sqlmock.NewResult(0, 1))
					} else {
						nextLedger.WillReturnRows(sqlmock.NewRows([]string{"checksum"}).AddRow(nextChecksum))
					}
					diagramSource, readErr := workspaceMigrations.ReadFile("migrations/06_diagram_generation.sql")
					if readErr != nil {
						test.Fatal(readErr)
					}
					diagramChecksum := fmt.Sprintf("%x", sha256.Sum256(diagramSource))
					diagramLedger := mock.ExpectQuery(regexp.QuoteMeta("SELECT checksum FROM schema_migrations WHERE version = $1")).WithArgs("06_diagram_generation")
					if scenario == "new migration" {
						diagramLedger.WillReturnError(sql.ErrNoRows)
						mock.ExpectExec(regexp.QuoteMeta(string(diagramSource))).WillReturnResult(sqlmock.NewResult(0, 0))
						mock.ExpectExec("INSERT INTO schema_migrations").WithArgs("06_diagram_generation", diagramChecksum).WillReturnResult(sqlmock.NewResult(0, 1))
					} else {
						diagramLedger.WillReturnRows(sqlmock.NewRows([]string{"checksum"}).AddRow(diagramChecksum))
					}
					mutationSource, readErr := workspaceMigrations.ReadFile("migrations/07_diagram_mutations.sql")
					if readErr != nil {
						test.Fatal(readErr)
					}
					mutationChecksum := fmt.Sprintf("%x", sha256.Sum256(mutationSource))
					mutationLedger := mock.ExpectQuery(regexp.QuoteMeta("SELECT checksum FROM schema_migrations WHERE version = $1")).WithArgs("07_diagram_mutations")
					if scenario == "new migration" {
						mutationLedger.WillReturnError(sql.ErrNoRows)
						mock.ExpectExec(regexp.QuoteMeta(string(mutationSource))).WillReturnResult(sqlmock.NewResult(0, 0))
						mock.ExpectExec("INSERT INTO schema_migrations").WithArgs("07_diagram_mutations", mutationChecksum).WillReturnResult(sqlmock.NewResult(0, 1))
					} else {
						mutationLedger.WillReturnRows(sqlmock.NewRows([]string{"checksum"}).AddRow(mutationChecksum))
					}
					authSource, readErr := workspaceMigrations.ReadFile("migrations/08_password_sessions.sql")
					if readErr != nil {
						test.Fatal(readErr)
					}
					authChecksum := fmt.Sprintf("%x", sha256.Sum256(authSource))
					authLedger := mock.ExpectQuery(regexp.QuoteMeta("SELECT checksum FROM schema_migrations WHERE version = $1")).WithArgs("08_password_sessions")
					if scenario == "new migration" {
						authLedger.WillReturnError(sql.ErrNoRows)
						mock.ExpectExec(regexp.QuoteMeta(string(authSource))).WillReturnResult(sqlmock.NewResult(0, 0))
						mock.ExpectExec("INSERT INTO schema_migrations").WithArgs("08_password_sessions", authChecksum).WillReturnResult(sqlmock.NewResult(0, 1))
					} else {
						authLedger.WillReturnRows(sqlmock.NewRows([]string{"checksum"}).AddRow(authChecksum))
					}
					mock.ExpectExec("LOCK TABLE projects, chat_messages, diagram_versions").WillReturnResult(sqlmock.NewResult(0, 0))
					backfill, readErr := workspaceMigrations.ReadFile("migrations/04_connected_workspace_backfill.sql")
					if readErr != nil {
						test.Fatal(readErr)
					}
					importQuery := mock.ExpectExec(regexp.QuoteMeta(string(backfill)))
					if scenario == "backfill failure" {
						importQuery.WillReturnError(errors.New("backfill failed"))
					} else {
						importQuery.WillReturnResult(sqlmock.NewResult(0, 0))
					}
				}
			}
			wantError := scenario != "new migration" && scenario != "already applied"
			if scenario == "commit failure" {
				mock.ExpectCommit().WillReturnError(errors.New("commit failed"))
			} else if wantError {
				mock.ExpectRollback()
			} else {
				mock.ExpectCommit()
			}
			if err := RunMigrations(context.Background(), db); (err != nil) != wantError {
				test.Fatalf("unexpected migration result: %v", err)
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				test.Fatal(err)
			}
		})
	}
}

func TestClose(test *testing.T) {
	db, _, err := sqlmock.New()
	if err != nil {
		test.Fatalf("failed to create sqlmock: %v", err)
	}

	Close(db)
	Close(nil) // Should not panic
}
