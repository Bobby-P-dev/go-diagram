package repository

import (
	"context"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestContextOperationsCancelWithoutDatabaseCalls(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil { t.Fatal(err) }
	t.Cleanup(func() { db.Close() })
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	projects := NewProjectRepository(db, "fallback-owner")
	if _, err := projects.CreateWithUserContext(ctx, "owner", "Title", "flowchart", "diagram", nil, nil, nil); !errors.Is(err, context.Canceled) { t.Fatalf("create: %v", err) }
	if _, _, err := projects.GetByIDScopedContext(ctx, "project", "owner", "member"); !errors.Is(err, context.Canceled) { t.Fatalf("get: %v", err) }
	if err := projects.UpdateGraphContext(ctx, "project", nil, nil); !errors.Is(err, context.Canceled) { t.Fatalf("update: %v", err) }
	if _, err := NewMessageRepository(db).AppendMessageContext(ctx, "project", "user", "Prompt", nil); !errors.Is(err, context.Canceled) { t.Fatalf("message: %v", err) }
	versions := NewVersionRepository(db)
	if _, err := versions.CreateVersionContext(ctx, "project", 1, "Snapshot", nil, nil, nil); !errors.Is(err, context.Canceled) { t.Fatalf("snapshot: %v", err) }
	if _, err := versions.GetLatestVersionNumberContext(ctx, "project"); !errors.Is(err, context.Canceled) { t.Fatalf("latest: %v", err) }
	if err := mock.ExpectationsWereMet(); err != nil { t.Fatal(err) }
}

func TestForeignAdminCannotReadProjectMessages(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil { t.Fatal(err) }
	t.Cleanup(func() { db.Close() })
	now := time.Now()
	mock.ExpectQuery("FROM projects").WithArgs("project").WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "title", "diagram_type", "project_mode", "metadata", "is_pinned", "current_nodes", "current_edges", "created_at", "updated_at"}).AddRow("project", "owner", "Title", "flowchart", "diagram", []byte(`{}`), false, []byte(`[]`), []byte(`[]`), now, now))
	project, messages, err := NewProjectRepository(db, "fallback-owner").GetByIDScoped("project", "foreign-admin", "admin")
	if err == nil || project != nil || messages != nil { t.Fatalf("foreign admin received project: %+v %+v %v", project, messages, err) }
	if err := mock.ExpectationsWereMet(); err != nil { t.Fatal(err) }
}

func TestPaginationBoundsAndEmptyShapes(t *testing.T) {
	for _, test := range []struct { name, owner string; limit, offset, wantLimit, wantOffset int }{
		{"scoped default", "owner", 0, -2, 15, 0},
		{"scoped cap", "owner", 200, 2, 100, 2},
		{"unscoped cap", "", 101, -1, 100, 0},
	} {
		t.Run(test.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			if err != nil { t.Fatal(err) }
			t.Cleanup(func() { db.Close() })
			count := mock.ExpectQuery(`SELECT COUNT\(\*\) FROM projects`)
			args := []driver.Value{test.wantLimit, test.wantOffset}
			if test.owner != "" { count.WithArgs(test.owner); args = append([]driver.Value{test.owner}, args...) }
			count.WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
			mock.ExpectQuery("FROM projects").WithArgs(args...).WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "title", "diagram_type", "project_mode", "is_pinned", "created_at", "updated_at"}))
			items, total, err := NewProjectRepository(db, "fallback-owner").GetPaginatedScoped(test.owner, test.limit, test.offset)
			if err != nil || items == nil || len(items) != 0 || total != 0 { t.Fatalf("empty pagination changed: %+v %d %v", items, total, err) }
			if err := mock.ExpectationsWereMet(); err != nil { t.Fatal(err) }
		})
	}
}

func TestProjectAndMessageListsRejectPartialStreams(t *testing.T) {
	for _, table := range []string{"projects", "chat_messages"} {
		t.Run(table, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			if err != nil { t.Fatal(err) }
			t.Cleanup(func() { db.Close() })
			failure := errors.New("row stream failed")
			now := time.Now()
			if table == "projects" {
				mock.ExpectQuery("FROM projects").WithArgs("owner").WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "title", "diagram_type", "project_mode", "metadata", "is_pinned", "current_nodes", "current_edges", "created_at", "updated_at"}).AddRow("project", "owner", "Title", "flowchart", "diagram", []byte(`{}`), false, []byte(`[]`), []byte(`[]`), now, now).RowError(0, failure)).RowsWillBeClosed()
				items, err := NewProjectRepository(db, "fallback-owner").GetAllScoped("owner")
				if !errors.Is(err, failure) || items != nil { t.Fatalf("partial projects: %+v %v", items, err) }
			} else {
				mock.ExpectQuery("FROM chat_messages").WithArgs("project").WillReturnRows(sqlmock.NewRows([]string{"id", "project_id", "role", "content", "target_node_ids", "created_at"}).AddRow("message", "project", "user", "Prompt", nil, now).RowError(0, failure)).RowsWillBeClosed()
				items, err := NewMessageRepository(db).GetByProjectID("project")
				if !errors.Is(err, failure) || items != nil { t.Fatalf("partial messages: %+v %v", items, err) }
			}
			if err := mock.ExpectationsWereMet(); err != nil { t.Fatal(err) }
		})
	}
}

func TestMessageTargetsRemainNullable(t *testing.T) {
	for _, input := range []json.RawMessage{nil, json.RawMessage("null")} {
		db, mock, err := sqlmock.New()
		if err != nil { t.Fatal(err) }
		t.Cleanup(func() { db.Close() })
		mock.ExpectQuery("INSERT INTO chat_messages").WithArgs("project", "user", "Prompt", nil).WillReturnRows(sqlmock.NewRows([]string{"id", "project_id", "role", "content", "target_node_ids", "created_at"}).AddRow("message", "project", "user", "Prompt", nil, time.Now()))
		message, err := NewMessageRepository(db).AppendMessage("project", "user", "Prompt", input)
		if err != nil || message.TargetNodeIDs != nil { t.Fatalf("nullable target changed: %+v %v", message, err) }
		if err := mock.ExpectationsWereMet(); err != nil { t.Fatal(err) }
	}
}

func TestGraphUpdateDistinguishesMissingRowsAndStorageFailures(t *testing.T) {
	for _, scenario := range []string{"missing", "exec", "affected"} {
		t.Run(scenario, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			if err != nil { t.Fatal(err) }
			t.Cleanup(func() { db.Close() })
			failure := errors.New("storage failed")
			expected := mock.ExpectExec("UPDATE projects").WithArgs(json.RawMessage(`[]`), json.RawMessage(`[]`), "project")
			switch scenario {
			case "missing": expected.WillReturnResult(sqlmock.NewResult(0, 0))
			case "exec": expected.WillReturnError(failure)
			case "affected": expected.WillReturnResult(sqlmock.NewErrorResult(failure))
			}
			err = NewProjectRepository(db, "fallback-owner").UpdateGraph("project", nil, nil)
			if err == nil || (scenario != "missing" && !errors.Is(err, failure)) { t.Fatalf("mutation result: %v", err) }
			if err := mock.ExpectationsWereMet(); err != nil { t.Fatal(err) }
		})
	}
}
