package repository_test

import (
	"context"
	"database/sql"
	"errors"
	"reflect"
	"testing"

	"github.com/Bobby-P-dev/go-diagram.git/internal/modules/uidesign"
	"github.com/Bobby-P-dev/go-diagram.git/internal/modules/uidesign/repository"
	"github.com/Bobby-P-dev/go-diagram.git/internal/modules/uidesign/service"
	"github.com/DATA-DOG/go-sqlmock"
)

var (
	_ service.JobBindingRepository = (*repository.PostgresJobBindingRepository)(nil)
)

var bindingColumns = []string{"job_id", "project_id", "user_id", "correlation_id"}

func newDatabase(t *testing.T) (*sql.DB, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Error(err)
		}
		db.Close()
	})
	return db, mock
}

func TestJobBindingsPersistOwnerAndCorrelation(t *testing.T) {
	db, mock := newDatabase(t)
	repo := repository.NewPostgresJobBindingRepository(db)
	want := uidesign.AsyncJobBinding{JobID: "job", ProjectID: "project", UserID: "owner", CorrelationID: "correlation"}
	mock.ExpectExec(`INSERT INTO async_job_bindings`).WithArgs("job", "project", "owner", "correlation").WillReturnResult(sqlmock.NewResult(0, 1))
	if err := repo.Create(context.Background(), want); err != nil {
		t.Fatal(err)
	}
	mock.ExpectQuery(`SELECT .* FROM async_job_bindings WHERE job_id = \$1`).WithArgs("job").WillReturnRows(sqlmock.NewRows(bindingColumns).AddRow("job", "project", "owner", "correlation"))
	got, err := repo.GetByID(context.Background(), "job")
	if err != nil || !reflect.DeepEqual(got, &want) {
		t.Fatalf("binding = %+v, err = %v", got, err)
	}
}

func TestJobBindingRequiresEveryIdentifier(t *testing.T) {
	for _, field := range []string{"job", "project", "owner", "correlation"} {
		t.Run(field, func(t *testing.T) {
			db, _ := newDatabase(t)
			binding := uidesign.AsyncJobBinding{JobID: "job", ProjectID: "project", UserID: "owner", CorrelationID: "correlation"}
			switch field {
			case "job":
				binding.JobID = " "
			case "project":
				binding.ProjectID = " "
			case "owner":
				binding.UserID = " "
			case "correlation":
				binding.CorrelationID = " "
			}
			if err := repository.NewPostgresJobBindingRepository(db).Create(context.Background(), binding); err == nil {
				t.Fatal("expected validation failure without SQL")
			}
		})
	}
}

func TestJobBindingErrors(t *testing.T) {
	db, mock := newDatabase(t)
	repo := repository.NewPostgresJobBindingRepository(db)
	if _, err := repo.GetByID(context.Background(), " "); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("blank job ID: %v", err)
	}
	mock.ExpectQuery(`SELECT .* FROM async_job_bindings`).WithArgs("missing").WillReturnRows(sqlmock.NewRows(bindingColumns))
	if _, err := repo.GetByID(context.Background(), "missing"); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("missing job: %v", err)
	}
	sentinel := errors.New("database failure")
	mock.ExpectExec(`INSERT INTO async_job_bindings`).WithArgs("job", "project", "owner", "correlation").WillReturnError(sentinel)
	if err := repo.Create(context.Background(), uidesign.AsyncJobBinding{JobID: "job", ProjectID: "project", UserID: "owner", CorrelationID: "correlation"}); !errors.Is(err, sentinel) {
		t.Fatalf("create failure: %v", err)
	}
	mock.ExpectQuery(`SELECT .* FROM async_job_bindings`).WithArgs("job").WillReturnError(sentinel)
	if _, err := repo.GetByID(context.Background(), "job"); !errors.Is(err, sentinel) {
		t.Fatalf("lookup failure: %v", err)
	}
	mock.ExpectQuery(`SELECT .* FROM async_job_bindings`).WithArgs("job").WillReturnRows(sqlmock.NewRows([]string{"job_id"}).AddRow("job"))
	if _, err := repo.GetByID(context.Background(), "job"); err == nil {
		t.Fatal("expected scan failure")
	}
}

func TestRepositoriesForwardCanceledContext(t *testing.T) {
	db, _ := newDatabase(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	jobs := repository.NewPostgresJobBindingRepository(db)
	_, jobErr := jobs.GetByID(ctx, "job")
	createErr := jobs.Create(ctx, uidesign.AsyncJobBinding{JobID: "job", ProjectID: "project", UserID: "owner", CorrelationID: "correlation"})
	for _, err := range []error{jobErr, createErr} {
		if !errors.Is(err, context.Canceled) {
			t.Errorf("expected context.Canceled without SQL, got %v", err)
		}
	}
}
