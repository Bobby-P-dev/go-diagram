package controller

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Bobby-P-dev/go-diagram.git/internal/modules/identity/authcontext"
	identityservice "github.com/Bobby-P-dev/go-diagram.git/internal/modules/identity/service"
	"github.com/Bobby-P-dev/go-diagram.git/internal/modules/workspace"
	"github.com/Bobby-P-dev/go-diagram.git/internal/modules/workspace/dto"
	"github.com/Bobby-P-dev/go-diagram.git/internal/modules/workspace/service"
)

type fakeService struct {
	err      error
	actor    *workspace.Actor
	id       string
	create   dto.CreateCommentRequest
	settings dto.UpdateSettingsRequest
}

func (svc *fakeService) GetComments(ctx context.Context, actor *workspace.Actor, id string) ([]workspace.DesignComment, error) {
	svc.actor, svc.id = actor, id
	return []workspace.DesignComment{{ID: "comment-1", ProjectID: id, Content: "Spacing"}}, svc.err
}

func (svc *fakeService) CreateComment(ctx context.Context, actor *workspace.Actor, id string, input dto.CreateCommentRequest) (*workspace.DesignComment, error) {
	svc.actor, svc.id, svc.create = actor, id, input
	return &workspace.DesignComment{ID: "comment-1", ProjectID: id, Content: input.Content}, svc.err
}

func (svc *fakeService) UpdateCommentStatus(ctx context.Context, actor *workspace.Actor, id string, input dto.UpdateCommentStatusRequest) (string, error) {
	svc.actor, svc.id = actor, id
	return "resolved", svc.err
}

func (svc *fakeService) DeleteComment(ctx context.Context, actor *workspace.Actor, id string) error {
	svc.actor, svc.id = actor, id
	return svc.err
}

func (svc *fakeService) GetFoundations() ([]workspace.DesignFoundation, error) {
	return []workspace.DesignFoundation{{ID: "foundation-1"}}, svc.err
}

func (svc *fakeService) GetSettings() (*workspace.WorkspaceSetting, error) {
	return &workspace.WorkspaceSetting{ID: "default", Theme: "dark"}, svc.err
}

func (svc *fakeService) UpdateSettings(input dto.UpdateSettingsRequest) (*workspace.WorkspaceSetting, error) {
	svc.settings = input
	return &workspace.WorkspaceSetting{ID: "default", Theme: input.Theme}, svc.err
}

func (svc *fakeService) GetExports(ctx context.Context, actor *workspace.Actor, id string) ([]workspace.ProjectExport, error) {
	svc.actor, svc.id = actor, id
	return []workspace.ProjectExport{{ID: "export-1", ProjectID: id}}, svc.err
}

func TestHandlersPreserveResponses(t *testing.T) {
	for _, test := range []struct {
		name, method, id, body, message string
		status                          int
		handler                         func(*WorkspaceController) http.HandlerFunc
	}{
		{"list comments", "GET", "project-a", "", "Comments retrieved successfully", 200, func(ctrl *WorkspaceController) http.HandlerFunc { return ctrl.GetComments }},
		{"create comment", "POST", "project-a", `{"content":"Spacing","position_x":2,"metadata":{"key":"value"}}`, "Comment created successfully", 201, func(ctrl *WorkspaceController) http.HandlerFunc { return ctrl.CreateComment }},
		{"update comment", "PATCH", "comment-1", `{}`, "Comment status updated", 200, func(ctrl *WorkspaceController) http.HandlerFunc { return ctrl.UpdateCommentStatus }},
		{"delete comment", "DELETE", "comment-1", "", "Comment deleted", 200, func(ctrl *WorkspaceController) http.HandlerFunc { return ctrl.DeleteComment }},
		{"foundations", "GET", "", "", "Foundations retrieved successfully", 200, func(ctrl *WorkspaceController) http.HandlerFunc { return ctrl.GetFoundations }},
		{"settings", "GET", "", "", "Settings retrieved successfully", 200, func(ctrl *WorkspaceController) http.HandlerFunc { return ctrl.GetSettings }},
		{"update settings", "PUT", "", `{"theme":"light","canvas_preferences":{"grid_size":8}}`, "Settings updated successfully", 200, func(ctrl *WorkspaceController) http.HandlerFunc { return ctrl.UpdateSettings }},
		{"exports", "GET", "project-a", "", "Exports retrieved successfully", 200, func(ctrl *WorkspaceController) http.HandlerFunc { return ctrl.GetExports }},
	} {
		t.Run(test.name, func(t *testing.T) {
			svc := &fakeService{}
			ctrl := NewWorkspaceController(svc)
			req := httptest.NewRequest(test.method, "/", strings.NewReader(test.body))
			req.SetPathValue("id", test.id)
			req = req.WithContext(authcontext.WithUser(req.Context(), &identityservice.CredentialRecord{ID: "user-a", Role: "member"}))
			writer := httptest.NewRecorder()
			test.handler(ctrl)(writer, req)
			var response struct {
				Status, Message string
				Data            json.RawMessage
			}
			if err := json.Unmarshal(writer.Body.Bytes(), &response); err != nil {
				t.Fatal(err)
			}
			if writer.Code != test.status || response.Status != "success" || response.Message != test.message || len(response.Data) == 0 || writer.Header().Get("Content-Type") != "application/json" {
				t.Fatalf("response: %d %s", writer.Code, writer.Body.String())
			}
			if test.id != "" && (svc.id != test.id || svc.actor == nil || svc.actor.ID != "user-a" || svc.actor.Role != "member") {
				t.Fatalf("incorrect caller or id: %+v %s", svc.actor, svc.id)
			}
			if test.name == "create comment" && (svc.create.Content != "Spacing" || svc.create.PosX != 2 || string(svc.create.Metadata) != `{"key":"value"}`) {
				t.Fatalf("request changed: %+v", svc.create)
			}
			if test.name == "update settings" && (svc.settings.Theme != "light" || string(svc.settings.CanvasPreferences) != `{"grid_size":8}`) {
				t.Fatalf("settings changed: %+v", svc.settings)
			}
		})
	}
}

func TestHandlersRejectMissingIDsAndMalformedJSON(t *testing.T) {
	ctrl := NewWorkspaceController(nil)
	for _, handler := range []http.HandlerFunc{ctrl.GetComments, ctrl.CreateComment, ctrl.UpdateCommentStatus, ctrl.DeleteComment, ctrl.GetExports} {
		writer := httptest.NewRecorder()
		handler(writer, httptest.NewRequest("GET", "/", nil))
		if writer.Code != 400 {
			t.Fatalf("missing id: %d", writer.Code)
		}
	}
	for _, handler := range []http.HandlerFunc{ctrl.CreateComment, ctrl.UpdateCommentStatus, ctrl.UpdateSettings} {
		writer := httptest.NewRecorder()
		req := httptest.NewRequest("POST", "/", strings.NewReader("{"))
		req.SetPathValue("id", "id")
		handler(writer, req)
		if writer.Code != 400 || !strings.Contains(writer.Body.String(), "Invalid request body") {
			t.Fatalf("bad JSON: %d %s", writer.Code, writer.Body.String())
		}
	}
}

func TestServiceErrorsMapToExistingHTTPContract(t *testing.T) {
	for _, test := range []struct {
		err             error
		code            int
		message, detail string
	}{
		{service.ErrUnauthenticated, 401, "Authentication required", "A valid access credential is required."},
		{service.ErrAccessUnavailable, 500, "Workspace authorization unavailable", "project model is not configured"},
		{service.ErrProjectNotFound, 404, "Project not found", "Project does not exist or is not owned by the caller."},
		{service.ErrCommentNotFound, 404, "Comment not found", "Comment does not exist."},
		{service.ErrContentRequired, 400, "Content is required", "content cannot be empty"},
		{errors.New("storage down"), 500, "Failed to get comments", "storage down"},
	} {
		t.Run(test.message, func(t *testing.T) {
			ctrl := NewWorkspaceController(&fakeService{err: test.err})
			req := httptest.NewRequest("GET", "/", nil)
			req.SetPathValue("id", "project-a")
			writer := httptest.NewRecorder()
			ctrl.GetComments(writer, req)
			var response struct{ Status, Message, Error string }
			if err := json.Unmarshal(writer.Body.Bytes(), &response); err != nil {
				t.Fatal(err)
			}
			if writer.Code != test.code || response.Status != "error" || response.Message != test.message || response.Error != test.detail {
				t.Fatalf("%d %s", writer.Code, writer.Body.String())
			}
		})
	}
}
