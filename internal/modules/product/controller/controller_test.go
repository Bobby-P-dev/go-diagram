package controller

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Bobby-P-dev/go-diagram.git/internal/modules/identity/authcontext"
	identityservice "github.com/Bobby-P-dev/go-diagram.git/internal/modules/identity/service"
	"github.com/Bobby-P-dev/go-diagram.git/internal/modules/product"
	"github.com/Bobby-P-dev/go-diagram.git/internal/modules/product/dto"
)

type mockProductService struct {
	createProjectFn func(ctx context.Context, userID string, req dto.CreateProjectRequest, key string) (*product.Project, error)
	listProjectsFn  func(ctx context.Context, userID string, query product.PageQuery) (*product.PageResult[*product.Project], error)
	createThreadFn  func(ctx context.Context, userID, projectID string, req dto.CreateThreadRequest, key string) (*product.ChatThread, error)
	listThreadsFn   func(ctx context.Context, userID, projectID string, query product.PageQuery) (*product.PageResult[*product.ChatThread], error)
	postMessageFn   func(ctx context.Context, userID, projectID, threadID string, req dto.PostMessageRequest, key string) (*product.ChatMessage, error)
	listMessagesFn  func(ctx context.Context, userID, projectID, threadID string, query product.PageQuery) (*product.PageResult[*product.ChatMessage], error)
}

func (mock *mockProductService) CreateProject(ctx context.Context, userID string, req dto.CreateProjectRequest, key string) (*product.Project, error) {
	if mock.createProjectFn != nil {
		return mock.createProjectFn(ctx, userID, req, key)
	}
	return nil, errors.New("not implemented")
}

func (mock *mockProductService) ListProjects(ctx context.Context, userID string, query product.PageQuery) (*product.PageResult[*product.Project], error) {
	if mock.listProjectsFn != nil {
		return mock.listProjectsFn(ctx, userID, query)
	}
	return nil, errors.New("not implemented")
}

func (mock *mockProductService) CreateThread(ctx context.Context, userID, projectID string, req dto.CreateThreadRequest, key string) (*product.ChatThread, error) {
	if mock.createThreadFn != nil {
		return mock.createThreadFn(ctx, userID, projectID, req, key)
	}
	return nil, errors.New("not implemented")
}

func (mock *mockProductService) ListThreads(ctx context.Context, userID, projectID string, query product.PageQuery) (*product.PageResult[*product.ChatThread], error) {
	if mock.listThreadsFn != nil {
		return mock.listThreadsFn(ctx, userID, projectID, query)
	}
	return nil, errors.New("not implemented")
}

func (mock *mockProductService) PostUserMessage(ctx context.Context, userID, projectID, threadID string, req dto.PostMessageRequest, key string) (*product.ChatMessage, error) {
	if mock.postMessageFn != nil {
		return mock.postMessageFn(ctx, userID, projectID, threadID, req, key)
	}
	return nil, errors.New("not implemented")
}

func (mock *mockProductService) ListMessages(ctx context.Context, userID, projectID, threadID string, query product.PageQuery) (*product.PageResult[*product.ChatMessage], error) {
	if mock.listMessagesFn != nil {
		return mock.listMessagesFn(ctx, userID, projectID, threadID, query)
	}
	return nil, errors.New("not implemented")
}

const (
	testUserID    = "11111111-1111-1111-1111-111111111111"
	testProjectID = "22222222-2222-2222-2222-222222222222"
	testThreadID  = "33333333-3333-3333-3333-333333333333"
)

func withAuth(req *http.Request) *http.Request {
	return req.WithContext(authcontext.WithUser(req.Context(), &identityservice.CredentialRecord{
		ID:       testUserID,
		Role:     "user",
		IsActive: true,
	}))
}

func TestController_CreateProject(test *testing.T) {
	svc := &mockProductService{}
	ctrl := NewProductController(svc)

	test.Run("unauthenticated", func(test *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/v2/projects", bytes.NewBufferString(`{"title":"Test"}`))
		response := httptest.NewRecorder()
		ctrl.CreateProject(response, req)
		if response.Code != http.StatusUnauthorized {
			test.Fatalf("expected 401, got %d", response.Code)
		}
	})

	test.Run("missing idempotency key", func(test *testing.T) {
		req := withAuth(httptest.NewRequest(http.MethodPost, "/api/v2/projects", bytes.NewBufferString(`{"title":"Test"}`)))
		response := httptest.NewRecorder()
		ctrl.CreateProject(response, req)
		if response.Code != http.StatusBadRequest {
			test.Fatalf("expected 400, got %d", response.Code)
		}
	})

	test.Run("success", func(test *testing.T) {
		svc.createProjectFn = func(ctx context.Context, userID string, req dto.CreateProjectRequest, key string) (*product.Project, error) {
			return &product.Project{
				ID:        testProjectID,
				UserID:    userID,
				Title:     req.Title,
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			}, nil
		}

		req := withAuth(httptest.NewRequest(http.MethodPost, "/api/v2/projects", bytes.NewBufferString(`{"title":"My Project"}`)))
		req.Header.Set("Idempotency-Key", "key-123")
		response := httptest.NewRecorder()
		ctrl.CreateProject(response, req)

		if response.Code != http.StatusCreated {
			test.Fatalf("expected 201, got %d", response.Code)
		}
		var body map[string]interface{}
		if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
			test.Fatal(err)
		}
		if body["status"] != "success" {
			test.Fatalf("expected success status, got %v", body["status"])
		}
	})

	test.Run("idempotency conflict", func(test *testing.T) {
		svc.createProjectFn = func(ctx context.Context, userID string, req dto.CreateProjectRequest, key string) (*product.Project, error) {
			return nil, product.ErrIdempotencyConflict
		}

		req := withAuth(httptest.NewRequest(http.MethodPost, "/api/v2/projects", bytes.NewBufferString(`{"title":"My Project"}`)))
		req.Header.Set("Idempotency-Key", "key-123")
		response := httptest.NewRecorder()
		ctrl.CreateProject(response, req)

		if response.Code != http.StatusConflict {
			test.Fatalf("expected 409, got %d", response.Code)
		}
	})
}

func TestController_ListProjects(test *testing.T) {
	svc := &mockProductService{}
	ctrl := NewProductController(svc)

	test.Run("success", func(test *testing.T) {
		cursor := "next-cursor-val"
		svc.listProjectsFn = func(ctx context.Context, userID string, query product.PageQuery) (*product.PageResult[*product.Project], error) {
			return &product.PageResult[*product.Project]{
				Items: []*product.Project{
					{ID: testProjectID, Title: "P1"},
				},
				NextCursor: &cursor,
			}, nil
		}

		req := withAuth(httptest.NewRequest(http.MethodGet, "/api/v2/projects?limit=10", nil))
		response := httptest.NewRecorder()
		ctrl.ListProjects(response, req)

		if response.Code != http.StatusOK {
			test.Fatalf("expected 200, got %d", response.Code)
		}
	})
}

func TestController_CreateThread(test *testing.T) {
	svc := &mockProductService{}
	ctrl := NewProductController(svc)

	test.Run("success", func(test *testing.T) {
		svc.createThreadFn = func(ctx context.Context, userID, projectID string, req dto.CreateThreadRequest, key string) (*product.ChatThread, error) {
			return &product.ChatThread{
				ID:        testThreadID,
				ProjectID: projectID,
				Title:     req.Title,
			}, nil
		}

		req := withAuth(httptest.NewRequest(http.MethodPost, "/api/v2/projects/"+testProjectID+"/threads", bytes.NewBufferString(`{"title":"Design Chat"}`)))
		req.SetPathValue("projectId", testProjectID)
		req.Header.Set("Idempotency-Key", "thread-key-1")
		response := httptest.NewRecorder()
		ctrl.CreateThread(response, req)

		if response.Code != http.StatusCreated {
			test.Fatalf("expected 201, got %d", response.Code)
		}
	})

	test.Run("project not found", func(test *testing.T) {
		svc.createThreadFn = func(ctx context.Context, userID, projectID string, req dto.CreateThreadRequest, key string) (*product.ChatThread, error) {
			return nil, product.ErrNotFound
		}

		req := withAuth(httptest.NewRequest(http.MethodPost, "/api/v2/projects/"+testProjectID+"/threads", bytes.NewBufferString(`{"title":"Design Chat"}`)))
		req.SetPathValue("projectId", testProjectID)
		req.Header.Set("Idempotency-Key", "thread-key-1")
		response := httptest.NewRecorder()
		ctrl.CreateThread(response, req)

		if response.Code != http.StatusNotFound {
			test.Fatalf("expected 404, got %d", response.Code)
		}
	})
}

func TestController_PostMessage(test *testing.T) {
	svc := &mockProductService{}
	ctrl := NewProductController(svc)

	test.Run("success", func(test *testing.T) {
		svc.postMessageFn = func(ctx context.Context, userID, projectID, threadID string, req dto.PostMessageRequest, key string) (*product.ChatMessage, error) {
			return &product.ChatMessage{
				ID:              "msg-1",
				ProjectID:       projectID,
				ThreadID:        threadID,
				Role:            "user",
				Content:         req.Content,
				ClientMessageID: &req.ClientMessageID,
				SequenceNumber:  1,
			}, nil
		}

		req := withAuth(httptest.NewRequest(http.MethodPost, "/api/v2/projects/"+testProjectID+"/threads/"+testThreadID+"/messages", bytes.NewBufferString(`{"content":"Hello world","client_message_id":"c-1"}`)))
		req.SetPathValue("projectId", testProjectID)
		req.SetPathValue("threadId", testThreadID)
		req.Header.Set("Idempotency-Key", "msg-key-1")
		response := httptest.NewRecorder()
		ctrl.PostMessage(response, req)

		if response.Code != http.StatusCreated {
			test.Fatalf("expected 201, got %d", response.Code)
		}
	})
}
