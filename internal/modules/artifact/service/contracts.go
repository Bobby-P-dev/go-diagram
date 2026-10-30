package service

import (
	"context"

	"github.com/Bobby-P-dev/go-diagram.git/internal/modules/artifact"
	"github.com/Bobby-P-dev/go-diagram.git/internal/modules/artifact/dto"
)

type ArtifactService interface {
	CreateArtifact(ctx context.Context, userID, projectID string, req dto.CreateArtifactRequest, idempotencyKey string) (*artifact.ArtifactDetail, error)
	GetArtifact(ctx context.Context, userID, projectID, artifactID string) (*artifact.ArtifactDetail, error)
	ListArtifacts(ctx context.Context, userID, projectID string, query artifact.PageQuery) (*artifact.PageResult[*artifact.Artifact], error)
	GetRevision(ctx context.Context, userID, projectID, artifactID, revisionID string) (*artifact.ArtifactRevision, error)
	ListRevisions(ctx context.Context, userID, projectID, artifactID string, query artifact.PageQuery) (*artifact.PageResult[*artifact.RevisionSummary], error)
	AppendRevision(ctx context.Context, userID, projectID, artifactID string, req dto.AppendRevisionRequest, idempotencyKey string) (*artifact.ArtifactRevision, error)
	RestoreRevision(ctx context.Context, userID, projectID, artifactID string, req dto.RestoreRevisionRequest, idempotencyKey string) (*artifact.ArtifactRevision, error)
}
