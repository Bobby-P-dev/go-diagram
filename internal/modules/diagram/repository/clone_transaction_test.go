package repository

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/Bobby-P-dev/go-diagram.git/internal/modules/diagram"
	"github.com/DATA-DOG/go-sqlmock"
)

func TestCloneProjectTransaction(t *testing.T) {
	for _, stage := range []string{"success", "begin", "project", "message", "snapshot", "commit"} {
		t.Run(stage, func(t *testing.T) {
			database, mock, err := sqlmock.New()
			if err != nil {
				t.Fatal(err)
			}
			defer database.Close()
			failure := errors.New("storage failed")
			now := time.Now()
			input := diagram.CloneInput{
				Original: diagram.Project{
					ID: "original", Title: "Source", DiagramType: "ui_design", ProjectMode: "ui_design",
					Metadata: json.RawMessage(`{"mode":"ui_design"}`), CurrentNodes: json.RawMessage(`[{"id":"frame-1"}]`), CurrentEdges: json.RawMessage(`[]`),
				},
				Messages: []diagram.ChatMessage{{Role: "user", Content: "Create UI", TargetNodeIDs: json.RawMessage(`["frame-1"]`), CreatedAt: now}},
				Title:    "Copy", UserID: "owner-1",
			}
			begin := mock.ExpectBegin()
			if stage == "begin" {
				begin.WillReturnError(failure)
			} else {
				project := mock.ExpectQuery(`INSERT INTO projects`).WithArgs(
					input.UserID, input.Title, input.Original.DiagramType, input.Original.ProjectMode,
					[]byte(input.Original.Metadata), []byte(input.Original.CurrentNodes), []byte(input.Original.CurrentEdges),
				)
				if stage == "project" {
					project.WillReturnError(failure)
					mock.ExpectRollback()
				} else {
					project.WillReturnRows(sqlmock.NewRows([]string{
						"id", "user_id", "title", "diagram_type", "project_mode", "metadata", "is_pinned", "current_nodes", "current_edges", "created_at", "updated_at",
					}).AddRow("clone-1", input.UserID, input.Title, input.Original.DiagramType, input.Original.ProjectMode,
						[]byte(input.Original.Metadata), false, []byte(input.Original.CurrentNodes), []byte(input.Original.CurrentEdges), now, now))
					message := mock.ExpectExec(`INSERT INTO chat_messages`).WithArgs("clone-1", "user", "Create UI", []byte(input.Messages[0].TargetNodeIDs), now)
					if stage == "message" {
						message.WillReturnError(failure)
						mock.ExpectRollback()
					} else {
						message.WillReturnResult(sqlmock.NewResult(1, 1))
						snapshot := mock.ExpectExec(`INSERT INTO diagram_versions`).WithArgs("clone-1", "Cloned from shared project: Source", []byte(input.Original.CurrentNodes), []byte(input.Original.CurrentEdges))
						if stage == "snapshot" {
							snapshot.WillReturnError(failure)
							mock.ExpectRollback()
						} else {
							snapshot.WillReturnResult(sqlmock.NewResult(1, 1))
							commit := mock.ExpectCommit()
							if stage == "commit" {
								commit.WillReturnError(failure)
							}
						}
					}
				}
			}
			cloned, err := NewCloneRepository(database).CloneProject(context.Background(), input)
			if stage == "success" {
				if err != nil || cloned == nil || cloned.ID != "clone-1" || cloned.UserID != "owner-1" || cloned.IsPinned || cloned.ProjectMode != "ui_design" {
					t.Fatalf("unexpected clone: %+v %v", cloned, err)
				}
			} else if cloned != nil || !errors.Is(err, failure) {
				t.Fatalf("failed %s must not return a successful clone: %+v %v", stage, cloned, err)
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestLoadProjectForCloneRejectsPartialMessageRows(t *testing.T) {
	database, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	now := time.Now()
	mock.ExpectQuery(`SELECT id, title, diagram_type`).WithArgs("project-1").
		WillReturnRows(sqlmock.NewRows([]string{"id", "title", "diagram_type", "project_mode", "metadata", "is_pinned", "current_nodes", "current_edges", "created_at", "updated_at"}).
			AddRow("project-1", "Source", "flowchart", "diagram", []byte(`{}`), false, []byte(`[]`), []byte(`[]`), now, now))
	failure := errors.New("row stream failed")
	mock.ExpectQuery(`SELECT id, project_id, role, content, target_node_ids, created_at`).WithArgs("project-1").
		WillReturnRows(sqlmock.NewRows([]string{"id", "project_id", "role", "content", "target_node_ids", "created_at"}).
			AddRow("message-1", "project-1", "user", "First", nil, now).
			AddRow("message-2", "project-1", "user", "Second", nil, now).RowError(1, failure)).RowsWillBeClosed()
	project, messages, err := NewCloneRepository(database).LoadProjectForClone(context.Background(), "project-1")
	if project != nil || messages != nil || !errors.Is(err, failure) || !strings.Contains(err.Error(), "failed to read chat messages") {
		t.Fatalf("partial content must not be forkable: %+v %+v %v", project, messages, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestSharingProjectGatewayHonorsCanceledContext(t *testing.T) {
	database, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	gateway := NewCloneRepository(database)
	if _, _, err := gateway.LoadProjectForClone(ctx, "project-1"); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if _, err := gateway.CloneProject(ctx, diagram.CloneInput{}); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
