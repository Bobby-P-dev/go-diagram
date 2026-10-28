package projectgateway

import (
	"context"

	"github.com/Bobby-P-dev/go-diagram.git/internal/modules/diagram"
	"github.com/Bobby-P-dev/go-diagram.git/internal/modules/workspace"
)

type WorkspaceProjectAccessModel interface {
	GetByIDScoped(id, userID, userRole string) (*diagram.Project, []diagram.ChatMessage, error)
}

type workspaceProjectAccessReader struct {
	model WorkspaceProjectAccessModel
}

func NewWorkspaceProjectAccessReader(model WorkspaceProjectAccessModel) workspace.ProjectAccessReader {
	if model == nil {
		return nil
	}
	return &workspaceProjectAccessReader{model: model}
}

func (reader *workspaceProjectAccessReader) CheckProjectAccess(ctx context.Context, projectID, userID, role string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if model, ok := reader.model.(interface {
		GetByIDScopedContext(context.Context, string, string, string) (*diagram.Project, []diagram.ChatMessage, error)
	}); ok {
		_, _, err := model.GetByIDScopedContext(ctx, projectID, userID, role)
		return err
	}
	_, _, err := reader.model.GetByIDScoped(projectID, userID, role)
	return err
}
