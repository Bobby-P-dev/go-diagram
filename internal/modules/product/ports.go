package product

import (
	"context"
	"errors"
)

var (
	ErrNotFound            = errors.New("resource not found")
	ErrUnauthenticated     = errors.New("authentication required")
	ErrInvalidInput        = errors.New("invalid request")
	ErrIdempotencyConflict = errors.New("idempotency key reused with different input")
	ErrIdempotencyInFlight = errors.New("request already in progress")
	ErrMessageConflict     = errors.New("client message ID reused with different content")
)

type PageQuery struct {
	Limit  int
	Cursor string
}

type PageResult[T any] struct {
	Items      []T     `json:"items"`
	NextCursor *string `json:"next_cursor"`
}

type WriteRequest struct {
	Key         string
	RequestHash string
	Resource    string
}

type ProjectInput struct {
	Title       string `json:"title"`
	Description string `json:"description"`
}

type ThreadInput struct {
	Title            string  `json:"title"`
	ModeHint         string  `json:"mode_hint"`
	ActiveArtifactID *string `json:"active_artifact_id"`
}

type MessageInput struct {
	Content         string `json:"content"`
	ClientMessageID string `json:"client_message_id"`
}

type Repository interface {
	CreateProject(ctx context.Context, userID string, input ProjectInput, write WriteRequest) (*Project, error)
	ListProjects(ctx context.Context, userID string, query PageQuery) (*PageResult[*Project], error)

	CreateThread(ctx context.Context, userID, projectID string, input ThreadInput, write WriteRequest) (*ChatThread, error)
	ListThreads(ctx context.Context, userID, projectID string, query PageQuery) (*PageResult[*ChatThread], error)

	PostUserMessage(ctx context.Context, userID, projectID, threadID string, input MessageInput, write WriteRequest) (*ChatMessage, error)
	ListMessages(ctx context.Context, userID, projectID, threadID string, query PageQuery) (*PageResult[*ChatMessage], error)
}
