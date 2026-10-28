package repository

import (
 "context"
	"database/sql"
	"encoding/json"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
 "github.com/Bobby-P-dev/go-diagram.git/internal/modules/diagram"
)

func TestProjectRepository_CreateAndGet(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()

	now := time.Now()
	repo := NewProjectRepository(db, "a0000000-0000-0000-0000-000000000001")

	mock.ExpectQuery(regexp.QuoteMeta(`INSERT INTO projects (user_id, title, diagram_type, project_mode, metadata, current_nodes, current_edges, is_pinned, created_at, updated_at)`)).
		WithArgs("a0000000-0000-0000-0000-000000000001", "Architecture", "flowchart", "diagram", json.RawMessage("{}"), json.RawMessage("[]"), json.RawMessage("[]")).
		WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "title", "diagram_type", "project_mode", "metadata", "is_pinned", "current_nodes", "current_edges", "created_at", "updated_at"}).
			AddRow("proj-1", "a0000000-0000-0000-0000-000000000001", "Architecture", "flowchart", "diagram", []byte("{}"), false, []byte("[]"), []byte("[]"), now, now))

	p, err := repo.Create("Architecture", "flowchart", nil, nil)
	if err != nil || p.ID != "proj-1" {
		t.Fatalf("failed to create project: %v, %+v", err, p)
	}

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT id, COALESCE(user_id, 'a0000000-0000-0000-0000-000000000001'::uuid), title, diagram_type, COALESCE(project_mode, 'diagram'), COALESCE(metadata, '{}'::jsonb), is_pinned, current_nodes, current_edges, created_at, updated_at FROM projects WHERE id = $1`)).
		WithArgs("proj-1").
		WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "title", "diagram_type", "project_mode", "metadata", "is_pinned", "current_nodes", "current_edges", "created_at", "updated_at"}).
			AddRow("proj-1", "user-1", "Architecture", "flowchart", "diagram", []byte("{}"), false, []byte("[]"), []byte("[]"), now, now))

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT id, project_id, role, content, target_node_ids, created_at FROM chat_messages WHERE project_id = $1 ORDER BY created_at ASC`)).
		WithArgs("proj-1").
		WillReturnRows(sqlmock.NewRows([]string{"id", "project_id", "role", "content", "target_node_ids", "created_at"}).
			AddRow("msg-1", "proj-1", "user", "Hello", nil, now))

	p, msgs, err := repo.GetByIDScoped("proj-1", "user-1", "member")
	if err != nil || p.ID != "proj-1" || len(msgs) != 1 {
		t.Fatalf("failed to get project scoped: %v, len msgs: %d", err, len(msgs))
	}

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT id, COALESCE(user_id, 'a0000000-0000-0000-0000-000000000001'::uuid), title, diagram_type, COALESCE(project_mode, 'diagram'), COALESCE(metadata, '{}'::jsonb), is_pinned, current_nodes, current_edges, created_at, updated_at FROM projects WHERE id = $1`)).
		WithArgs("proj-1").
		WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "title", "diagram_type", "project_mode", "metadata", "is_pinned", "current_nodes", "current_edges", "created_at", "updated_at"}).
			AddRow("proj-1", "user-1", "Architecture", "flowchart", "diagram", []byte("{}"), false, []byte("[]"), []byte("[]"), now, now))

	_, _, err = repo.GetByIDScoped("proj-1", "user-2", "member")
	if err == nil {
		t.Fatal("expected unauthorized error for foreign user")
	}

	mock.ExpectExec(regexp.QuoteMeta(`UPDATE projects SET current_nodes = $1, current_edges = $2, updated_at = NOW() WHERE id = $3`)).
		WithArgs(json.RawMessage(`[{"id":"1"}]`), json.RawMessage("[]"), "proj-1").
		WillReturnResult(sqlmock.NewResult(1, 1))

	if err := repo.UpdateGraph("proj-1", json.RawMessage(`[{"id":"1"}]`), json.RawMessage("[]")); err != nil {
		t.Fatalf("failed to update graph: %v", err)
	}

	mock.ExpectQuery(regexp.QuoteMeta(`UPDATE projects SET is_pinned = NOT is_pinned, updated_at = NOW() WHERE id = $1 RETURNING is_pinned`)).
		WithArgs("proj-1").
		WillReturnRows(sqlmock.NewRows([]string{"is_pinned"}).AddRow(true))

	isPinned, err := repo.TogglePin("proj-1")
	if err != nil || !isPinned {
		t.Fatalf("failed to toggle pin: %v, isPinned: %v", err, isPinned)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestProjectRepository_QueriesAndPagination(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()

	now := time.Now()
	repo := NewProjectRepository(db, "a0000000-0000-0000-0000-000000000001")

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT id, COALESCE(user_id, 'a0000000-0000-0000-0000-000000000001'::uuid), title, diagram_type, COALESCE(project_mode, 'diagram'), COALESCE(metadata, '{}'::jsonb), is_pinned, current_nodes, current_edges, created_at, updated_at FROM projects WHERE user_id = $1 ORDER BY is_pinned DESC, updated_at DESC`)).
		WithArgs("user-1").
		WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "title", "diagram_type", "project_mode", "metadata", "is_pinned", "current_nodes", "current_edges", "created_at", "updated_at"}).
			AddRow("proj-1", "user-1", "Architecture", "flowchart", "diagram", []byte("{}"), false, []byte("[]"), []byte("[]"), now, now))

	all, err := repo.GetAllScoped("user-1")
	if err != nil || len(all) != 1 {
		t.Fatalf("failed to get all scoped: %v", err)
	}

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT COUNT(*) FROM projects WHERE user_id = $1`)).
		WithArgs("user-1").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT id, COALESCE(user_id, 'a0000000-0000-0000-0000-000000000001'::uuid), title, diagram_type, COALESCE(project_mode, 'diagram'), is_pinned, created_at, updated_at FROM projects WHERE user_id = $1 ORDER BY is_pinned DESC, updated_at DESC LIMIT $2 OFFSET $3`)).
		WithArgs("user-1", 10, 0).
		WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "title", "diagram_type", "project_mode", "is_pinned", "created_at", "updated_at"}).
			AddRow("proj-1", "user-1", "Architecture", "flowchart", "diagram", false, now, now))

	items, total, err := repo.GetPaginatedScoped("user-1", 10, 0)
	if err != nil || total != 1 || len(items) != 1 {
		t.Fatalf("failed to get paginated scoped: %v, total: %d, items: %d", err, total, len(items))
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestVersionRepository_CRUD(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()

	now := time.Now()
	repo := NewVersionRepository(db)

	triggerMsgID := "msg-1"
	mock.ExpectQuery(regexp.QuoteMeta(`INSERT INTO diagram_versions (project_id, version_number, change_summary, nodes, edges, trigger_message_id, created_at)`)).
		WithArgs("proj-1", 1, "Initial version", json.RawMessage("[]"), json.RawMessage("[]"), &triggerMsgID, sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"id", "created_at"}).AddRow("ver-1", now))

	ver, err := repo.CreateVersion("proj-1", 1, "Initial version", json.RawMessage("[]"), json.RawMessage("[]"), &triggerMsgID)
	if err != nil || ver.ID != "ver-1" {
		t.Fatalf("failed to create version: %v", err)
	}

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT id, project_id, version_number, change_summary, nodes, edges, trigger_message_id, created_at FROM diagram_versions WHERE project_id = $1 ORDER BY version_number DESC`)).
		WithArgs("proj-1").
		WillReturnRows(sqlmock.NewRows([]string{"id", "project_id", "version_number", "change_summary", "nodes", "edges", "trigger_message_id", "created_at"}).
			AddRow("ver-1", "proj-1", 1, "Initial version", []byte("[]"), []byte("[]"), "msg-1", now))

	versions, err := repo.GetVersionsByProjectID("proj-1")
	if err != nil || len(versions) != 1 {
		t.Fatalf("failed to get versions: %v", err)
	}

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT id, project_id, version_number, change_summary, nodes, edges, trigger_message_id, created_at FROM diagram_versions WHERE id = $1`)).
		WithArgs("ver-1").
		WillReturnRows(sqlmock.NewRows([]string{"id", "project_id", "version_number", "change_summary", "nodes", "edges", "trigger_message_id", "created_at"}).
			AddRow("ver-1", "proj-1", 1, "Initial version", []byte("[]"), []byte("[]"), "msg-1", now))

	v, err := repo.GetVersionByID("ver-1")
	if err != nil || v.ID != "ver-1" {
		t.Fatalf("failed to get version: %v", err)
	}

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT id, project_id, version_number, change_summary, nodes, edges, trigger_message_id, created_at FROM diagram_versions WHERE id = $1`)).
		WithArgs("missing").
		WillReturnError(sql.ErrNoRows)

	_, err = repo.GetVersionByID("missing")
	if err == nil {
		t.Fatal("expected error on missing version")
	}

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT COALESCE(MAX(version_number), 0) FROM diagram_versions WHERE project_id = $1`)).
		WithArgs("proj-1").
		WillReturnRows(sqlmock.NewRows([]string{"max"}).AddRow(2))

	maxVer, err := repo.GetLatestVersionNumber("proj-1")
	if err != nil || maxVer != 2 {
		t.Fatalf("failed to get latest version: %v, maxVer: %d", err, maxVer)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestMessageAndTemplateRepositories(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()

	now := time.Now()
	msgRepo := NewMessageRepository(db)
	tplRepo := NewTemplateRepository(db)

	mock.ExpectQuery(regexp.QuoteMeta(`INSERT INTO chat_messages (project_id, role, content, target_node_ids, created_at)`)).
		WithArgs("proj-1", "user", "Please add a database node", json.RawMessage(`["node-1"]`)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "project_id", "role", "content", "target_node_ids", "created_at"}).
			AddRow("msg-1", "proj-1", "user", "Please add a database node", []byte(`["node-1"]`), now))

	msg, err := msgRepo.AppendMessage("proj-1", "user", "Please add a database node", json.RawMessage(`["node-1"]`))
	if err != nil || msg.ID != "msg-1" {
		t.Fatalf("failed to append message: %v", err)
	}

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT id, project_id, role, content, target_node_ids, created_at FROM chat_messages WHERE project_id = $1 ORDER BY created_at ASC`)).
		WithArgs("proj-1").
		WillReturnRows(sqlmock.NewRows([]string{"id", "project_id", "role", "content", "target_node_ids", "created_at"}).
			AddRow("msg-1", "proj-1", "user", "Please add a database node", []byte(`["node-1"]`), now))

	msgs, err := msgRepo.GetByProjectID("proj-1")
	if err != nil || len(msgs) != 1 {
		t.Fatalf("failed to get messages: %v", err)
	}

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT id, name, category, description, diagram_type, nodes, edges, is_featured, created_at FROM diagram_templates ORDER BY is_featured DESC, name ASC`)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "category", "description", "diagram_type", "nodes", "edges", "is_featured", "created_at"}).
			AddRow("tpl-1", "Microservices", "backend", "Microservices arch", "architecture", []byte("[]"), []byte("[]"), true, now))

	templates, err := tplRepo.GetAllTemplates()
	if err != nil || len(templates) != 1 || templates[0].ID != "tpl-1" {
		t.Fatalf("failed to get templates: %v", err)
	}

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT id, name, category, description, diagram_type, nodes, edges, is_featured, created_at FROM diagram_templates WHERE id = $1`)).
		WithArgs("tpl-1").
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "category", "description", "diagram_type", "nodes", "edges", "is_featured", "created_at"}).
			AddRow("tpl-1", "Microservices", "backend", "Microservices arch", "architecture", []byte("[]"), []byte("[]"), true, now))

	tpl, err := tplRepo.GetTemplateByID("tpl-1")
	if err != nil || tpl.ID != "tpl-1" {
		t.Fatalf("failed to get template by id: %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestCloneRepository_LoadAndClone(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()

	now := time.Now()
	repo := NewCloneRepository(db)

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT id, title, diagram_type, COALESCE(project_mode, 'diagram'), COALESCE(metadata, '{}'::jsonb), is_pinned, current_nodes, current_edges, created_at, updated_at FROM projects WHERE id = $1`)).
		WithArgs("proj-1").
		WillReturnRows(sqlmock.NewRows([]string{"id", "title", "diagram_type", "project_mode", "metadata", "is_pinned", "current_nodes", "current_edges", "created_at", "updated_at"}).
			AddRow("proj-1", "Original Title", "flowchart", "diagram", []byte("{}"), false, []byte("[]"), []byte("[]"), now, now))

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT id, project_id, role, content, target_node_ids, created_at FROM chat_messages WHERE project_id = $1 ORDER BY created_at ASC`)).
		WithArgs("proj-1").
		WillReturnRows(sqlmock.NewRows([]string{"id", "project_id", "role", "content", "target_node_ids", "created_at"}).
			AddRow("msg-1", "proj-1", "user", "Prompt", []byte(`["node-1"]`), now))

	proj, msgs, err := repo.LoadProjectForClone(context.Background(), "proj-1")
	if err != nil || proj.ID != "proj-1" || len(msgs) != 1 {
		t.Fatalf("failed to load project for clone: %v, len: %d", err, len(msgs))
	}

	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(`INSERT INTO projects (user_id, title, diagram_type, project_mode, metadata, current_nodes, current_edges, is_pinned, created_at, updated_at)`)).
		WithArgs("user-new", "Forked Title", "flowchart", "diagram", json.RawMessage("{}"), json.RawMessage("[]"), json.RawMessage("[]")).
		WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "title", "diagram_type", "project_mode", "metadata", "is_pinned", "current_nodes", "current_edges", "created_at", "updated_at"}).
			AddRow("proj-forked", "user-new", "Forked Title", "flowchart", "diagram", []byte("{}"), false, []byte("[]"), []byte("[]"), now, now))

	mock.ExpectExec(regexp.QuoteMeta(`INSERT INTO chat_messages (project_id, role, content, target_node_ids, created_at)`)).
		WithArgs("proj-forked", "user", "Prompt", json.RawMessage(`["node-1"]`), now).
		WillReturnResult(sqlmock.NewResult(1, 1))

	mock.ExpectExec(regexp.QuoteMeta(`INSERT INTO diagram_versions (project_id, version_number, change_summary, nodes, edges, created_at)`)).
		WithArgs("proj-forked", "Cloned from shared project: Original Title", json.RawMessage("[]"), json.RawMessage("[]")).
		WillReturnResult(sqlmock.NewResult(1, 1))

	mock.ExpectCommit()

	cloneInput := diagram.CloneInput{
		Original: *proj,
		Messages: msgs,
		Title:    "Forked Title",
		UserID:   "user-new",
	}

	cloned, err := repo.CloneProject(context.Background(), cloneInput)
	if err != nil || cloned.ID != "proj-forked" {
		t.Fatalf("failed to clone project: %v, %+v", err, cloned)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestProjectRepository_CreateWithModeAndGetAll(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()

	now := time.Now()
	repo := NewProjectRepository(db, "a0000000-0000-0000-0000-000000000001")

	mock.ExpectQuery(regexp.QuoteMeta(`INSERT INTO projects (user_id, title, diagram_type, project_mode, metadata, current_nodes, current_edges, is_pinned, created_at, updated_at)`)).
		WithArgs("a0000000-0000-0000-0000-000000000001", "Mode Title", "ui_design", "ui_design", json.RawMessage(`{"a":1}`), json.RawMessage("[]"), json.RawMessage("[]")).
		WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "title", "diagram_type", "project_mode", "metadata", "is_pinned", "current_nodes", "current_edges", "created_at", "updated_at"}).
			AddRow("proj-mode", "a0000000-0000-0000-0000-000000000001", "Mode Title", "ui_design", "ui_design", []byte(`{"a":1}`), false, []byte("[]"), []byte("[]"), now, now))

	p, err := repo.CreateWithMode("Mode Title", "ui_design", "ui_design", json.RawMessage(`{"a":1}`), nil, nil)
	if err != nil || p.ID != "proj-mode" {
		t.Fatalf("failed CreateWithMode: %v", err)
	}

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT id, COALESCE(user_id, 'a0000000-0000-0000-0000-000000000001'::uuid), title, diagram_type, COALESCE(project_mode, 'diagram'), COALESCE(metadata, '{}'::jsonb), is_pinned, current_nodes, current_edges, created_at, updated_at FROM projects WHERE id = $1`)).
		WithArgs("proj-mode").
		WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "title", "diagram_type", "project_mode", "metadata", "is_pinned", "current_nodes", "current_edges", "created_at", "updated_at"}).
			AddRow("proj-mode", "a0000000-0000-0000-0000-000000000001", "Mode Title", "ui_design", "ui_design", []byte(`{"a":1}`), false, []byte("[]"), []byte("[]"), now, now))

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT id, project_id, role, content, target_node_ids, created_at FROM chat_messages WHERE project_id = $1 ORDER BY created_at ASC`)).
		WithArgs("proj-mode").
		WillReturnRows(sqlmock.NewRows([]string{"id", "project_id", "role", "content", "target_node_ids", "created_at"}))

	p2, _, err := repo.GetByID("proj-mode")
	if err != nil || p2.ID != "proj-mode" {
		t.Fatalf("failed GetByID: %v", err)
	}

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT id, COALESCE(user_id, 'a0000000-0000-0000-0000-000000000001'::uuid), title, diagram_type, COALESCE(project_mode, 'diagram'), COALESCE(metadata, '{}'::jsonb), is_pinned, current_nodes, current_edges, created_at, updated_at FROM projects ORDER BY is_pinned DESC, updated_at DESC`)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "title", "diagram_type", "project_mode", "metadata", "is_pinned", "current_nodes", "current_edges", "created_at", "updated_at"}).
			AddRow("proj-1", "user-1", "Arch", "flowchart", "diagram", []byte("{}"), false, []byte("[]"), []byte("[]"), now, now))

	all, err := repo.GetAll()
	if err != nil || len(all) != 1 {
		t.Fatalf("failed GetAll: %v", err)
	}

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT COUNT(*) FROM projects`)).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT id, COALESCE(user_id, 'a0000000-0000-0000-0000-000000000001'::uuid), title, diagram_type, COALESCE(project_mode, 'diagram'), is_pinned, created_at, updated_at FROM projects ORDER BY is_pinned DESC, updated_at DESC LIMIT $1 OFFSET $2`)).
		WithArgs(15, 0).
		WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "title", "diagram_type", "project_mode", "is_pinned", "created_at", "updated_at"}).
			AddRow("proj-1", "user-1", "Arch", "flowchart", "diagram", false, now, now))

	items, total, err := repo.GetPaginated(0, -1)
	if err != nil || total != 1 || len(items) != 1 {
		t.Fatalf("failed GetPaginated: %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}
