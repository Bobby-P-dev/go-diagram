package controller

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Bobby-P-dev/go-diagram.git/internal/modules/diagram/dto"
	"github.com/Bobby-P-dev/go-diagram.git/internal/modules/identity/authcontext"
	identityservice "github.com/Bobby-P-dev/go-diagram.git/internal/modules/identity/service"
)

type fakeDiagramService struct {
	createFn       func(userID string, req dto.CreateProjectRequest) (*dto.ProjectResponse, error)
	chatFn         func(projectID string, req dto.ChatRequest, userID string) (*dto.ProjectResponse, error)
	getByIDFn      func(id, userID, userRole string) (*dto.ProjectResponse, error)
	updateGraphFn  func(id, userID string, nodes, edges json.RawMessage) error
	listProjectsFn func(userID string, limit, offset int) (*dto.PaginatedProjectsResponse, error)
	togglePinFn    func(id, userID string) (*dto.TogglePinResponse, error)
	getVersionsFn  func(projectID, userID string) ([]dto.DiagramVersionDTO, error)
	rollbackFn     func(projectID, versionID, userID string) (*dto.ProjectResponse, error)
	listTplFn      func() ([]dto.TemplateDTO, error)
}

func (fake *fakeDiagramService) CreateNewProjectForUser(userID string, req dto.CreateProjectRequest) (*dto.ProjectResponse, error) {
	if fake.createFn != nil {
		return fake.createFn(userID, req)
	}
	return &dto.ProjectResponse{ID: "proj-1", Title: "New Project"}, nil
}

func (fake *fakeDiagramService) SendChatMessageScoped(projectID string, req dto.ChatRequest, userID string) (*dto.ProjectResponse, error) {
	if fake.chatFn != nil {
		return fake.chatFn(projectID, req, userID)
	}
	return &dto.ProjectResponse{ID: projectID, Title: "Chat Updated"}, nil
}

func (fake *fakeDiagramService) GetProjectByIDScoped(id, userID, userRole string) (*dto.ProjectResponse, error) {
	if fake.getByIDFn != nil {
		return fake.getByIDFn(id, userID, userRole)
	}
	return &dto.ProjectResponse{ID: id, Title: "Existing Project"}, nil
}

func (fake *fakeDiagramService) UpdateProjectGraphScoped(id, userID string, nodes, edges json.RawMessage) error {
	if fake.updateGraphFn != nil {
		return fake.updateGraphFn(id, userID, nodes, edges)
	}
	return nil
}

func (fake *fakeDiagramService) ListProjectsForUser(userID string, limit, offset int) (*dto.PaginatedProjectsResponse, error) {
	if fake.listProjectsFn != nil {
		return fake.listProjectsFn(userID, limit, offset)
	}
	return &dto.PaginatedProjectsResponse{
		Items:      []dto.ProjectSidebarItem{{ID: "p-1", Title: "Test"}},
		TotalCount: 1,
		Limit:      limit,
		Offset:     offset,
	}, nil
}

func (fake *fakeDiagramService) ToggleProjectPinScoped(id, userID string) (*dto.TogglePinResponse, error) {
	if fake.togglePinFn != nil {
		return fake.togglePinFn(id, userID)
	}
	return &dto.TogglePinResponse{ID: id, IsPinned: true}, nil
}

func (fake *fakeDiagramService) GetProjectVersionsScoped(projectID, userID string) ([]dto.DiagramVersionDTO, error) {
	if fake.getVersionsFn != nil {
		return fake.getVersionsFn(projectID, userID)
	}
	return []dto.DiagramVersionDTO{{ID: "v-1", VersionNumber: 1}}, nil
}

func (fake *fakeDiagramService) RollbackToVersionScoped(projectID, versionID, userID string) (*dto.ProjectResponse, error) {
	if fake.rollbackFn != nil {
		return fake.rollbackFn(projectID, versionID, userID)
	}
	return &dto.ProjectResponse{ID: projectID, Title: "Rolled Back"}, nil
}

func (fake *fakeDiagramService) ListTemplates() ([]dto.TemplateDTO, error) {
	if fake.listTplFn != nil {
		return fake.listTplFn()
	}
	return []dto.TemplateDTO{{ID: "t-1", Name: "Flowchart Template"}}, nil
}

func TestProjectController_Create(test *testing.T) {
	svc := &fakeDiagramService{}
	ctrl := NewProjectController(svc)

	test.Run("Invalid body returns 400", func(test *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/projects", strings.NewReader("{invalid-json"))
		rec := httptest.NewRecorder()
		ctrl.Create(rec, req)

		if rec.Code != http.StatusBadRequest {
			test.Errorf("expected 400, got %d", rec.Code)
		}
	})

	test.Run("Forbidden when user cannot generate diagram", func(test *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/projects", strings.NewReader(`{"prompt":"test"}`))
		req = req.WithContext(authcontext.WithUser(req.Context(), &identityservice.CredentialRecord{
			ID:                 "user-restricted",
			CanGenerateDiagram: false,
		}))
		rec := httptest.NewRecorder()
		ctrl.Create(rec, req)

		if rec.Code != http.StatusForbidden {
			test.Errorf("expected 403, got %d", rec.Code)
		}
	})

	test.Run("Service failure returns 500", func(test *testing.T) {
		svc.createFn = func(userID string, req dto.CreateProjectRequest) (*dto.ProjectResponse, error) {
			return nil, errors.New("database error")
		}
		defer func() { svc.createFn = nil }()

		req := httptest.NewRequest(http.MethodPost, "/api/projects", strings.NewReader(`{"prompt":"test"}`))
		rec := httptest.NewRecorder()
		ctrl.Create(rec, req)

		if rec.Code != http.StatusInternalServerError {
			test.Errorf("expected 500, got %d", rec.Code)
		}
	})

	test.Run("Success returns 201", func(test *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/projects", strings.NewReader(`{"prompt":"test"}`))
		rec := httptest.NewRecorder()
		ctrl.Create(rec, req)

		if rec.Code != http.StatusCreated {
			test.Errorf("expected 201, got %d", rec.Code)
		}
	})
}

func TestProjectController_Chat(test *testing.T) {
	svc := &fakeDiagramService{}
	ctrl := NewProjectController(svc)

	test.Run("Missing project ID returns 400", func(test *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/projects//chat", strings.NewReader(`{"prompt":"add node"}`))
		rec := httptest.NewRecorder()
		ctrl.Chat(rec, req)

		if rec.Code != http.StatusBadRequest {
			test.Errorf("expected 400, got %d", rec.Code)
		}
	})

	test.Run("Forbidden when cannot generate diagram", func(test *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/projects/p-1/chat", strings.NewReader(`{"prompt":"add node"}`))
		req.SetPathValue("id", "p-1")
		req = req.WithContext(authcontext.WithUser(req.Context(), &identityservice.CredentialRecord{
			ID:                 "user-restricted",
			CanGenerateDiagram: false,
		}))
		rec := httptest.NewRecorder()
		ctrl.Chat(rec, req)

		if rec.Code != http.StatusForbidden {
			test.Errorf("expected 403, got %d", rec.Code)
		}
	})

	test.Run("Invalid body returns 400", func(test *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/projects/p-1/chat", strings.NewReader(`{invalid`))
		req.SetPathValue("id", "p-1")
		rec := httptest.NewRecorder()
		ctrl.Chat(rec, req)

		if rec.Code != http.StatusBadRequest {
			test.Errorf("expected 400, got %d", rec.Code)
		}
	})

	test.Run("Service failure returns 500", func(test *testing.T) {
		svc.chatFn = func(projectID string, req dto.ChatRequest, userID string) (*dto.ProjectResponse, error) {
			return nil, errors.New("AI unavailable")
		}
		defer func() { svc.chatFn = nil }()

		req := httptest.NewRequest(http.MethodPost, "/api/projects/p-1/chat", strings.NewReader(`{"prompt":"hello"}`))
		req.SetPathValue("id", "p-1")
		rec := httptest.NewRecorder()
		ctrl.Chat(rec, req)

		if rec.Code != http.StatusInternalServerError {
			test.Errorf("expected 500, got %d", rec.Code)
		}
	})

	test.Run("Success returns 200", func(test *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/projects/p-1/chat", strings.NewReader(`{"prompt":"hello"}`))
		req.SetPathValue("id", "p-1")
		rec := httptest.NewRecorder()
		ctrl.Chat(rec, req)

		if rec.Code != http.StatusOK {
			test.Errorf("expected 200, got %d", rec.Code)
		}
	})
}

func TestProjectController_GetByIDAndGetAll(test *testing.T) {
	svc := &fakeDiagramService{}
	ctrl := NewProjectController(svc)

	test.Run("GetByID missing id returns 400", func(test *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/projects/", nil)
		rec := httptest.NewRecorder()
		ctrl.GetByID(rec, req)

		if rec.Code != http.StatusBadRequest {
			test.Errorf("expected 400, got %d", rec.Code)
		}
	})

	test.Run("GetByID service error returns 404", func(test *testing.T) {
		svc.getByIDFn = func(id, userID, userRole string) (*dto.ProjectResponse, error) {
			return nil, errors.New("project not found")
		}
		defer func() { svc.getByIDFn = nil }()

		req := httptest.NewRequest(http.MethodGet, "/api/projects/p-missing", nil)
		req.SetPathValue("id", "p-missing")
		rec := httptest.NewRecorder()
		ctrl.GetByID(rec, req)

		if rec.Code != http.StatusNotFound {
			test.Errorf("expected 404, got %d", rec.Code)
		}
	})

	test.Run("GetByID success returns 200", func(test *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/projects/p-1", nil)
		req.SetPathValue("id", "p-1")
		rec := httptest.NewRecorder()
		ctrl.GetByID(rec, req)

		if rec.Code != http.StatusOK {
			test.Errorf("expected 200, got %d", rec.Code)
		}
	})

	test.Run("GetAll query params and success", func(test *testing.T) {
		var capturedLimit, capturedOffset int
		svc.listProjectsFn = func(userID string, limit, offset int) (*dto.PaginatedProjectsResponse, error) {
			capturedLimit = limit
			capturedOffset = offset
			return &dto.PaginatedProjectsResponse{Items: []dto.ProjectSidebarItem{}, TotalCount: 0}, nil
		}
		defer func() { svc.listProjectsFn = nil }()

		req := httptest.NewRequest(http.MethodGet, "/api/projects?limit=25&offset=50", nil)
		rec := httptest.NewRecorder()
		ctrl.GetAll(rec, req)

		if rec.Code != http.StatusOK {
			test.Errorf("expected 200, got %d", rec.Code)
		}
		if capturedLimit != 25 || capturedOffset != 50 {
			test.Errorf("expected limit 25, offset 50, got limit %d, offset %d", capturedLimit, capturedOffset)
		}
	})

	test.Run("GetAll service error returns 500", func(test *testing.T) {
		svc.listProjectsFn = func(userID string, limit, offset int) (*dto.PaginatedProjectsResponse, error) {
			return nil, errors.New("db error")
		}
		defer func() { svc.listProjectsFn = nil }()

		req := httptest.NewRequest(http.MethodGet, "/api/projects", nil)
		rec := httptest.NewRecorder()
		ctrl.GetAll(rec, req)

		if rec.Code != http.StatusInternalServerError {
			test.Errorf("expected 500, got %d", rec.Code)
		}
	})
}

func TestProjectController_UpdateGraphAndPin(test *testing.T) {
	svc := &fakeDiagramService{}
	ctrl := NewProjectController(svc)

	test.Run("UpdateGraph missing id returns 400", func(test *testing.T) {
		req := httptest.NewRequest(http.MethodPut, "/api/projects//graph", strings.NewReader(`{}`))
		rec := httptest.NewRecorder()
		ctrl.UpdateGraph(rec, req)

		if rec.Code != http.StatusBadRequest {
			test.Errorf("expected 400, got %d", rec.Code)
		}
	})

	test.Run("UpdateGraph invalid body returns 400", func(test *testing.T) {
		req := httptest.NewRequest(http.MethodPut, "/api/projects/p-1/graph", strings.NewReader(`invalid`))
		req.SetPathValue("id", "p-1")
		rec := httptest.NewRecorder()
		ctrl.UpdateGraph(rec, req)

		if rec.Code != http.StatusBadRequest {
			test.Errorf("expected 400, got %d", rec.Code)
		}
	})

	test.Run("UpdateGraph service error returns 404", func(test *testing.T) {
		svc.updateGraphFn = func(id, userID string, nodes, edges json.RawMessage) error {
			return errors.New("unauthorized or missing")
		}
		defer func() { svc.updateGraphFn = nil }()

		req := httptest.NewRequest(http.MethodPut, "/api/projects/p-1/graph", strings.NewReader(`{"nodes":[],"edges":[]}`))
		req.SetPathValue("id", "p-1")
		rec := httptest.NewRecorder()
		ctrl.UpdateGraph(rec, req)

		if rec.Code != http.StatusNotFound {
			test.Errorf("expected 404, got %d", rec.Code)
		}
	})

	test.Run("UpdateGraph success returns 200", func(test *testing.T) {
		req := httptest.NewRequest(http.MethodPut, "/api/projects/p-1/graph", strings.NewReader(`{"nodes":[],"edges":[]}`))
		req.SetPathValue("id", "p-1")
		rec := httptest.NewRecorder()
		ctrl.UpdateGraph(rec, req)

		if rec.Code != http.StatusOK {
			test.Errorf("expected 200, got %d", rec.Code)
		}
	})

	test.Run("TogglePin missing id returns 400", func(test *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/projects//pin", nil)
		rec := httptest.NewRecorder()
		ctrl.TogglePin(rec, req)

		if rec.Code != http.StatusBadRequest {
			test.Errorf("expected 400, got %d", rec.Code)
		}
	})

	test.Run("TogglePin service failure returns 500", func(test *testing.T) {
		svc.togglePinFn = func(id, userID string) (*dto.TogglePinResponse, error) {
			return nil, errors.New("failed")
		}
		defer func() { svc.togglePinFn = nil }()

		req := httptest.NewRequest(http.MethodPost, "/api/projects/p-1/pin", nil)
		req.SetPathValue("id", "p-1")
		rec := httptest.NewRecorder()
		ctrl.TogglePin(rec, req)

		if rec.Code != http.StatusInternalServerError {
			test.Errorf("expected 500, got %d", rec.Code)
		}
	})

	test.Run("TogglePin success returns 200", func(test *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/projects/p-1/pin", nil)
		req.SetPathValue("id", "p-1")
		rec := httptest.NewRecorder()
		ctrl.TogglePin(rec, req)

		if rec.Code != http.StatusOK {
			test.Errorf("expected 200, got %d", rec.Code)
		}
	})
}

func TestProjectController_VersionsRollbackAndTemplates(test *testing.T) {
	svc := &fakeDiagramService{}
	ctrl := NewProjectController(svc)

	test.Run("GetVersions missing id returns 400", func(test *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/projects//versions", nil)
		rec := httptest.NewRecorder()
		ctrl.GetVersions(rec, req)

		if rec.Code != http.StatusBadRequest {
			test.Errorf("expected 400, got %d", rec.Code)
		}
	})

	test.Run("GetVersions service failure returns 500", func(test *testing.T) {
		svc.getVersionsFn = func(projectID, userID string) ([]dto.DiagramVersionDTO, error) {
			return nil, errors.New("db error")
		}
		defer func() { svc.getVersionsFn = nil }()

		req := httptest.NewRequest(http.MethodGet, "/api/projects/p-1/versions", nil)
		req.SetPathValue("id", "p-1")
		rec := httptest.NewRecorder()
		ctrl.GetVersions(rec, req)

		if rec.Code != http.StatusInternalServerError {
			test.Errorf("expected 500, got %d", rec.Code)
		}
	})

	test.Run("GetVersions success returns 200", func(test *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/projects/p-1/versions", nil)
		req.SetPathValue("id", "p-1")
		rec := httptest.NewRecorder()
		ctrl.GetVersions(rec, req)

		if rec.Code != http.StatusOK {
			test.Errorf("expected 200, got %d", rec.Code)
		}
	})

	test.Run("Rollback missing path parameters returns 400", func(test *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/projects/p-1/rollback/", nil)
		req.SetPathValue("id", "p-1")
		rec := httptest.NewRecorder()
		ctrl.Rollback(rec, req)

		if rec.Code != http.StatusBadRequest {
			test.Errorf("expected 400, got %d", rec.Code)
		}
	})

	test.Run("Rollback service failure returns 500", func(test *testing.T) {
		svc.rollbackFn = func(projectID, versionID, userID string) (*dto.ProjectResponse, error) {
			return nil, errors.New("version mismatch")
		}
		defer func() { svc.rollbackFn = nil }()

		req := httptest.NewRequest(http.MethodPost, "/api/projects/p-1/rollback/v-2", nil)
		req.SetPathValue("id", "p-1")
		req.SetPathValue("versionId", "v-2")
		rec := httptest.NewRecorder()
		ctrl.Rollback(rec, req)

		if rec.Code != http.StatusInternalServerError {
			test.Errorf("expected 500, got %d", rec.Code)
		}
	})

	test.Run("Rollback success returns 200", func(test *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/projects/p-1/rollback/v-2", nil)
		req.SetPathValue("id", "p-1")
		req.SetPathValue("versionId", "v-2")
		rec := httptest.NewRecorder()
		ctrl.Rollback(rec, req)

		if rec.Code != http.StatusOK {
			test.Errorf("expected 200, got %d", rec.Code)
		}
	})

	test.Run("GetTemplates failure returns 500", func(test *testing.T) {
		svc.listTplFn = func() ([]dto.TemplateDTO, error) {
			return nil, errors.New("template error")
		}
		defer func() { svc.listTplFn = nil }()

		req := httptest.NewRequest(http.MethodGet, "/api/templates", nil)
		rec := httptest.NewRecorder()
		ctrl.GetTemplates(rec, req)

		if rec.Code != http.StatusInternalServerError {
			test.Errorf("expected 500, got %d", rec.Code)
		}
	})

	test.Run("GetTemplates success returns 200", func(test *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/templates", nil)
		rec := httptest.NewRecorder()
		ctrl.GetTemplates(rec, req)

		if rec.Code != http.StatusOK {
			test.Errorf("expected 200, got %d", rec.Code)
		}
	})
}

func TestDefaultUserReader(test *testing.T) {
	if DefaultUserReader(nil) != nil {
		test.Fatal("expected nil for nil context")
	}
	if DefaultUserReader(context.Background()) != nil {
		test.Fatal("expected nil for context without user")
	}

	user := &identityservice.CredentialRecord{
		ID:                 "usr-42",
		Role:               "designer",
		CanGenerateDiagram: true,
	}
	ctx := authcontext.WithUser(context.Background(), user)
	retrieved := DefaultUserReader(ctx)
	if retrieved == nil || retrieved.ID != "usr-42" || !retrieved.CanGenerateDiagram || retrieved.Role != "designer" {
		test.Fatalf("unexpected user retrieved: %+v", retrieved)
	}
}
