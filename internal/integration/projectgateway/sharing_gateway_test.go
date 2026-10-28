package projectgateway_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/Bobby-P-dev/go-diagram.git/internal/integration/projectgateway"
	"github.com/Bobby-P-dev/go-diagram.git/internal/modules/sharing"
	"github.com/DATA-DOG/go-sqlmock"
)

func TestSharingProjectGateway(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	gateway := projectgateway.NewSQLProjectGateway(db)
	if gateway == nil {
		t.Fatal("expected non-nil gateway")
	}

	t.Run("Context canceled on LoadSharedProject", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		if _, _, err := gateway.LoadSharedProject(ctx, "proj-1"); !errors.Is(err, context.Canceled) {
			t.Fatalf("expected context.Canceled, got %v", err)
		}
	})

	t.Run("Context canceled on CloneSharedProject", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		if _, err := gateway.CloneSharedProject(ctx, sharing.CloneInput{}); !errors.Is(err, context.Canceled) {
			t.Fatalf("expected context.Canceled, got %v", err)
		}
	})

	t.Run("LoadSharedProject success", func(t *testing.T) {
		now := time.Now()
		mock.ExpectQuery(`SELECT id, title, diagram_type`).WithArgs("proj-1").
			WillReturnRows(sqlmock.NewRows([]string{
				"id", "title", "diagram_type", "project_mode", "metadata", "is_pinned", "current_nodes", "current_edges", "created_at", "updated_at",
			}).AddRow("proj-1", "Test Project", "flowchart", "diagram", []byte(`{}`), false, []byte(`[]`), []byte(`[]`), now, now))

		mock.ExpectQuery(`SELECT id, project_id, role, content, target_node_ids, created_at`).WithArgs("proj-1").
			WillReturnRows(sqlmock.NewRows([]string{
				"id", "project_id", "role", "content", "target_node_ids", "created_at",
			}).AddRow("msg-1", "proj-1", "user", "hello", json.RawMessage(`[]`), now))

		project, messages, err := gateway.LoadSharedProject(context.Background(), "proj-1")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if project.ID != "proj-1" || len(messages) != 1 {
			t.Fatalf("unexpected result: project=%+v, messages=%d", project, len(messages))
		}
	})
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
