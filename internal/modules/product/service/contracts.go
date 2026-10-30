package service

import (
	"context"

	"github.com/Bobby-P-dev/go-diagram.git/internal/modules/product"
	"github.com/Bobby-P-dev/go-diagram.git/internal/modules/product/dto"
)

type ProductService interface {
	CreateProject(ctx context.Context, userID string, req dto.CreateProjectRequest, idempotencyKey string) (*product.Project, error)
	ListProjects(ctx context.Context, userID string, query product.PageQuery) (*product.PageResult[*product.Project], error)
	CreateThread(ctx context.Context, userID, projectID string, req dto.CreateThreadRequest, idempotencyKey string) (*product.ChatThread, error)
	ListThreads(ctx context.Context, userID, projectID string, query product.PageQuery) (*product.PageResult[*product.ChatThread], error)
	PostUserMessage(ctx context.Context, userID, projectID, threadID string, req dto.PostMessageRequest, idempotencyKey string) (*product.ChatMessage, error)
	ListMessages(ctx context.Context, userID, projectID, threadID string, query product.PageQuery) (*product.PageResult[*product.ChatMessage], error)
}
