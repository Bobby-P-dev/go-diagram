package repository

import (
	"context"
	"errors"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestFindByKeyHashMapsCredentialWithoutDisplayMasking(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	now := time.Now()
	expiresAt := now.Add(time.Hour)
	query := regexp.QuoteMeta(`
		SELECT id, credential_key_prefix, name, role, is_active, can_generate_diagram, can_generate_ui, expires_at, last_login_at, created_at, updated_at
		FROM access_credentials
		WHERE credential_key = $1 AND credential_key_prefix <> ''
	`)
	mock.ExpectQuery(query).
		WithArgs("hashed-key").
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "credential_key_prefix", "name", "role", "is_active",
			"can_generate_diagram", "can_generate_ui", "expires_at", "last_login_at", "created_at", "updated_at",
		}).AddRow("user-1", "RL-ABCD-", "Client", "user", true, true, false, expiresAt, nil, now, now))

	credential, err := NewPostgresRepository(db).FindByKeyHash(context.Background(), "hashed-key")
	if err != nil {
		t.Fatalf("unexpected lookup error: %v", err)
	}
	if credential.KeyPrefix != "RL-ABCD-" || credential.Name != "Client" {
		t.Fatalf("unexpected credential mapping: %+v", credential)
	}
	if credential.ExpiresAt == nil || !credential.ExpiresAt.Equal(expiresAt) {
		t.Fatalf("expiry was not mapped: %+v", credential.ExpiresAt)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestCreatePersistsPreparedHashAndPrefix(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	now := time.Now()
	mock.ExpectQuery(`INSERT INTO access_credentials \(credential_key, credential_key_prefix`).
		WithArgs("hashed-key", "RL-ABCD-", "Client", "admin", nil, true, false).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "credential_key_prefix", "name", "role", "is_active",
			"can_generate_diagram", "can_generate_ui", "expires_at", "last_login_at", "created_at", "updated_at",
		}).AddRow("user-1", "RL-ABCD-", "Client", "admin", true, true, false, nil, nil, now, now))

	credential, err := NewPostgresRepository(db).Create(context.Background(), NewCredential{
		KeyHash:            "hashed-key",
		KeyPrefix:          "RL-ABCD-",
		Name:               "Client",
		Role:               "admin",
		CanGenerateDiagram: true,
	})
	if err != nil {
		t.Fatalf("unexpected create error: %v", err)
	}
	if credential.ID != "user-1" || credential.Role != "admin" {
		t.Fatalf("unexpected created credential: %+v", credential)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestEnsureMasterAdminDoesNotValidateKeyWhenAccountExists(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT EXISTS`).
		WithArgs("a0000000-0000-0000-0000-000000000001").
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))
	mock.ExpectExec(`UPDATE projects SET user_id = \$1 WHERE user_id IS NULL`).
		WithArgs("a0000000-0000-0000-0000-000000000001").
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectCommit()

	if err := NewPostgresRepository(db).EnsureMasterAdmin(context.Background(), ""); err != nil {
		t.Fatalf("existing admin must not require a bootstrap key: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestDeleteMissingCredentialReturnsNotFound(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	mock.ExpectExec(regexp.QuoteMeta(`DELETE FROM access_credentials WHERE id = $1`)).
		WithArgs("missing").
		WillReturnResult(sqlmock.NewResult(0, 0))

	err = NewPostgresRepository(db).Delete(context.Background(), "missing")
	if err == nil {
		t.Fatal("expected missing credential error")
	}
	if err.Error() != "kredensial tidak ditemukan: missing" {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestRepositoryHonorsCanceledContext(t *testing.T) {
	operations := map[string]func(Repository, context.Context) error{
		"migrate": func(repo Repository, ctx context.Context) error {
			return repo.MigrateCredentialKeys(ctx)
		},
		"bootstrap": func(repo Repository, ctx context.Context) error {
			return repo.EnsureMasterAdmin(ctx, "")
		},
		"find by hash": func(repo Repository, ctx context.Context) error {
			_, err := repo.FindByKeyHash(ctx, "hash")
			return err
		},
		"find by ID": func(repo Repository, ctx context.Context) error {
			_, err := repo.FindByID(ctx, "user-1")
			return err
		},
		"create": func(repo Repository, ctx context.Context) error {
			_, err := repo.Create(ctx, NewCredential{})
			return err
		},
		"update": func(repo Repository, ctx context.Context) error {
			_, err := repo.Update(ctx, UpdateCredential{})
			return err
		},
		"delete": func(repo Repository, ctx context.Context) error {
			return repo.Delete(ctx, "user-1")
		},
		"last login": func(repo Repository, ctx context.Context) error {
			return repo.UpdateLastLogin(ctx, "user-1")
		},
		"list": func(repo Repository, ctx context.Context) error {
			_, err := repo.GetAllWithStats(ctx)
			return err
		},
	}
	for name, operation := range operations {
		t.Run(name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			if err := operation(NewPostgresRepository(db), ctx); !errors.Is(err, context.Canceled) {
				t.Fatalf("expected context.Canceled without executing SQL, got %v", err)
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestGetAllWithStatsRejectsPartialResults(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	now := time.Now()
	failure := errors.New("failed reading credential rows")
	rows := sqlmock.NewRows([]string{
		"id", "credential_key_prefix", "name", "role", "is_active",
		"can_generate_diagram", "can_generate_ui", "expires_at", "last_login_at",
		"created_at", "updated_at", "project_count",
	}).AddRow("user-1", "hidden", "Client", "user", true, true, false, nil, nil, now, now, 3).
		AddRow("user-2", "hidden", "Other", "user", true, true, false, nil, nil, now, now, 0).
		RowError(1, failure)
	mock.ExpectQuery(`SELECT`).WillReturnRows(rows).RowsWillBeClosed()
	result, err := NewPostgresRepository(db).GetAllWithStats(context.Background())
	if !errors.Is(err, failure) || result != nil {
		t.Fatalf("expected wrapped failure and no partial results, got %+v, %v", result, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
