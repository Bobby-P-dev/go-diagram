package service

import (
	"encoding/json"

	"github.com/Bobby-P-dev/go-diagram.git/internal/modules/workspace"
)

type CommentRepository interface {
	Create(projectID, nodeID, author, content string, posX, posY float64, metadata []byte) (*workspace.DesignComment, error)
	GetByID(id string) (*workspace.DesignComment, error)
	GetByProjectID(projectID string) ([]workspace.DesignComment, error)
	UpdateStatus(id, status string) error
	Delete(id string) error
}

type FoundationRepository interface {
	GetAll() ([]workspace.DesignFoundation, error)
}

type SettingsRepository interface {
	Get() (*workspace.WorkspaceSetting, error)
	Update(defaultMode, theme, preferredModel string, canvasPreferences json.RawMessage) (*workspace.WorkspaceSetting, error)
}

type ExportRepository interface {
	GetByProjectID(projectID string) ([]workspace.ProjectExport, error)
}
