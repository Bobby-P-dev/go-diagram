package uidesign

import (
	"context"
	"encoding/json"
)

type ProjectAccessReader interface {
	CheckProjectAccess(ctx context.Context, projectID, userID string) error
}

type ProjectGateway interface {
	ProjectAccessReader
	CreateProject(ctx context.Context, input CreateProjectInput) (*Project, error)
	GetProjectScoped(ctx context.Context, id, userID, userRole string) (*Project, []ChatMessage, error)
	UpdateGraph(ctx context.Context, id string, nodes, edges json.RawMessage) error
	CreateVersion(ctx context.Context, input CreateVersionInput) error
	GetLatestVersionNumber(ctx context.Context, projectID string) (int, error)
	AppendMessage(ctx context.Context, projectID, role, content string, targetNodeIDs json.RawMessage) (*ChatMessage, error)
}
