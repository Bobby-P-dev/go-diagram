package controller

import (
	"context"
	"encoding/json"

	"github.com/Bobby-P-dev/go-diagram.git/internal/modules/diagram/dto"
	"github.com/Bobby-P-dev/go-diagram.git/internal/modules/identity/authcontext"
)

type DiagramService interface {
	CreateNewProjectForUser(userID string, req dto.CreateProjectRequest) (*dto.ProjectResponse, error)
	SendChatMessageScoped(projectID string, req dto.ChatRequest, userID string) (*dto.ProjectResponse, error)
	GetProjectByIDScoped(id, userID, userRole string) (*dto.ProjectResponse, error)
	UpdateProjectGraphScoped(id, userID string, nodes, edges json.RawMessage) error
	ListProjectsForUser(userID string, limit, offset int) (*dto.PaginatedProjectsResponse, error)
	ToggleProjectPinScoped(id, userID string) (*dto.TogglePinResponse, error)
	GetProjectVersionsScoped(projectID, userID string) ([]dto.DiagramVersionDTO, error)
	RollbackToVersionScoped(projectID, versionID, userID string) (*dto.ProjectResponse, error)
	ListTemplates() ([]dto.TemplateDTO, error)
}

type AuthenticatedUser struct {
	ID                 string
	Role               string
	CanGenerateDiagram bool
}

type UserReader func(context.Context) *AuthenticatedUser

func DefaultUserReader(ctx context.Context) *AuthenticatedUser {
	if ctx == nil {
		return nil
	}
	user := authcontext.UserFromContext(ctx)
	if user == nil {
		return nil
	}
	return &AuthenticatedUser{
		ID:                 user.ID,
		Role:               user.Role,
		CanGenerateDiagram: user.CanGenerateDiagram,
	}
}

func readUser(reader UserReader, ctx context.Context) *AuthenticatedUser {
	if reader == nil {
		return DefaultUserReader(ctx)
	}
	return reader(ctx)
}
