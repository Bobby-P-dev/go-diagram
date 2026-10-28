package controller

import (
	"context"

	"github.com/Bobby-P-dev/go-diagram.git/internal/modules/workspace"
	"github.com/Bobby-P-dev/go-diagram.git/internal/modules/workspace/dto"
)

type WorkspaceService interface {
	GetComments(context.Context, *workspace.Actor, string) ([]workspace.DesignComment, error)
	CreateComment(context.Context, *workspace.Actor, string, dto.CreateCommentRequest) (*workspace.DesignComment, error)
	UpdateCommentStatus(context.Context, *workspace.Actor, string, dto.UpdateCommentStatusRequest) (string, error)
	DeleteComment(context.Context, *workspace.Actor, string) error
	GetFoundations() ([]workspace.DesignFoundation, error)
	GetSettings() (*workspace.WorkspaceSetting, error)
	UpdateSettings(dto.UpdateSettingsRequest) (*workspace.WorkspaceSetting, error)
	GetExports(context.Context, *workspace.Actor, string) ([]workspace.ProjectExport, error)
}
