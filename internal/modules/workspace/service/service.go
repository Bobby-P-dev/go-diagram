package service

import (
	"context"
	"errors"

	"github.com/Bobby-P-dev/go-diagram.git/internal/modules/workspace"
)

var (
	ErrUnauthenticated   = errors.New("authentication required")
	ErrAccessUnavailable = errors.New("workspace authorization unavailable")
	ErrProjectNotFound   = errors.New("project not found")
	ErrCommentNotFound   = errors.New("comment not found")
	ErrContentRequired   = errors.New("content is required")
)

type WorkspaceService struct {
	projects    workspace.ProjectAccessReader
	comments    CommentRepository
	foundations FoundationRepository
	settings    SettingsRepository
	exports     ExportRepository
}

func NewWorkspaceService(projects workspace.ProjectAccessReader, comments CommentRepository, foundations FoundationRepository, settings SettingsRepository, exports ExportRepository) *WorkspaceService {
	return &WorkspaceService{projects: projects, comments: comments, foundations: foundations, settings: settings, exports: exports}
}

func (s *WorkspaceService) authorize(ctx context.Context, actor *workspace.Actor, projectID string) error {
	if actor == nil {
		return ErrUnauthenticated
	}
	if s.projects == nil {
		return ErrAccessUnavailable
	}
	if err := s.projects.CheckProjectAccess(ctx, projectID, actor.ID, actor.Role); err != nil {
		return ErrProjectNotFound
	}
	return nil
}
