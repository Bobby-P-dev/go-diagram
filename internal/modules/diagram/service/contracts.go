package service

import (
	"encoding/json"

	"github.com/Bobby-P-dev/go-diagram.git/internal/modules/diagram"
	"github.com/Bobby-P-dev/go-diagram.git/internal/modules/diagram/dto"
)

type ProjectRepository interface {
	CreateWithUser(userID, title, diagramType, projectMode string, metadata, nodes, edges json.RawMessage) (*diagram.Project, error)
	GetByIDScoped(id, userID, userRole string) (*diagram.Project, []diagram.ChatMessage, error)
	UpdateGraph(id string, nodes, edges json.RawMessage) error
	GetPaginatedScoped(userID string, limit, offset int) ([]diagram.ProjectSummary, int, error)
	TogglePin(id string) (bool, error)
}

type MessageRepository interface {
	AppendMessage(projectID, role, content string, targetNodeIDs json.RawMessage) (*diagram.ChatMessage, error)
}

type VersionRepository interface {
	CreateVersion(projectID string, versionNumber int, summary string, nodes, edges json.RawMessage, triggerMessageID *string) (*diagram.DiagramVersion, error)
	GetLatestVersionNumber(projectID string) (int, error)
	GetVersionByID(id string) (*diagram.DiagramVersion, error)
	GetVersionsByProjectID(projectID string) ([]diagram.DiagramVersion, error)
}

type TemplateRepository interface {
	GetTemplateByID(id string) (*diagram.DiagramTemplate, error)
	GetAllTemplates() ([]diagram.DiagramTemplate, error)
}

type AIClient interface {
	GenerateDiagram(history []diagram.ChatMessage, currentGraph, prompt, diagramType string, targetedNodeIDs []string) (*dto.GraphPayload, string, error)
	Provider() string
	Model() string
}
