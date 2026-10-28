package controller

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Bobby-P-dev/go-diagram.git/internal/modules/identity/authcontext"
	identityservice "github.com/Bobby-P-dev/go-diagram.git/internal/modules/identity/service"
	"github.com/Bobby-P-dev/go-diagram.git/internal/modules/sharing"
	"github.com/Bobby-P-dev/go-diagram.git/internal/modules/sharing/dto"
)

type mockShareService struct {
	createFunc func(ctx context.Context, projectID, title string) (*sharing.Share, error)
	statusFunc func(ctx context.Context, projectID string) (*sharing.Share, error)
	revokeFunc func(ctx context.Context, projectID string) error
	tokenFunc  func(ctx context.Context, token string) (*sharing.Share, *sharing.Project, []sharing.Message, error)
	forkFunc   func(ctx context.Context, token, newTitle, userID string) (*sharing.Project, error)
}

func (m *mockShareService) CreateOrGetShare(ctx context.Context, projectID, title string) (*sharing.Share, error) {
	if m.createFunc != nil {
		return m.createFunc(ctx, projectID, title)
	}
	return nil, nil
}

func (m *mockShareService) GetShareByProjectID(ctx context.Context, projectID string) (*sharing.Share, error) {
	if m.statusFunc != nil {
		return m.statusFunc(ctx, projectID)
	}
	return nil, nil
}

func (m *mockShareService) RevokeShare(ctx context.Context, projectID string) error {
	if m.revokeFunc != nil {
		return m.revokeFunc(ctx, projectID)
	}
	return nil
}

func (m *mockShareService) GetShareByToken(ctx context.Context, token string) (*sharing.Share, *sharing.Project, []sharing.Message, error) {
	if m.tokenFunc != nil {
		return m.tokenFunc(ctx, token)
	}
	return nil, nil, nil, nil
}

func (m *mockShareService) ForkSharedProjectWithUser(ctx context.Context, token, newTitle, userID string) (*sharing.Project, error) {
	if m.forkFunc != nil {
		return m.forkFunc(ctx, token, newTitle, userID)
	}
	return nil, nil
}

func TestShareController_CreateShare(t *testing.T) {
	now := time.Now()
	service := &mockShareService{
		createFunc: func(ctx context.Context, projectID, title string) (*sharing.Share, error) {
			return &sharing.Share{
				ID:         "sh-1",
				ShareToken: "tok-abc",
				ProjectID:  projectID,
				Title:      title,
				IsActive:   true,
				ViewCount:  0,
				CreatedAt:  now,
				UpdatedAt:  now,
			}, nil
		},
	}
	reader := ProjectAccessReaderFunc(func(ctx context.Context, projectID, userID string) (string, error) {
		if projectID == "unauthorized-proj" {
			return "", errors.New("project not found or unauthorized")
		}
		return "Project Title", nil
	})
	ctrl := NewShareController(service, reader, nil)

	req := httptest.NewRequest("POST", "/api/projects//share", nil)
	w := httptest.NewRecorder()
	ctrl.CreateShare(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}

	req = httptest.NewRequest("POST", "/api/projects/unauthorized-proj/share", nil)
	req.SetPathValue("id", "unauthorized-proj")
	w = httptest.NewRecorder()
	ctrl.CreateShare(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}

	req = httptest.NewRequest("POST", "/api/projects/proj-1/share", nil)
	req.SetPathValue("id", "proj-1")
	user := &identityservice.CredentialRecord{ID: "usr-1"}
	req = req.WithContext(authcontext.WithUser(req.Context(), user))
	w = httptest.NewRecorder()
	ctrl.CreateShare(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var env struct {
		Data dto.ProjectShareDTO `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &env); err != nil {
		t.Fatal(err)
	}
	if env.Data.ShareToken != "tok-abc" || env.Data.ShareURL != "/share/tok-abc" {
		t.Fatalf("unexpected response: %+v", env.Data)
	}
}

func TestShareController_GetShareStatus(t *testing.T) {
	now := time.Now()
	service := &mockShareService{
		statusFunc: func(ctx context.Context, projectID string) (*sharing.Share, error) {
			if projectID == "shared-proj" {
				return &sharing.Share{ID: "sh-1", ShareToken: "tok-1", IsActive: true, CreatedAt: now, UpdatedAt: now}, nil
			}
			return nil, nil
		},
	}
	ctrl := NewShareController(service, nil, nil)

	req := httptest.NewRequest("GET", "/api/projects/not-shared/share", nil)
	req.SetPathValue("id", "not-shared")
	w := httptest.NewRecorder()
	ctrl.GetShareStatus(w, req)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"is_active":false`) {
		t.Fatalf("expected inactive status, got %d: %s", w.Code, w.Body.String())
	}

	req = httptest.NewRequest("GET", "/api/projects/shared-proj/share", nil)
	req.SetPathValue("id", "shared-proj")
	w = httptest.NewRecorder()
	ctrl.GetShareStatus(w, req)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"tok-1"`) {
		t.Fatalf("expected active share, got %d: %s", w.Code, w.Body.String())
	}
}

func TestShareController_ForkSharedProject(t *testing.T) {
	service := &mockShareService{
		forkFunc: func(ctx context.Context, token, newTitle, userID string) (*sharing.Project, error) {
			return &sharing.Project{ID: "cloned-proj-1", Title: newTitle}, nil
		},
	}
	loader := ProjectDetailLoaderFunc(func(ctx context.Context, id string) (interface{}, error) {
		return map[string]string{"id": id, "title": "Full Cloned Project"}, nil
	})
	ctrl := NewShareController(service, nil, loader)

	req := httptest.NewRequest("POST", "/api/shared/token-123/fork", strings.NewReader(`{"title": "Forked Flow"}`))
	req.SetPathValue("token", "token-123")
	user := &identityservice.CredentialRecord{ID: "usr-1"}
	req = req.WithContext(authcontext.WithUser(req.Context(), user))
	w := httptest.NewRecorder()
	ctrl.ForkSharedProject(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created, got %d: %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "Full Cloned Project") {
		t.Fatalf("expected full project response: %s", w.Body.String())
	}
}

func TestSharingControllerRequestContextAndOwnership(t *testing.T) {
	base, cancel := context.WithCancel(context.Background())
	defer cancel()
	ctx := authcontext.WithUser(base, &identityservice.CredentialRecord{ID: "owner-1"})
	cancel()
	check := func(actual context.Context) {
		t.Helper()
		if actual != ctx || !errors.Is(actual.Err(), context.Canceled) {
			t.Fatal("controller dropped the request context")
		}
	}
	var calls []string
	stub := &mockShareService{
		createFunc: func(actual context.Context, id, title string) (*sharing.Share, error) {
			check(actual)
			calls = append(calls, "create")
			if id != "project-1" || title != "Project Title" {
				t.Fatalf("unexpected create arguments: %s %s", id, title)
			}
			return &sharing.Share{ID: "share-1", IsActive: true}, nil
		},
		statusFunc: func(actual context.Context, id string) (*sharing.Share, error) {
			check(actual)
			calls = append(calls, "status")
			return nil, nil
		},
		revokeFunc: func(actual context.Context, id string) error {
			check(actual)
			calls = append(calls, "revoke")
			return nil
		},
		tokenFunc: func(actual context.Context, token string) (*sharing.Share, *sharing.Project, []sharing.Message, error) {
			check(actual)
			calls = append(calls, "public")
			return &sharing.Share{}, &sharing.Project{}, nil, nil
		},
		forkFunc: func(actual context.Context, token, title, owner string) (*sharing.Project, error) {
			check(actual)
			calls = append(calls, "fork")
			if owner != "owner-1" || title != "" || token != "token-1" {
				t.Fatalf("invalid fork arguments: %s %s %s", token, title, owner)
			}
			return &sharing.Project{ID: "clone-1"}, nil
		},
	}
	reader := ProjectAccessReaderFunc(func(actual context.Context, id, owner string) (string, error) {
		check(actual)
		calls = append(calls, "access")
		if owner != "owner-1" || id != "project-1" {
			t.Fatalf("ownership scope lost: %s %s", id, owner)
		}
		return "Project Title", nil
	})
	loader := ProjectDetailLoaderFunc(func(actual context.Context, id string) (interface{}, error) {
		check(actual)
		calls = append(calls, "detail")
		return &dto.ProjectResponse{ID: id}, nil
	})
	controller := NewShareController(stub, reader, loader)
	for _, handler := range []http.HandlerFunc{controller.CreateShare, controller.GetShareStatus, controller.RevokeShare, controller.GetSharedProject, controller.ForkSharedProject} {
		request := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("{invalid")).WithContext(ctx)
		request.SetPathValue("id", "project-1")
		request.SetPathValue("token", "token-1")
		response := httptest.NewRecorder()
		handler(response, request)
		if response.Code >= 400 {
			t.Fatalf("request failed: %d %s", response.Code, response.Body.String())
		}
	}
	if len(calls) != 9 {
		t.Fatal(calls)
	}
	rejected := NewShareController(&mockShareService{}, ProjectAccessReaderFunc(func(actual context.Context, id, owner string) (string, error) {
		check(actual)
		return "", errors.New("project not found or unauthorized")
	}), nil)
	for _, handler := range []http.HandlerFunc{rejected.CreateShare, rejected.GetShareStatus, rejected.RevokeShare} {
		request := httptest.NewRequest(http.MethodPost, "/", nil).WithContext(ctx)
		request.SetPathValue("id", "project-1")
		response := httptest.NewRecorder()
		handler(response, request)
		if response.Code != http.StatusNotFound {
			t.Fatalf("ownership failure not preserved: %d %s", response.Code, response.Body.String())
		}
	}
}
