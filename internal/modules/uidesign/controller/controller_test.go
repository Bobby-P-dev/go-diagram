package controller_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Bobby-P-dev/go-diagram.git/internal/modules/uidesign"
	uidesigncontroller "github.com/Bobby-P-dev/go-diagram.git/internal/modules/uidesign/controller"
	dto "github.com/Bobby-P-dev/go-diagram.git/internal/modules/uidesign/dto"
	"github.com/Bobby-P-dev/go-diagram.git/internal/modules/uidesign/service"
)

type fakeUIDesignService struct {
	templates      []uidesign.UITemplate
	templatesErr   error
	genResult      *dto.ProjectResponse
	genErr         error
	streamEvents   []string
	streamErr      error
	chatResult     *dto.ProjectResponse
	chatErr        error
	lastChatPrompt string
}

func (s *fakeUIDesignService) GetTemplates(ctx context.Context) ([]uidesign.UITemplate, error) {
	if s.templatesErr != nil {
		return nil, s.templatesErr
	}
	return s.templates, nil
}

func (s *fakeUIDesignService) CreateProjectFromTemplate(ctx context.Context, templateID string) (*dto.ProjectResponse, error) {
	return nil, nil
}

func (s *fakeUIDesignService) GenerateUIDesign(ctx context.Context, req dto.CreateUIDesignRequest) (*dto.ProjectResponse, error) {
	if s.genErr != nil {
		return nil, s.genErr
	}
	return s.genResult, nil
}

func (s *fakeUIDesignService) GenerateUIDesignStream(ctx context.Context, req dto.CreateUIDesignRequest, onEvent func(string, any)) (*dto.ProjectResponse, error) {
	for _, ev := range s.streamEvents {
		onEvent("progress", map[string]string{"step": ev})
	}
	if s.streamErr != nil {
		return nil, s.streamErr
	}
	return s.genResult, nil
}

func (s *fakeUIDesignService) IterateUIDesignWithChat(ctx context.Context, projectID, prompt string, targetedNodeIDs []string) (*dto.ProjectResponse, error) {
	return nil, nil
}

func (s *fakeUIDesignService) IterateUIDesignWithTargetedChat(ctx context.Context, projectID string, req *dto.ChatRequest) (*dto.ProjectResponse, error) {
	if req != nil {
		s.lastChatPrompt = req.Prompt
	}
	if s.chatErr != nil {
		return nil, s.chatErr
	}
	return s.chatResult, nil
}

func TestUIDesignController_GetTemplates(t *testing.T) {
	svc := &fakeUIDesignService{
		templates: []uidesign.UITemplate{
			{ID: "tmpl-1", Name: "SaaS Landing"},
		},
	}
	ctrl := uidesigncontroller.NewUIDesignController(svc)

	req := httptest.NewRequest(http.MethodGet, "/api/ui-design/templates", nil)
	rec := httptest.NewRecorder()
	ctrl.GetTemplates(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "SaaS Landing") {
		t.Fatalf("expected template in body: %s", rec.Body.String())
	}

	// Error case
	svc.templatesErr = errors.New("db error")
	rec = httptest.NewRecorder()
	ctrl.GetTemplates(rec, req)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rec.Code)
	}
}

func TestUIDesignController_Create(t *testing.T) {
	svc := &fakeUIDesignService{
		genResult: &dto.ProjectResponse{ID: "proj-123"},
	}

	// 1. Forbidden when CanGenerateUI is false
	userReader := func(ctx context.Context) *uidesigncontroller.AuthenticatedUser {
		return &uidesigncontroller.AuthenticatedUser{
			ID:            "user-1",
			CanGenerateUI: false,
		}
	}
	ctrl := uidesigncontroller.NewUIDesignControllerWithUserReader(svc, userReader)

	req := httptest.NewRequest(http.MethodPost, "/api/ui-design/generate", strings.NewReader(`{}`))
	rec := httptest.NewRecorder()
	ctrl.Create(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", rec.Code)
	}

	// 2. Invalid JSON payload
	userReader = func(ctx context.Context) *uidesigncontroller.AuthenticatedUser {
		return &uidesigncontroller.AuthenticatedUser{
			ID:            "user-1",
			CanGenerateUI: true,
		}
	}
	ctrl = uidesigncontroller.NewUIDesignControllerWithUserReader(svc, userReader)

	req = httptest.NewRequest(http.MethodPost, "/api/ui-design/generate", strings.NewReader(`{invalid}`))
	rec = httptest.NewRecorder()
	ctrl.Create(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}

	// 3. Success returns 201 Created
	req = httptest.NewRequest(http.MethodPost, "/api/ui-design/generate", strings.NewReader(`{"prompt":"Dashboard"}`))
	rec = httptest.NewRecorder()
	ctrl.Create(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "proj-123") {
		t.Fatalf("expected proj-123 in response: %s", rec.Body.String())
	}
}

func TestUIDesignController_CreateStream(t *testing.T) {
	svc := &fakeUIDesignService{
		genResult:    &dto.ProjectResponse{ID: "proj-stream"},
		streamEvents: []string{"step-1", "step-2"},
	}
	userReader := func(ctx context.Context) *uidesigncontroller.AuthenticatedUser {
		return &uidesigncontroller.AuthenticatedUser{
			ID:            "user-1",
			CanGenerateUI: true,
		}
	}
	ctrl := uidesigncontroller.NewUIDesignControllerWithUserReader(svc, userReader)

	req := httptest.NewRequest(http.MethodPost, "/api/ui-design/generate/stream", strings.NewReader(`{"prompt":"Shop"}`))
	rec := httptest.NewRecorder()
	ctrl.CreateStream(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if !strings.Contains(rec.Header().Get("Content-Type"), "text/event-stream") {
		t.Fatalf("expected text/event-stream, got %s", rec.Header().Get("Content-Type"))
	}
	body := rec.Body.String()
	if !strings.Contains(body, "event: progress") || !strings.Contains(body, "step-1") {
		t.Fatalf("expected progress events in SSE body: %s", body)
	}
}

func TestUIDesignController_Chat(t *testing.T) {
	svc := &fakeUIDesignService{
		chatResult: &dto.ProjectResponse{ID: "proj-chat"},
	}
	userReader := func(ctx context.Context) *uidesigncontroller.AuthenticatedUser {
		return &uidesigncontroller.AuthenticatedUser{
			ID:            "user-1",
			CanGenerateUI: true,
		}
	}
	ctrl := uidesigncontroller.NewUIDesignControllerWithUserReader(svc, userReader)

	// Missing project ID
	req := httptest.NewRequest(http.MethodPost, "/api/ui-design/projects//chat", strings.NewReader(`{}`))
	rec := httptest.NewRecorder()
	ctrl.Chat(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for empty id, got %d", rec.Code)
	}

	// Success with instruction fallback to prompt
	req = httptest.NewRequest(http.MethodPost, "/api/ui-design/projects/proj-1/chat", strings.NewReader(`{"instruction":"Make button green"}`))
	req.SetPathValue("id", "proj-1")
	rec = httptest.NewRecorder()
	ctrl.Chat(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if svc.lastChatPrompt != "Make button green" {
		t.Fatalf("expected prompt to receive instruction fallback, got %q", svc.lastChatPrompt)
	}
}

type fakeJobDispatcher struct {
	initResp    *dto.AsyncJobInitResponse
	initErr     error
	statusResp  *dto.JobStatusResponse
	statusErr   error
	callbackErr error
}

func (d *fakeJobDispatcher) DispatchUIDesignJob(ctx context.Context, req dto.CreateUIDesignRequest, userID string) (*dto.AsyncJobInitResponse, error) {
	if d.initErr != nil {
		return nil, d.initErr
	}
	return d.initResp, nil
}

func (d *fakeJobDispatcher) GetJobStatus(ctx context.Context, jobID, userID string) (*dto.JobStatusResponse, error) {
	if d.statusErr != nil {
		return nil, d.statusErr
	}
	return d.statusResp, nil
}

func (d *fakeJobDispatcher) ProcessJobCallback(ctx context.Context, req dto.JobCallbackRequest) error {
	return d.callbackErr
}

func TestAsyncJobController_CreateAsyncUIDesign(t *testing.T) {
	disp := &fakeJobDispatcher{
		initResp: &dto.AsyncJobInitResponse{
			JobID:     "job-1",
			ProjectID: "proj-1",
			Status:    "queued",
			Message:   "Queued",
		},
	}

	// 1. Unauthenticated (401)
	ctrl := uidesigncontroller.NewAsyncJobController(disp)
	req := httptest.NewRequest(http.MethodPost, "/api/ui-design/generate/async", strings.NewReader(`{}`))
	rec := httptest.NewRecorder()
	ctrl.CreateAsyncUIDesign(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}

	// 2. Forbidden (403)
	userReader := func(ctx context.Context) *uidesigncontroller.AuthenticatedUser {
		return &uidesigncontroller.AuthenticatedUser{ID: "user-1", CanGenerateUI: false}
	}
	ctrl = uidesigncontroller.NewAsyncJobControllerWithOptions(disp, userReader, nil)
	req = httptest.NewRequest(http.MethodPost, "/api/ui-design/generate/async", strings.NewReader(`{}`))
	rec = httptest.NewRecorder()
	ctrl.CreateAsyncUIDesign(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", rec.Code)
	}

	// 3. Success (202 Accepted)
	userReader = func(ctx context.Context) *uidesigncontroller.AuthenticatedUser {
		return &uidesigncontroller.AuthenticatedUser{ID: "user-1", CanGenerateUI: true}
	}
	ctrl = uidesigncontroller.NewAsyncJobControllerWithOptions(disp, userReader, nil)
	req = httptest.NewRequest(http.MethodPost, "/api/ui-design/generate/async", strings.NewReader(`{"prompt":"UI"}`))
	rec = httptest.NewRecorder()
	ctrl.CreateAsyncUIDesign(rec, req)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "job-1") {
		t.Fatalf("expected job-1 in response: %s", rec.Body.String())
	}
}

func TestAsyncJobController_GetJobStatus(t *testing.T) {
	disp := &fakeJobDispatcher{
		statusResp: &dto.JobStatusResponse{
			JobID:  "job-1",
			Status: "completed",
		},
	}
	userReader := func(ctx context.Context) *uidesigncontroller.AuthenticatedUser {
		return &uidesigncontroller.AuthenticatedUser{ID: "user-1", CanGenerateUI: true}
	}
	ctrl := uidesigncontroller.NewAsyncJobControllerWithOptions(disp, userReader, nil)

	// 1. Missing job ID (400)
	req := httptest.NewRequest(http.MethodGet, "/api/jobs/", nil)
	rec := httptest.NewRecorder()
	ctrl.GetJobStatus(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for empty id, got %d", rec.Code)
	}

	// 2. Not found (404)
	disp.statusErr = service.ErrJobNotFound
	req = httptest.NewRequest(http.MethodGet, "/api/jobs/missing", nil)
	req.SetPathValue("id", "missing")
	rec = httptest.NewRecorder()
	ctrl.GetJobStatus(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}

	// 3. Success (200)
	disp.statusErr = nil
	req = httptest.NewRequest(http.MethodGet, "/api/jobs/job-1", nil)
	req.SetPathValue("id", "job-1")
	rec = httptest.NewRecorder()
	ctrl.GetJobStatus(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "completed") {
		t.Fatalf("expected completed status: %s", rec.Body.String())
	}
}

func TestAsyncJobController_HandleJobCallback(t *testing.T) {
	disp := &fakeJobDispatcher{}
	validator := func(request *http.Request) bool {
		return request.Header.Get("X-Internal-Service-Key") == "secret-key"
	}
	ctrl := uidesigncontroller.NewAsyncJobControllerWithOptions(disp, nil, validator)

	// 1. Unauthorized key (401)
	req := httptest.NewRequest(http.MethodPost, "/internal/v1/jobs/callback", strings.NewReader(`{}`))
	req.Header.Set("X-Internal-Service-Key", "wrong-key")
	rec := httptest.NewRecorder()
	ctrl.HandleJobCallback(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for wrong key, got %d", rec.Code)
	}

	// 2. Invalid callback error (400)
	disp.callbackErr = service.ErrInvalidJobCallback
	req = httptest.NewRequest(http.MethodPost, "/internal/v1/jobs/callback", strings.NewReader(`{"job_id":"j1"}`))
	req.Header.Set("X-Internal-Service-Key", "secret-key")
	rec = httptest.NewRecorder()
	ctrl.HandleJobCallback(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for ErrInvalidJobCallback, got %d", rec.Code)
	}

	// 3. Project mismatch error (409)
	disp.callbackErr = service.ErrJobProjectMismatch
	req = httptest.NewRequest(http.MethodPost, "/internal/v1/jobs/callback", strings.NewReader(`{"job_id":"j1"}`))
	req.Header.Set("X-Internal-Service-Key", "secret-key")
	rec = httptest.NewRecorder()
	ctrl.HandleJobCallback(rec, req)
	if rec.Code != http.StatusConflict {
		t.Fatalf("expected 409 for ErrJobProjectMismatch, got %d", rec.Code)
	}

	// 4. Job not found error (404)
	disp.callbackErr = service.ErrJobNotFound
	req = httptest.NewRequest(http.MethodPost, "/internal/v1/jobs/callback", strings.NewReader(`{"job_id":"j1"}`))
	req.Header.Set("X-Internal-Service-Key", "secret-key")
	rec = httptest.NewRecorder()
	ctrl.HandleJobCallback(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for ErrJobNotFound, got %d", rec.Code)
	}

	// 5. Success (200)
	disp.callbackErr = nil
	req = httptest.NewRequest(http.MethodPost, "/internal/v1/jobs/callback", strings.NewReader(`{"job_id":"j1"}`))
	req.Header.Set("X-Internal-Service-Key", "secret-key")
	rec = httptest.NewRecorder()
	ctrl.HandleJobCallback(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "persisted") {
		t.Fatalf("expected persisted response: %s", rec.Body.String())
	}
}
