package service

import (
	"context"

	"github.com/Bobby-P-dev/go-diagram.git/internal/modules/workspace"
	"github.com/Bobby-P-dev/go-diagram.git/internal/modules/workspace/dto"
)

func (s *WorkspaceService) GetComments(ctx context.Context, actor *workspace.Actor, projectID string) ([]workspace.DesignComment, error) {
	if err := s.authorize(ctx, actor, projectID); err != nil {
		return nil, err
	}
	return s.comments.GetByProjectID(projectID)
}

func (s *WorkspaceService) CreateComment(ctx context.Context, actor *workspace.Actor, projectID string, input dto.CreateCommentRequest) (*workspace.DesignComment, error) {
	if input.Content == "" {
		return nil, ErrContentRequired
	}
	if input.Author == "" {
		input.Author = "Designer"
	}
	if err := s.authorize(ctx, actor, projectID); err != nil {
		return nil, err
	}
	return s.comments.Create(projectID, input.NodeID, input.Author, input.Content, input.PosX, input.PosY, input.Metadata)
}

func (s *WorkspaceService) UpdateCommentStatus(ctx context.Context, actor *workspace.Actor, id string, input dto.UpdateCommentStatusRequest) (string, error) {
	comment, err := s.comments.GetByID(id)
	if err != nil {
		return "", ErrCommentNotFound
	}
	if err := s.authorize(ctx, actor, comment.ProjectID); err != nil {
		return "", err
	}
	if input.Status == "" {
		input.Status = "resolved"
	}
	if err := s.comments.UpdateStatus(id, input.Status); err != nil {
		return "", err
	}
	return input.Status, nil
}

func (s *WorkspaceService) DeleteComment(ctx context.Context, actor *workspace.Actor, id string) error {
	comment, err := s.comments.GetByID(id)
	if err != nil {
		return ErrCommentNotFound
	}
	if err := s.authorize(ctx, actor, comment.ProjectID); err != nil {
		return err
	}
	return s.comments.Delete(id)
}
