package projectgateway_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/Bobby-P-dev/go-diagram.git/internal/integration/projectgateway"
	"github.com/Bobby-P-dev/go-diagram.git/internal/modules/diagram"
)

type mockProjectRepo struct {
	project *diagram.Project
	err     error
	calls   int
	ctxCalls int
}

func (m *mockProjectRepo) Create(title, diagramType string, nodes, edges json.RawMessage) (*diagram.Project, error) {
	return nil, nil
}
func (m *mockProjectRepo) CreateWithMode(title, diagramType, projectMode string, metadata, nodes, edges json.RawMessage) (*diagram.Project, error) {
	return nil, nil
}
func (m *mockProjectRepo) CreateWithUser(userID, title, diagramType, projectMode string, metadata, nodes, edges json.RawMessage) (*diagram.Project, error) {
	return nil, nil
}
func (m *mockProjectRepo) GetByID(id string) (*diagram.Project, []diagram.ChatMessage, error) {
	return nil, nil, nil
}
func (m *mockProjectRepo) GetByIDScoped(id, userID, userRole string) (*diagram.Project, []diagram.ChatMessage, error) {
	m.calls++
	return m.project, nil, m.err
}
func (m *mockProjectRepo) GetByIDScopedContext(ctx context.Context, id, userID, userRole string) (*diagram.Project, []diagram.ChatMessage, error) {
	m.ctxCalls++
	if err := ctx.Err(); err != nil {
		return nil, nil, err
	}
	return m.project, nil, m.err
}
func (m *mockProjectRepo) UpdateGraph(id string, nodes, edges json.RawMessage) error {
	return nil
}
func (m *mockProjectRepo) GetAll() ([]diagram.Project, error) {
	return nil, nil
}
func (m *mockProjectRepo) GetAllScoped(userID string) ([]diagram.Project, error) {
	return nil, nil
}
func (m *mockProjectRepo) GetPaginated(limit, offset int) ([]diagram.ProjectSummary, int, error) {
	return nil, 0, nil
}
func (m *mockProjectRepo) GetPaginatedScoped(userID string, limit, offset int) ([]diagram.ProjectSummary, int, error) {
	return nil, 0, nil
}
func (m *mockProjectRepo) TogglePin(id string) (bool, error) {
	return false, nil
}

func TestWorkspaceAccessReader(t *testing.T) {
	if projectgateway.NewWorkspaceProjectAccessReader(nil) != nil {
		t.Fatal("nil model must return nil")
	}

	repo := &mockProjectRepo{project: &diagram.Project{ID: "proj-1"}}
	reader := projectgateway.NewWorkspaceProjectAccessReader(repo)

	if err := reader.CheckProjectAccess(context.Background(), "proj-1", "user-1", "admin"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if repo.ctxCalls != 1 {
		t.Fatalf("expected context-aware call, got ctxCalls=%d", repo.ctxCalls)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := reader.CheckProjectAccess(ctx, "proj-1", "user-1", "admin"); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
}
