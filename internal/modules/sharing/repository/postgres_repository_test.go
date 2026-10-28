package repository

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/Bobby-P-dev/go-diagram.git/internal/modules/sharing"
	"github.com/DATA-DOG/go-sqlmock"
)

func TestPostgresShareRepository_Create(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	repo := NewPostgresRepository(db)
	now := time.Now()

	mock.ExpectQuery(`INSERT INTO project_shares \(share_token, project_id, title, is_active, view_count, created_at, updated_at\)`).
		WithArgs("token-xyz", "proj-1", "My Flow").
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "share_token", "project_id", "title", "is_active", "view_count", "created_at", "updated_at",
		}).AddRow("share-1", "token-xyz", "proj-1", "My Flow", true, 0, now, now))

	share, err := repo.Create(context.Background(), sharing.NewShare{
		Token:     "token-xyz",
		ProjectID: "proj-1",
		Title:     "My Flow",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if share.ID != "share-1" || share.ShareToken != "token-xyz" || share.Title != "My Flow" {
		t.Fatalf("unexpected share returned: %+v", share)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestPostgresShareRepository_Reactivate(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	repo := NewPostgresRepository(db)
	now := time.Now()

	mock.ExpectQuery(`UPDATE project_shares SET is_active = true, title = \$1, updated_at = NOW\(\) WHERE id = \$2`).
		WithArgs("Updated Title", "share-1").
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "share_token", "project_id", "title", "is_active", "view_count", "created_at", "updated_at",
		}).AddRow("share-1", "token-xyz", "proj-1", "Updated Title", true, 3, now, now))

	share, err := repo.Reactivate(context.Background(), "share-1", "Updated Title")
	if err != nil || share.Title != "Updated Title" {
		t.Fatalf("reactivate failed: %+v, %v", share, err)
	}
}

func TestPostgresShareRepository_FindByProjectID(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	repo := NewPostgresRepository(db)
	now := time.Now()

	mock.ExpectQuery(`SELECT id, share_token, project_id, title, is_active, view_count, created_at, updated_at FROM project_shares WHERE project_id = \$1`).
		WithArgs("proj-1").
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "share_token", "project_id", "title", "is_active", "view_count", "created_at", "updated_at",
		}).AddRow("share-1", "token-xyz", "proj-1", "Title", true, 5, now, now))

	share, err := repo.FindByProjectID(context.Background(), "proj-1")
	if err != nil || share == nil || share.ViewCount != 5 {
		t.Fatalf("unexpected result: share=%+v, err=%v", share, err)
	}

	mock.ExpectQuery(`SELECT id, share_token, project_id, title, is_active, view_count, created_at, updated_at FROM project_shares WHERE project_id = \$1`).
		WithArgs("proj-2").
		WillReturnError(sql.ErrNoRows)

	share, err = repo.FindByProjectID(context.Background(), "proj-2")
	if err != nil || share != nil {
		t.Fatalf("expected nil share without error, got share=%v err=%v", share, err)
	}
}

func TestPostgresShareRepository_FindActiveByToken(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	repo := NewPostgresRepository(db)
	now := time.Now()

	mock.ExpectQuery(`SELECT id, share_token, project_id, title, is_active, view_count, created_at, updated_at FROM project_shares WHERE share_token = \$1 AND is_active = true`).
		WithArgs("token-active").
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "share_token", "project_id", "title", "is_active", "view_count", "created_at", "updated_at",
		}).AddRow("share-1", "token-active", "proj-1", "Title", true, 2, now, now))

	share, err := repo.FindActiveByToken(context.Background(), "token-active")
	if err != nil || share == nil || share.ID != "share-1" {
		t.Fatalf("unexpected share: %+v, err: %v", share, err)
	}
}

func TestPostgresShareRepository_Revoke(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	repo := NewPostgresRepository(db)

	mock.ExpectExec(`UPDATE project_shares SET is_active = false, updated_at = NOW\(\) WHERE project_id = \$1`).
		WithArgs("proj-1").
		WillReturnResult(sqlmock.NewResult(0, 1))

	err = repo.Revoke(context.Background(), "proj-1")
	if err != nil {
		t.Fatalf("expected success, got %v", err)
	}

	mock.ExpectExec(`UPDATE project_shares SET is_active = false, updated_at = NOW\(\) WHERE project_id = \$1`).
		WithArgs("proj-missing").
		WillReturnResult(sqlmock.NewResult(0, 0))

	err = repo.Revoke(context.Background(), "proj-missing")
	if err == nil {
		t.Fatal("expected error when no rows affected")
	}
}

func TestPostgresShareRepository_ContextCancellation(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	repo := NewPostgresRepository(db)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	for _, operation := range []func() error{
		func() error { _, err := repo.FindByProjectID(ctx, "proj-1"); return err },
		func() error { _, err := repo.FindActiveByToken(ctx, "token"); return err },
		func() error { _, err := repo.Create(ctx, sharing.NewShare{}); return err },
		func() error { _, err := repo.Reactivate(ctx, "share-1", "Title"); return err },
		func() error { return repo.Revoke(ctx, "proj-1") },
		func() error { return repo.IncrementViewCount(ctx, "share-1") },
	} {
		if err := operation(); !errors.Is(err, context.Canceled) {
			t.Fatalf("expected context.Canceled error, got %v", err)
		}
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
