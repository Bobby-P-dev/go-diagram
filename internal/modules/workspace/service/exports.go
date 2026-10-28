package service

import (
	"context"

	"github.com/Bobby-P-dev/go-diagram.git/internal/modules/workspace"
)

func (s *WorkspaceService) GetExports(ctx context.Context, actor *workspace.Actor, projectID string) ([]workspace.ProjectExport, error) {
	if err := s.authorize(ctx, actor, projectID); err != nil {
		return nil, err
	}
	return s.exports.GetByProjectID(projectID)
}
