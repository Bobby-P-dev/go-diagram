package repository_test

import (
	"context"
	"database/sql"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/Bobby-P-dev/go-diagram.git/internal/modules/uidesign"
	"github.com/Bobby-P-dev/go-diagram.git/internal/modules/uidesign/repository"
	"github.com/Bobby-P-dev/go-diagram.git/internal/modules/uidesign/service"
	"github.com/DATA-DOG/go-sqlmock"
)

var (
	_ service.TemplateRepository   = (*repository.PostgresTemplateRepository)(nil)
	_ service.JobBindingRepository = (*repository.PostgresJobBindingRepository)(nil)
)

var templateColumns = []string{"id", "name", "category", "device", "description", "theme", "sections", "code_export", "is_featured", "created_at"}
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

func templateRows() *sqlmock.Rows {
	return sqlmock.NewRows(templateColumns).AddRow("template-1", "Dashboard", "business", "web", "description", []byte(`{"accent":"blue"}`), []byte(`[]`), []byte(`{}`), true, time.Unix(1234, 0))
}

func TestTemplatesReadAndMapFields(t *testing.T) {
	db, mock := newDatabase(t)
	repo := repository.NewPostgresTemplateRepository(db)
	mock.ExpectQuery(`SELECT .* FROM ui_templates ORDER BY is_featured DESC, name ASC`).WillReturnRows(templateRows()).RowsWillBeClosed()
	got, err := repo.GetAllUITemplates(context.Background())
	if err != nil || len(got) != 1 {
		t.Fatalf("templates = %+v, err = %v", got, err)
	}
	want := uidesign.UITemplate{ID: "template-1", Name: "Dashboard", Category: "business", Device: "web", Description: "description", Theme: []byte(`{"accent":"blue"}`), Sections: []byte(`[]`), CodeExport: []byte(`{}`), IsFeatured: true, CreatedAt: time.Unix(1234, 0)}
	if !reflect.DeepEqual(got[0], want) {
		t.Fatalf("template = %+v, want %+v", got[0], want)
	}
	mock.ExpectQuery(`SELECT .* FROM ui_templates WHERE id = \$1`).WithArgs("template-1").WillReturnRows(templateRows())
	item, err := repo.GetUITemplateByID(context.Background(), "template-1")
	if err != nil || !reflect.DeepEqual(item, &want) {
		t.Fatalf("template by ID = %+v, err = %v", item, err)
	}
}

func TestTemplatesEmptyListRemainsNil(t *testing.T) {
	db, mock := newDatabase(t)
	mock.ExpectQuery(`SELECT .* FROM ui_templates`).WillReturnRows(sqlmock.NewRows(templateColumns)).RowsWillBeClosed()
	got, err := repository.NewPostgresTemplateRepository(db).GetAllUITemplates(context.Background())
	if err != nil || got != nil {
		t.Fatalf("empty templates = %#v, err = %v", got, err)
	}
}

func TestTemplateReadFailures(t *testing.T) {
	sentinel := errors.New("database unavailable")
	tests := []struct {
		name       string
		list       bool
		rows       *sqlmock.Rows
		queryError error
		wantError  error
		message    string
	}{
		{name: "list query", list: true, queryError: sentinel, wantError: sentinel},
		{name: "list scan", list: true, rows: sqlmock.NewRows(templateColumns).AddRow("id", "name", "category", "web", "description", "{}", "[]", "{}", "invalid-bool", time.Now()), message: "failed to scan ui template"},
		{name: "iteration", list: true, rows: templateRows().RowError(0, sentinel), wantError: sentinel},
		{name: "missing", rows: sqlmock.NewRows(templateColumns), message: "ui template not found"},
		{name: "lookup query", queryError: sentinel, wantError: sentinel},
		{name: "lookup scan", rows: sqlmock.NewRows([]string{"id"}).AddRow("id"), message: "failed to get ui template"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			db, mock := newDatabase(t)
			expect := mock.ExpectQuery(`SELECT .* FROM ui_templates`)
			if !test.list {
				expect.WithArgs("id")
			}
			if test.queryError != nil {
				expect.WillReturnError(test.queryError)
			} else {
				expect.WillReturnRows(test.rows).RowsWillBeClosed()
			}
			repo := repository.NewPostgresTemplateRepository(db)
			var err error
			if test.list {
				var got []uidesign.UITemplate
				got, err = repo.GetAllUITemplates(context.Background())
				if got != nil {
					t.Fatalf("must not return partial templates: %+v", got)
				}
			} else {
				_, err = repo.GetUITemplateByID(context.Background(), "id")
			}
			if err == nil || (test.wantError != nil && !errors.Is(err, test.wantError)) || !strings.Contains(err.Error(), test.message) {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
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
	templates := repository.NewPostgresTemplateRepository(db)
	jobs := repository.NewPostgresJobBindingRepository(db)
	_, listErr := templates.GetAllUITemplates(ctx)
	_, getErr := templates.GetUITemplateByID(ctx, "id")
	_, jobErr := jobs.GetByID(ctx, "job")
	createErr := jobs.Create(ctx, uidesign.AsyncJobBinding{JobID: "job", ProjectID: "project", UserID: "owner", CorrelationID: "correlation"})
	for _, err := range []error{listErr, getErr, jobErr, createErr} {
		if !errors.Is(err, context.Canceled) {
			t.Errorf("expected context.Canceled without SQL, got %v", err)
		}
	}
}

func TestTemplateQueryCanBeCanceledInFlight(t *testing.T) {
	db, mock := newDatabase(t)
	mock.ExpectQuery(`SELECT .* FROM ui_templates`).WillDelayFor(time.Second).WillReturnRows(templateRows())
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	got, err := repository.NewPostgresTemplateRepository(db).GetAllUITemplates(ctx)
	if err == nil || got != nil || ctx.Err() != context.DeadlineExceeded {
		t.Fatalf("canceled query = %+v, err = %v, ctx = %v", got, err, ctx.Err())
	}
}
