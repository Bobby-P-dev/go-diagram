package repository

import (
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestRepositoryDefaults(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	now := time.Now()
	mock.ExpectQuery("INSERT INTO design_comments").WithArgs("project-a", nil, "Designer", "Spacing", 1.0, 2.0, []byte("{}")).WillReturnRows(sqlmock.NewRows([]string{"id", "project_id", "node_id", "author", "content", "status", "position_x", "position_y", "metadata", "created_at", "updated_at"}).AddRow("comment-1", "project-a", nil, "Designer", "Spacing", "open", 1.0, 2.0, []byte("{}"), now, now))
	comment, err := NewCommentRepository(db).Create("project-a", "", "Designer", "Spacing", 1, 2, nil)
	if err != nil || comment.NodeID != nil || string(comment.Metadata) != "{}" {
		t.Fatalf("comment: %+v %v", comment, err)
	}
	mock.ExpectQuery("INSERT INTO design_foundations").WithArgs("Custom", "custom", "", json.RawMessage("{}")).WillReturnRows(sqlmock.NewRows([]string{"id", "name", "category", "description", "tokens", "is_default", "created_at", "updated_at"}).AddRow("foundation-1", "Custom", "custom", "", []byte("{}"), false, now, now))
	foundation, err := NewFoundationRepository(db).Create("Custom", "", "", nil)
	if err != nil || foundation.Category != "custom" || string(foundation.Tokens) != "{}" {
		t.Fatalf("foundation: %+v %v", foundation, err)
	}
	mock.ExpectQuery("FROM workspace_settings").WillReturnError(sql.ErrNoRows)
	settings, err := NewSettingsRepository(db).Get()
	if err != nil || settings.ID != "default" || settings.DefaultMode != "ui_design" || settings.Theme != "dark" || settings.PreferredAIModel != "cx/gpt-6-luna" || string(settings.CanvasPreferences) != `{"snap_to_grid": true, "grid_size": 20}` {
		t.Fatalf("settings: %+v %v", settings, err)
	}
	mock.ExpectQuery("INSERT INTO workspace_settings").WithArgs("diagram", "light", "custom-model", json.RawMessage(`{"snap_to_grid": true, "grid_size": 20}`)).WillReturnRows(sqlmock.NewRows([]string{"id", "default_mode", "theme", "preferred_ai_model", "canvas_preferences", "updated_at"}).AddRow("default", "diagram", "light", "custom-model", []byte(`{"snap_to_grid": true, "grid_size": 20}`), now))
	if _, err := NewSettingsRepository(db).Update("diagram", "light", "custom-model", nil); err != nil {
		t.Fatal(err)
	}
	mock.ExpectQuery("INSERT INTO project_exports").WithArgs("project-a", "html", "index.html", "\u00e9", len("\u00e9"), json.RawMessage("{}")).WillReturnRows(sqlmock.NewRows([]string{"id", "project_id", "export_type", "file_name", "content", "file_size_bytes", "metadata", "created_at"}).AddRow("export-1", "project-a", "html", "index.html", "\u00e9", len("\u00e9"), []byte("{}"), now))
	export, err := NewExportRepository(db).SaveExport("project-a", "html", "index.html", "\u00e9", nil)
	if err != nil || export.FileSizeBytes != 2 || string(export.Metadata) != "{}" {
		t.Fatalf("export: %+v %v", export, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestListRepositoryEmptyAndErrorResults(t *testing.T) {
	for _, table := range []string{"design_comments", "design_foundations", "project_exports"} {
		for _, scenario := range []string{"empty", "query error", "scan error"} {
			t.Run(table+"/"+scenario, func(t *testing.T) {
				db, mock, err := sqlmock.New()
				if err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() { db.Close() })
				query := mock.ExpectQuery("FROM " + table)
				if table != "design_foundations" {
					query.WithArgs("project-a")
				}
				if scenario == "query error" {
					query.WillReturnError(errors.New("storage down"))
				} else {
					rows := sqlmock.NewRows([]string{"id"})
					if scenario == "scan error" {
						rows.AddRow("invalid-row")
					}
					query.WillReturnRows(rows)
				}
				var count int
				var nonNil bool
				switch table {
				case "design_comments":
					result, resultErr := NewCommentRepository(db).GetByProjectID("project-a")
					count, nonNil, err = len(result), result != nil, resultErr
				case "design_foundations":
					result, resultErr := NewFoundationRepository(db).GetAll()
					count, nonNil, err = len(result), result != nil, resultErr
				case "project_exports":
					result, resultErr := NewExportRepository(db).GetByProjectID("project-a")
					count, nonNil, err = len(result), result != nil, resultErr
				}
				if scenario == "empty" && (err != nil || count != 0 || !nonNil) {
					t.Fatalf("empty list changed: count=%d nonNil=%v err=%v", count, nonNil, err)
				}
				if scenario != "empty" && err == nil {
					t.Fatal("expected persistence error")
				}
				if err := mock.ExpectationsWereMet(); err != nil {
					t.Fatal(err)
				}
			})
		}
	}
}

func TestCommentMutationsReportMissingRowsAndStorageFailures(t *testing.T) {
	for _, operation := range []string{"update", "delete"} {
		for _, failure := range []bool{false, true} {
			t.Run(operation+"/"+map[bool]string{false: "missing", true: "storage"}[failure], func(t *testing.T) {
				db, mock, err := sqlmock.New()
				if err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() { db.Close() })
				pattern := "DELETE FROM design_comments"
				arguments := []driver.Value{"comment-1"}
				if operation == "update" {
					pattern = "UPDATE design_comments"
					arguments = []driver.Value{"resolved", "comment-1"}
				}
				expected := mock.ExpectExec(pattern).WithArgs(arguments...)
				if failure {
					expected.WillReturnError(errors.New("storage down"))
				} else {
					expected.WillReturnResult(sqlmock.NewResult(0, 0))
				}
				repo := NewCommentRepository(db)
				if operation == "update" {
					err = repo.UpdateStatus("comment-1", "resolved")
				} else {
					err = repo.Delete("comment-1")
				}
				if err == nil {
					t.Fatal("expected error")
				}
				if err := mock.ExpectationsWereMet(); err != nil {
					t.Fatal(err)
				}
			})
		}
	}
}
