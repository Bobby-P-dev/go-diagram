package service

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/Bobby-P-dev/go-diagram.git/internal/modules/product"
	"github.com/Bobby-P-dev/go-diagram.git/internal/modules/product/dto"
)

type mockRepo struct {
	createProjectFn func(ctx context.Context, userID string, input product.ProjectInput, write product.WriteRequest) (*product.Project, error)
	listProjectsFn  func(ctx context.Context, userID string, query product.PageQuery) (*product.PageResult[*product.Project], error)
	createThreadFn  func(ctx context.Context, userID, projectID string, input product.ThreadInput, write product.WriteRequest) (*product.ChatThread, error)
	listThreadsFn   func(ctx context.Context, userID, projectID string, query product.PageQuery) (*product.PageResult[*product.ChatThread], error)
	postMessageFn   func(ctx context.Context, userID, projectID, threadID string, input product.MessageInput, write product.WriteRequest) (*product.ChatMessage, error)
	listMessagesFn  func(ctx context.Context, userID, projectID, threadID string, query product.PageQuery) (*product.PageResult[*product.ChatMessage], error)
}

func (mock *mockRepo) CreateProject(ctx context.Context, userID string, input product.ProjectInput, write product.WriteRequest) (*product.Project, error) {
	if mock.createProjectFn != nil {
		return mock.createProjectFn(ctx, userID, input, write)
	}
	return nil, errors.New("not implemented")
}

func (mock *mockRepo) ListProjects(ctx context.Context, userID string, query product.PageQuery) (*product.PageResult[*product.Project], error) {
	if mock.listProjectsFn != nil {
		return mock.listProjectsFn(ctx, userID, query)
	}
	return nil, errors.New("not implemented")
}

func (mock *mockRepo) CreateThread(ctx context.Context, userID, projectID string, input product.ThreadInput, write product.WriteRequest) (*product.ChatThread, error) {
	if mock.createThreadFn != nil {
		return mock.createThreadFn(ctx, userID, projectID, input, write)
	}
	return nil, errors.New("not implemented")
}

func (mock *mockRepo) ListThreads(ctx context.Context, userID, projectID string, query product.PageQuery) (*product.PageResult[*product.ChatThread], error) {
	if mock.listThreadsFn != nil {
		return mock.listThreadsFn(ctx, userID, projectID, query)
	}
	return nil, errors.New("not implemented")
}

func (mock *mockRepo) PostUserMessage(ctx context.Context, userID, projectID, threadID string, input product.MessageInput, write product.WriteRequest) (*product.ChatMessage, error) {
	if mock.postMessageFn != nil {
		return mock.postMessageFn(ctx, userID, projectID, threadID, input, write)
	}
	return nil, errors.New("not implemented")
}

func (mock *mockRepo) ListMessages(ctx context.Context, userID, projectID, threadID string, query product.PageQuery) (*product.PageResult[*product.ChatMessage], error) {
	if mock.listMessagesFn != nil {
		return mock.listMessagesFn(ctx, userID, projectID, threadID, query)
	}
	return nil, errors.New("not implemented")
}

const (
	testUserID     = "11111111-1111-1111-1111-111111111111"
	testProjectID  = "22222222-2222-2222-2222-222222222222"
	testThreadID   = "33333333-3333-3333-3333-333333333333"
	testArtifactID = "44444444-4444-4444-4444-444444444444"
)

func TestService_CreateProject(test *testing.T) {
	repo := &mockRepo{}
	svc := NewService(repo)

	test.Run("unauthenticated", func(test *testing.T) {
		_, err := svc.CreateProject(context.Background(), "", dto.CreateProjectRequest{Title: "Proj"}, "key-1")
		if !errors.Is(err, product.ErrUnauthenticated) {
			test.Fatalf("expected ErrUnauthenticated, got %v", err)
		}
	})

	test.Run("empty title", func(test *testing.T) {
		_, err := svc.CreateProject(context.Background(), testUserID, dto.CreateProjectRequest{Title: "   "}, "key-1")
		if !errors.Is(err, product.ErrInvalidInput) {
			test.Fatalf("expected ErrInvalidInput, got %v", err)
		}
	})

	test.Run("title too long", func(test *testing.T) {
		_, err := svc.CreateProject(context.Background(), testUserID, dto.CreateProjectRequest{Title: strings.Repeat("a", 256)}, "key-1")
		if !errors.Is(err, product.ErrInvalidInput) {
			test.Fatalf("expected ErrInvalidInput, got %v", err)
		}
	})

	test.Run("invalid idempotency key", func(test *testing.T) {
		_, err := svc.CreateProject(context.Background(), testUserID, dto.CreateProjectRequest{Title: "Proj"}, "")
		if !errors.Is(err, product.ErrInvalidInput) {
			test.Fatalf("expected ErrInvalidInput, got %v", err)
		}
	})

	test.Run("success", func(test *testing.T) {
		called := false
		repo.createProjectFn = func(ctx context.Context, userID string, input product.ProjectInput, write product.WriteRequest) (*product.Project, error) {
			called = true
			if userID != testUserID {
				test.Errorf("expected userID %s, got %s", testUserID, userID)
			}
			if input.Title != "My Project" {
				test.Errorf("expected title My Project, got %s", input.Title)
			}
			if write.Key != "key-1" || len(write.RequestHash) != 64 {
				test.Errorf("expected valid write request: %+v", write)
			}
			return &product.Project{
				ID:        testProjectID,
				UserID:    userID,
				Title:     input.Title,
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			}, nil
		}

		res, err := svc.CreateProject(context.Background(), testUserID, dto.CreateProjectRequest{Title: "  My Project  "}, "key-1")
		if err != nil {
			test.Fatalf("unexpected error: %v", err)
		}
		if !called || res.ID != testProjectID {
			test.Fatalf("unexpected result: %+v", res)
		}
	})
}

func TestService_CreateThread(test *testing.T) {
	repo := &mockRepo{}
	svc := NewService(repo)

	test.Run("invalid project ID", func(test *testing.T) {
		_, err := svc.CreateThread(context.Background(), testUserID, "not-a-uuid", dto.CreateThreadRequest{Title: "Thread"}, "key-1")
		if !errors.Is(err, product.ErrInvalidInput) {
			test.Fatalf("expected ErrInvalidInput, got %v", err)
		}
	})

	test.Run("empty title", func(test *testing.T) {
		_, err := svc.CreateThread(context.Background(), testUserID, testProjectID, dto.CreateThreadRequest{Title: ""}, "key-1")
		if !errors.Is(err, product.ErrInvalidInput) {
			test.Fatalf("expected ErrInvalidInput, got %v", err)
		}
	})

	test.Run("invalid active artifact ID", func(test *testing.T) {
		badArt := "invalid-uuid"
		_, err := svc.CreateThread(context.Background(), testUserID, testProjectID, dto.CreateThreadRequest{
			Title:            "Thread",
			ActiveArtifactID: &badArt,
		}, "key-1")
		if !errors.Is(err, product.ErrInvalidInput) {
			test.Fatalf("expected ErrInvalidInput, got %v", err)
		}
	})

	test.Run("success with default mode hint", func(test *testing.T) {
		repo.createThreadFn = func(ctx context.Context, userID, projectID string, input product.ThreadInput, write product.WriteRequest) (*product.ChatThread, error) {
			if input.ModeHint != "general" {
				test.Errorf("expected general, got %s", input.ModeHint)
			}
			return &product.ChatThread{
				ID:        testThreadID,
				ProjectID: projectID,
				Title:     input.Title,
				ModeHint:  input.ModeHint,
			}, nil
		}

		res, err := svc.CreateThread(context.Background(), testUserID, testProjectID, dto.CreateThreadRequest{Title: "Discussion"}, "key-1")
		if err != nil {
			test.Fatalf("unexpected error: %v", err)
		}
		if res.ID != testThreadID {
			test.Fatalf("unexpected thread ID: %s", res.ID)
		}
	})
}

func TestService_PostUserMessage(test *testing.T) {
	repo := &mockRepo{}
	svc := NewService(repo)

	test.Run("empty content", func(test *testing.T) {
		_, err := svc.PostUserMessage(context.Background(), testUserID, testProjectID, testThreadID, dto.PostMessageRequest{
			Content:         "",
			ClientMessageID: "msg-1",
		}, "key-1")
		if !errors.Is(err, product.ErrInvalidInput) {
			test.Fatalf("expected ErrInvalidInput, got %v", err)
		}
	})

	test.Run("empty client message ID", func(test *testing.T) {
		_, err := svc.PostUserMessage(context.Background(), testUserID, testProjectID, testThreadID, dto.PostMessageRequest{
			Content:         "Hello",
			ClientMessageID: "",
		}, "key-1")
		if !errors.Is(err, product.ErrInvalidInput) {
			test.Fatalf("expected ErrInvalidInput, got %v", err)
		}
	})

	test.Run("success", func(test *testing.T) {
		repo.postMessageFn = func(ctx context.Context, userID, projectID, threadID string, input product.MessageInput, write product.WriteRequest) (*product.ChatMessage, error) {
			return &product.ChatMessage{
				ID:             "msg-uuid",
				ProjectID:      projectID,
				ThreadID:       threadID,
				Role:           "user",
				Content:        input.Content,
				SequenceNumber: 1,
			}, nil
		}

		res, err := svc.PostUserMessage(context.Background(), testUserID, testProjectID, testThreadID, dto.PostMessageRequest{
			Content:         "Let's brainstorm the ERD",
			ClientMessageID: "client-1",
		}, "key-1")
		if err != nil {
			test.Fatalf("unexpected error: %v", err)
		}
		if res.Role != "user" || res.Content != "Let's brainstorm the ERD" {
			test.Fatalf("unexpected message: %+v", res)
		}
	})
}
