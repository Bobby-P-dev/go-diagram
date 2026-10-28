package repository

import (
	"database/sql"
	"encoding/json"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestCommentRepository_CRUD(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()

	repo := NewCommentRepository(db)
	now := time.Now()

	mock.ExpectQuery(regexp.QuoteMeta(`INSERT INTO design_comments (project_id, node_id, author, content, status, position_x, position_y, metadata, created_at, updated_at)`)).
		WithArgs("proj-1", "node-1", "Designer", "Fix spacing", 10.0, 20.0, []byte("{}")).
		WillReturnRows(sqlmock.NewRows([]string{"id", "project_id", "node_id", "author", "content", "status", "position_x", "position_y", "metadata", "created_at", "updated_at"}).
			AddRow("comm-1", "proj-1", "node-1", "Designer", "Fix spacing", "open", 10.0, 20.0, []byte("{}"), now, now))

	comment, err := repo.Create("proj-1", "node-1", "Designer", "Fix spacing", 10.0, 20.0, []byte("{}"))
	if err != nil {
		t.Fatalf("failed to create comment: %v", err)
	}
	if comment.ID != "comm-1" || comment.Author != "Designer" {
		t.Fatalf("unexpected comment: %+v", comment)
	}

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT id, project_id, node_id, author, content, status, position_x, position_y, COALESCE(metadata, '{}'::jsonb), created_at, updated_at FROM design_comments WHERE id = $1`)).
		WithArgs("comm-1").
		WillReturnRows(sqlmock.NewRows([]string{"id", "project_id", "node_id", "author", "content", "status", "position_x", "position_y", "metadata", "created_at", "updated_at"}).
			AddRow("comm-1", "proj-1", "node-1", "Designer", "Fix spacing", "open", 10.0, 20.0, []byte("{}"), now, now))

	comment, err = repo.GetByID("comm-1")
	if err != nil {
		t.Fatalf("failed to get comment: %v", err)
	}
	if comment.ID != "comm-1" {
		t.Fatalf("unexpected comment: %+v", comment)
	}

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT id, project_id, node_id, author, content, status, position_x, position_y, COALESCE(metadata, '{}'::jsonb), created_at, updated_at FROM design_comments WHERE id = $1`)).
		WithArgs("missing").
		WillReturnError(sql.ErrNoRows)

	_, err = repo.GetByID("missing")
	if err == nil {
		t.Fatal("expected error on missing comment")
	}

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT id, project_id, node_id, author, content, status, position_x, position_y, COALESCE(metadata, '{}'::jsonb), created_at, updated_at FROM design_comments WHERE project_id = $1 ORDER BY created_at ASC`)).
		WithArgs("proj-1").
		WillReturnRows(sqlmock.NewRows([]string{"id", "project_id", "node_id", "author", "content", "status", "position_x", "position_y", "metadata", "created_at", "updated_at"}).
			AddRow("comm-1", "proj-1", "node-1", "Designer", "Fix spacing", "open", 10.0, 20.0, []byte("{}"), now, now))

	comments, err := repo.GetByProjectID("proj-1")
	if err != nil {
		t.Fatalf("failed to get comments: %v", err)
	}
	if len(comments) != 1 || comments[0].ID != "comm-1" {
		t.Fatalf("unexpected comments count: %d", len(comments))
	}

	mock.ExpectExec(regexp.QuoteMeta(`UPDATE design_comments SET status = $1, updated_at = NOW() WHERE id = $2`)).
		WithArgs("resolved", "comm-1").
		WillReturnResult(sqlmock.NewResult(1, 1))

	if err := repo.UpdateStatus("comm-1", "resolved"); err != nil {
		t.Fatalf("failed to update status: %v", err)
	}

	mock.ExpectExec(regexp.QuoteMeta(`DELETE FROM design_comments WHERE id = $1`)).
		WithArgs("comm-1").
		WillReturnResult(sqlmock.NewResult(1, 1))

	if err := repo.Delete("comm-1"); err != nil {
		t.Fatalf("failed to delete comment: %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestFoundationRepository_GetAllAndCreate(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()

	repo := NewFoundationRepository(db)
	now := time.Now()

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT id, name, category, COALESCE(description, ''), tokens, is_default, created_at, updated_at FROM design_foundations ORDER BY is_default DESC, name ASC`)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "category", "description", "tokens", "is_default", "created_at", "updated_at"}).
			AddRow("f-1", "Minimal", "design", "Minimal theme", []byte("{}"), true, now, now))

	foundations, err := repo.GetAll()
	if err != nil {
		t.Fatalf("failed to get foundations: %v", err)
	}
	if len(foundations) != 1 || foundations[0].Name != "Minimal" {
		t.Fatalf("unexpected foundations: %+v", foundations)
	}

	mock.ExpectQuery(regexp.QuoteMeta(`INSERT INTO design_foundations (name, category, description, tokens, is_default, created_at, updated_at) VALUES ($1, $2, $3, $4, false, NOW(), NOW()) RETURNING id, name, category, COALESCE(description, ''), tokens, is_default, created_at, updated_at`)).
		WithArgs("CustomTheme", "custom", "Custom desc", json.RawMessage("{}")).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "category", "description", "tokens", "is_default", "created_at", "updated_at"}).
			AddRow("f-2", "CustomTheme", "custom", "Custom desc", []byte("{}"), false, now, now))

	created, err := repo.Create("CustomTheme", "custom", "Custom desc", json.RawMessage("{}"))
	if err != nil {
		t.Fatalf("failed to create foundation: %v", err)
	}
	if created.Name != "CustomTheme" {
		t.Fatalf("unexpected foundation: %+v", created)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestSettingsRepository_GetAndUpdate(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()

	repo := NewSettingsRepository(db)
	now := time.Now()

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT id, default_mode, theme, preferred_ai_model, canvas_preferences, updated_at FROM workspace_settings WHERE id = 'default' LIMIT 1`)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "default_mode", "theme", "preferred_ai_model", "canvas_preferences", "updated_at"}).
			AddRow("default", "ui_design", "dark", "gpt-4", []byte(`{"grid":true}`), now))

	settings, err := repo.Get()
	if err != nil {
		t.Fatalf("failed to get settings: %v", err)
	}
	if settings.Theme != "dark" {
		t.Fatalf("unexpected settings: %+v", settings)
	}

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT id, default_mode, theme, preferred_ai_model, canvas_preferences, updated_at FROM workspace_settings WHERE id = 'default' LIMIT 1`)).
		WillReturnError(sql.ErrNoRows)

	fallback, err := repo.Get()
	if err != nil {
		t.Fatalf("failed to get default fallback settings: %v", err)
	}
	if fallback.ID != "default" || fallback.DefaultMode != "ui_design" {
		t.Fatalf("unexpected fallback: %+v", fallback)
	}

	mock.ExpectQuery(regexp.QuoteMeta(`INSERT INTO workspace_settings (id, default_mode, theme, preferred_ai_model, canvas_preferences, updated_at) VALUES ('default', $1, $2, $3, $4, NOW()) ON CONFLICT (id) DO UPDATE SET default_mode = EXCLUDED.default_mode, theme = EXCLUDED.theme, preferred_ai_model = EXCLUDED.preferred_ai_model, canvas_preferences = EXCLUDED.canvas_preferences, updated_at = NOW() RETURNING id, default_mode, theme, preferred_ai_model, canvas_preferences, updated_at`)).
		WithArgs("diagram", "light", "gpt-4", json.RawMessage(`{"grid":false}`)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "default_mode", "theme", "preferred_ai_model", "canvas_preferences", "updated_at"}).
			AddRow("default", "diagram", "light", "gpt-4", []byte(`{"grid":false}`), now))

	updated, err := repo.Update("diagram", "light", "gpt-4", json.RawMessage(`{"grid":false}`))
	if err != nil {
		t.Fatalf("failed to update settings: %v", err)
	}
	if updated.Theme != "light" || updated.DefaultMode != "diagram" {
		t.Fatalf("unexpected settings: %+v", updated)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestExportRepository_SaveAndGet(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()

	repo := NewExportRepository(db)
	now := time.Now()

	mock.ExpectQuery(regexp.QuoteMeta(`INSERT INTO project_exports (project_id, export_type, file_name, content, file_size_bytes, metadata, created_at) VALUES ($1, $2, $3, $4, $5, $6, NOW()) RETURNING id, project_id, export_type, file_name, content, file_size_bytes, metadata, created_at`)).
		WithArgs("proj-1", "png", "diagram.png", "binarycontent", 13, json.RawMessage("{}")).
		WillReturnRows(sqlmock.NewRows([]string{"id", "project_id", "export_type", "file_name", "content", "file_size_bytes", "metadata", "created_at"}).
			AddRow("exp-1", "proj-1", "png", "diagram.png", "binarycontent", 13, []byte("{}"), now))

	exp, err := repo.SaveExport("proj-1", "png", "diagram.png", "binarycontent", json.RawMessage("{}"))
	if err != nil {
		t.Fatalf("failed to save export: %v", err)
	}
	if exp.ID != "exp-1" || exp.FileName != "diagram.png" {
		t.Fatalf("unexpected export: %+v", exp)
	}

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT id, project_id, export_type, file_name, content, file_size_bytes, metadata, created_at FROM project_exports WHERE project_id = $1 ORDER BY created_at DESC`)).
		WithArgs("proj-1").
		WillReturnRows(sqlmock.NewRows([]string{"id", "project_id", "export_type", "file_name", "content", "file_size_bytes", "metadata", "created_at"}).
			AddRow("exp-1", "proj-1", "png", "diagram.png", "binarycontent", 13, []byte("{}"), now))

	exports, err := repo.GetByProjectID("proj-1")
	if err != nil {
		t.Fatalf("failed to get exports: %v", err)
	}
	if len(exports) != 1 || exports[0].ID != "exp-1" {
		t.Fatalf("unexpected exports: %+v", exports)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}
