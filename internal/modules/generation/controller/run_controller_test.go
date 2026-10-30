package controller

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Bobby-P-dev/go-diagram.git/internal/modules/generation"
	"github.com/Bobby-P-dev/go-diagram.git/internal/modules/identity/authcontext"
	identityservice "github.com/Bobby-P-dev/go-diagram.git/internal/modules/identity/service"
)

func TestDiagramRunResponsesHidePrivateSnapshots(test *testing.T) {
	run := &generation.Run{
		ID: "run-diagram", OutputType: "erd", Status: "succeeded", CommittedArtifactID: "artifact-diagram", CommittedRevisionID: "revision-diagram",
		Prompt: "private-prompt", BaseGraph: &generation.DiagramGraph{Nodes: []generation.DiagramNode{{ID: "private-base"}}},
	}
	controller := NewRunController(&fakeRunService{getFn: func(context.Context, generation.Scope, string) (*generation.Run, error) { return run, nil }})
	request := func() *http.Request {
		request := withUserContext(httptest.NewRequest(http.MethodGet, "/run", nil), "user")
		request.SetPathValue("projectId", "project")
		request.SetPathValue("runId", run.ID)
		return request
	}
	response := httptest.NewRecorder()
	controller.Get(response, request())
	var envelope struct {
		Data generation.Run `json:"data"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &envelope); err != nil || response.Code != http.StatusOK || envelope.Data.OutputType != "erd" {
		test.Fatalf("get response: %d %s %v", response.Code, response.Body.String(), err)
	}
	if strings.Contains(response.Body.String(), "private-") || strings.Contains(response.Body.String(), "base_graph") {
		test.Fatal("GET exposed private snapshot")
	}
	response = httptest.NewRecorder()
	controller.Events(response, request())
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), "event: complete") || !strings.Contains(response.Body.String(), "revision-diagram") {
		test.Fatalf("events: %d %s", response.Code, response.Body.String())
	}
	if strings.Contains(response.Body.String(), "private-") || strings.Contains(response.Body.String(), "base_graph") {
		test.Fatal("SSE exposed private snapshot")
	}
}

type fakeRunService struct {
	startFn  func(context.Context, generation.Scope, generation.StartInput, string) (*generation.Run, error)
	getFn    func(context.Context, generation.Scope, string) (*generation.Run, error)
	cancelFn func(context.Context, generation.Scope, string) (*generation.Run, error)
}

func (service *fakeRunService) Start(ctx context.Context, scope generation.Scope, input generation.StartInput, key string) (*generation.Run, error) {
	if service.startFn != nil {
		return service.startFn(ctx, scope, input, key)
	}
	return &generation.Run{ID: "run-1", Status: "queued"}, nil
}

func (service *fakeRunService) Get(ctx context.Context, scope generation.Scope, id string) (*generation.Run, error) {
	if service.getFn != nil {
		return service.getFn(ctx, scope, id)
	}
	return &generation.Run{ID: id, Status: "running"}, nil
}

func (service *fakeRunService) Cancel(ctx context.Context, scope generation.Scope, id string) (*generation.Run, error) {
	if service.cancelFn != nil {
		return service.cancelFn(ctx, scope, id)
	}
	return &generation.Run{ID: id, Status: "canceled"}, nil
}

func withUserContext(request *http.Request, userID string) *http.Request {
	return request.WithContext(authcontext.WithUser(request.Context(), &identityservice.CredentialRecord{ID: userID, CredentialKey: "test", Role: "user"}))
}

func TestRunController_Start(test *testing.T) {
	validBody := `{"trigger_message_id":"11111111-1111-1111-1111-111111111111","output_type":"prd","output_title":"PRD Draft","context":{"mode":"explicit"}}`

	test.Run("unauthenticated", func(test *testing.T) {
		svc := &fakeRunService{}
		ctrl := NewRunController(svc)
		req := httptest.NewRequest("POST", "/api/v2/projects/22222222-2222-2222-2222-222222222222/threads/33333333-3333-3333-3333-333333333333/generations", strings.NewReader(validBody))
		req.Header.Set("Idempotency-Key", "idemp-key-1")
		req.SetPathValue("projectId", "22222222-2222-2222-2222-222222222222")
		req.SetPathValue("threadId", "33333333-3333-3333-3333-333333333333")
		rec := httptest.NewRecorder()
		ctrl.Start(rec, req)
		if rec.Code != http.StatusUnauthorized {
			test.Fatalf("expected 401, got %d", rec.Code)
		}
	})

	test.Run("missing idempotency key", func(test *testing.T) {
		svc := &fakeRunService{}
		ctrl := NewRunController(svc)
		req := withUserContext(httptest.NewRequest("POST", "/api/v2/projects/22222222-2222-2222-2222-222222222222/threads/33333333-3333-3333-3333-333333333333/generations", strings.NewReader(validBody)), "user-1")
		req.SetPathValue("projectId", "22222222-2222-2222-2222-222222222222")
		req.SetPathValue("threadId", "33333333-3333-3333-3333-333333333333")
		rec := httptest.NewRecorder()
		ctrl.Start(rec, req)
		if rec.Code != http.StatusBadRequest {
			test.Fatalf("expected 400, got %d", rec.Code)
		}
	})

	test.Run("successful accept", func(test *testing.T) {
		svc := &fakeRunService{
			startFn: func(_ context.Context, scope generation.Scope, input generation.StartInput, key string) (*generation.Run, error) {
				return &generation.Run{ID: "run-abc", Status: "queued", OutputType: input.OutputType}, nil
			},
		}
		ctrl := NewRunController(svc)
		req := withUserContext(httptest.NewRequest("POST", "/api/v2/projects/22222222-2222-2222-2222-222222222222/threads/33333333-3333-3333-3333-333333333333/generations", strings.NewReader(validBody)), "user-1")
		req.Header.Set("Idempotency-Key", "idemp-key-1")
		req.SetPathValue("projectId", "22222222-2222-2222-2222-222222222222")
		req.SetPathValue("threadId", "33333333-3333-3333-3333-333333333333")
		rec := httptest.NewRecorder()
		ctrl.Start(rec, req)
		if rec.Code != http.StatusAccepted {
			test.Fatalf("expected 202 Accepted, got %d: %service", rec.Code, rec.Body.String())
		}
	})

	test.Run("idempotency conflict", func(test *testing.T) {
		svc := &fakeRunService{
			startFn: func(_ context.Context, _ generation.Scope, _ generation.StartInput, _ string) (*generation.Run, error) {
				return nil, generation.ErrIdempotencyConflict
			},
		}
		ctrl := NewRunController(svc)
		req := withUserContext(httptest.NewRequest("POST", "/api/v2/projects/22222222-2222-2222-2222-222222222222/threads/33333333-3333-3333-3333-333333333333/generations", strings.NewReader(validBody)), "user-1")
		req.Header.Set("Idempotency-Key", "idemp-key-1")
		req.SetPathValue("projectId", "22222222-2222-2222-2222-222222222222")
		req.SetPathValue("threadId", "33333333-3333-3333-3333-333333333333")
		rec := httptest.NewRecorder()
		ctrl.Start(rec, req)
		if rec.Code != http.StatusConflict {
			test.Fatalf("expected 409 Conflict, got %d", rec.Code)
		}
	})

	test.Run("body too large", func(test *testing.T) {
		svc := &fakeRunService{}
		ctrl := NewRunController(svc)
		largeBody := `{"trigger_message_id":"1","output_type":"` + strings.Repeat("a", 70*1024) + `"}`
		req := withUserContext(httptest.NewRequest("POST", "/api/v2/projects/22222222-2222-2222-2222-222222222222/threads/33333333-3333-3333-3333-333333333333/generations", strings.NewReader(largeBody)), "user-1")
		req.Header.Set("Idempotency-Key", "idemp-key-1")
		req.SetPathValue("projectId", "22222222-2222-2222-2222-222222222222")
		req.SetPathValue("threadId", "33333333-3333-3333-3333-333333333333")
		rec := httptest.NewRecorder()
		ctrl.Start(rec, req)
		if rec.Code != http.StatusRequestEntityTooLarge {
			test.Fatalf("expected 413, got %d", rec.Code)
		}
	})

	test.Run("null body rejected", func(test *testing.T) {
		svc := &fakeRunService{}
		ctrl := NewRunController(svc)
		req := withUserContext(httptest.NewRequest("POST", "/api/v2/projects/22222222-2222-2222-2222-222222222222/threads/33333333-3333-3333-3333-333333333333/generations", strings.NewReader("null")), "user-1")
		req.Header.Set("Idempotency-Key", "idemp-key-1")
		req.SetPathValue("projectId", "22222222-2222-2222-2222-222222222222")
		req.SetPathValue("threadId", "33333333-3333-3333-3333-333333333333")
		rec := httptest.NewRecorder()
		ctrl.Start(rec, req)
		if rec.Code != http.StatusBadRequest {
			test.Fatalf("expected 400, got %d", rec.Code)
		}
	})
}

func TestRunController_Get(test *testing.T) {
	test.Run("success", func(test *testing.T) {
		svc := &fakeRunService{
			getFn: func(_ context.Context, scope generation.Scope, id string) (*generation.Run, error) {
				return &generation.Run{ID: id, Status: "running"}, nil
			},
		}
		ctrl := NewRunController(svc)
		req := withUserContext(httptest.NewRequest("GET", "/api/v2/projects/p-1/generations/request-1", nil), "user-1")
		req.SetPathValue("projectId", "p-1")
		req.SetPathValue("runId", "request-1")
		rec := httptest.NewRecorder()
		ctrl.Get(rec, req)
		if rec.Code != http.StatusOK {
			test.Fatalf("expected 200, got %d", rec.Code)
		}
	})

	test.Run("not found", func(test *testing.T) {
		svc := &fakeRunService{
			getFn: func(_ context.Context, _ generation.Scope, _ string) (*generation.Run, error) {
				return nil, generation.ErrNotFound
			},
		}
		ctrl := NewRunController(svc)
		req := withUserContext(httptest.NewRequest("GET", "/api/v2/projects/p-1/generations/request-1", nil), "user-1")
		req.SetPathValue("projectId", "p-1")
		req.SetPathValue("runId", "request-1")
		rec := httptest.NewRecorder()
		ctrl.Get(rec, req)
		if rec.Code != http.StatusNotFound {
			test.Fatalf("expected 404, got %d", rec.Code)
		}
	})
}

func TestRunController_Cancel(test *testing.T) {
	test.Run("success", func(test *testing.T) {
		svc := &fakeRunService{
			cancelFn: func(_ context.Context, _ generation.Scope, id string) (*generation.Run, error) {
				return &generation.Run{ID: id, Status: "canceled"}, nil
			},
		}
		ctrl := NewRunController(svc)
		req := withUserContext(httptest.NewRequest("POST", "/api/v2/projects/p-1/generations/request-1/cancel", nil), "user-1")
		req.SetPathValue("projectId", "p-1")
		req.SetPathValue("runId", "request-1")
		rec := httptest.NewRecorder()
		ctrl.Cancel(rec, req)
		if rec.Code != http.StatusOK {
			test.Fatalf("expected 200, got %d", rec.Code)
		}
	})

	test.Run("terminal conflict", func(test *testing.T) {
		svc := &fakeRunService{
			cancelFn: func(_ context.Context, _ generation.Scope, _ string) (*generation.Run, error) {
				return nil, generation.ErrRunTerminal
			},
		}
		ctrl := NewRunController(svc)
		req := withUserContext(httptest.NewRequest("POST", "/api/v2/projects/p-1/generations/request-1/cancel", nil), "user-1")
		req.SetPathValue("projectId", "p-1")
		req.SetPathValue("runId", "request-1")
		rec := httptest.NewRecorder()
		ctrl.Cancel(rec, req)
		if rec.Code != http.StatusConflict {
			test.Fatalf("expected 409 Conflict, got %d", rec.Code)
		}
	})
}

func TestRunController_Events(test *testing.T) {
	test.Run("terminal run emits single event and closes", func(test *testing.T) {
		completedAt := time.Now()
		svc := &fakeRunService{
			getFn: func(_ context.Context, _ generation.Scope, id string) (*generation.Run, error) {
				return &generation.Run{
					ID:                  id,
					Status:              "succeeded",
					CommittedArtifactID: "art-1",
					CommittedRevisionID: "rev-1",
					CompletedAt:         &completedAt,
				}, nil
			},
		}
		ctrl := NewRunController(svc)
		req := withUserContext(httptest.NewRequest("GET", "/api/v2/projects/p-1/generations/request-1/events", nil), "user-1")
		req.SetPathValue("projectId", "p-1")
		req.SetPathValue("runId", "request-1")
		rec := httptest.NewRecorder()
		ctrl.Events(rec, req)
		if rec.Code != http.StatusOK {
			test.Fatalf("expected 200, got %d", rec.Code)
		}
		if !strings.Contains(rec.Header().Get("Content-Type"), "text/event-stream") {
			test.Fatalf("expected text/event-stream, got %service", rec.Header().Get("Content-Type"))
		}
		body := rec.Body.String()
		if !strings.Contains(body, "event: complete") {
			test.Fatalf("expected event: complete input body, got %service", body)
		}
	})
}
