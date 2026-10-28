package service

import (
	"context"

	"github.com/Bobby-P-dev/go-diagram.git/internal/modules/sharing"
)

type ShareRepository interface {
	FindByProjectID(ctx context.Context, projectID string) (*sharing.Share, error)
	FindActiveByToken(ctx context.Context, token string) (*sharing.Share, error)
	Create(ctx context.Context, input sharing.NewShare) (*sharing.Share, error)
	Reactivate(ctx context.Context, id, title string) (*sharing.Share, error)
	Revoke(ctx context.Context, projectID string) error
	IncrementViewCount(ctx context.Context, shareID string) error
}

type ProjectGateway interface {
	LoadSharedProject(ctx context.Context, projectID string) (*sharing.Project, []sharing.Message, error)
	CloneSharedProject(ctx context.Context, input sharing.CloneInput) (*sharing.Project, error)
}

type ShareService interface {
	CreateOrGetShare(ctx context.Context, projectID, title string) (*sharing.Share, error)
	GetShareByProjectID(ctx context.Context, projectID string) (*sharing.Share, error)
	GetShareByToken(ctx context.Context, token string) (*sharing.Share, *sharing.Project, []sharing.Message, error)
	RevokeShare(ctx context.Context, projectID string) error
	ForkSharedProjectWithUser(ctx context.Context, token, newTitle, userID string) (*sharing.Project, error)
}
