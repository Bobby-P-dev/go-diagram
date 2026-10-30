package service

import (
	"context"
	"fmt"
	"strings"

	"github.com/Bobby-P-dev/go-diagram.git/internal/modules/artifact"
	"github.com/Bobby-P-dev/go-diagram.git/internal/modules/artifact/dto"
)

func (service *Service) GetRevision(
	ctx context.Context,
	userID, projectID, artifactID, revisionID string,
) (*artifact.ArtifactRevision, error) {
	if err := validateActor(userID); err != nil {
		return nil, err
	}
	projectID = strings.ToLower(strings.TrimSpace(projectID))
	artifactID = strings.ToLower(strings.TrimSpace(artifactID))
	revisionID = strings.ToLower(strings.TrimSpace(revisionID))
	if err := validateID(projectID); err != nil {
		return nil, err
	}
	if err := validateID(artifactID); err != nil {
		return nil, err
	}
	if err := validateID(revisionID); err != nil {
		return nil, err
	}
	return service.repo.GetRevision(ctx, userID, projectID, artifactID, revisionID)
}

func (service *Service) ListRevisions(
	ctx context.Context,
	userID, projectID, artifactID string,
	query artifact.PageQuery,
) (*artifact.PageResult[*artifact.RevisionSummary], error) {
	if err := validateActor(userID); err != nil {
		return nil, err
	}
	projectID = strings.ToLower(strings.TrimSpace(projectID))
	artifactID = strings.ToLower(strings.TrimSpace(artifactID))
	if err := validateID(projectID); err != nil {
		return nil, err
	}
	if err := validateID(artifactID); err != nil {
		return nil, err
	}
	query, err := pageQuery(query)
	if err != nil {
		return nil, err
	}
	if query.TypeFilter != "" {
		return nil, artifact.ErrInvalidInput
	}
	return service.repo.ListRevisions(ctx, userID, projectID, artifactID, query)
}

func (service *Service) AppendRevision(
	ctx context.Context,
	userID, projectID, artifactID string,
	req dto.AppendRevisionRequest,
	idempotencyKey string,
) (*artifact.ArtifactRevision, error) {
	if err := validateActor(userID); err != nil {
		return nil, err
	}
	projectID = strings.ToLower(strings.TrimSpace(projectID))
	artifactID = strings.ToLower(strings.TrimSpace(artifactID))
	expectedRevID := strings.ToLower(strings.TrimSpace(req.ExpectedRevisionID))
	if err := validateID(projectID); err != nil {
		return nil, err
	}
	if err := validateID(artifactID); err != nil {
		return nil, err
	}
	if err := validateID(expectedRevID); err != nil {
		return nil, err
	}
	if err := artifact.ValidateKey(idempotencyKey); err != nil {
		return nil, err
	}
	if err := artifact.ValidateText(req.ChangeSummary, 1000, false); err != nil {
		return nil, err
	}
	schemaVersion, triggerMessageID, sources, err := normalizeRevisionContext(req.SchemaVersion, req.TriggerMessageID, req.Sources)
	if err != nil {
		return nil, err
	}
	for _, source := range sources {
		if source.ArtifactID == artifactID {
			return nil, artifact.ErrInvalidInput
		}
	}
	format := "graph"
	if req.DocumentContent != nil {
		format = "markdown"
	} else if len(req.FrameData) > 0 {
		format = "frame"
	}
	if err := artifact.ValidateContent(format, req.DocumentContent, req.GraphNodes, req.GraphEdges, req.FrameData); err != nil {
		return nil, err
	}

	input := artifact.AppendRevisionInput{
		ExpectedRevisionID: expectedRevID,
		ChangeSummary:      strings.TrimSpace(req.ChangeSummary),
		DocumentContent:    req.DocumentContent,
		GraphNodes:         req.GraphNodes,
		GraphEdges:         req.GraphEdges,
		FrameData:          req.FrameData,
		SchemaVersion:      schemaVersion,
		TriggerMessageID:   triggerMessageID,
		Sources:            sources,
	}
	write := artifact.WriteRequest{
		Key:         strings.TrimSpace(idempotencyKey),
		RequestHash: computeRequestHash(input),
		Resource:    fmt.Sprintf("/api/v2/projects/%s/artifacts/%s/revisions", projectID, artifactID),
	}

	return service.repo.AppendRevision(ctx, userID, projectID, artifactID, input, write)
}

func (service *Service) RestoreRevision(
	ctx context.Context,
	userID, projectID, artifactID string,
	req dto.RestoreRevisionRequest,
	idempotencyKey string,
) (*artifact.ArtifactRevision, error) {
	if err := validateActor(userID); err != nil {
		return nil, err
	}
	projectID = strings.ToLower(strings.TrimSpace(projectID))
	artifactID = strings.ToLower(strings.TrimSpace(artifactID))
	targetRevID := strings.ToLower(strings.TrimSpace(req.TargetRevisionID))
	expectedRevID := strings.ToLower(strings.TrimSpace(req.ExpectedRevisionID))
	if err := validateID(projectID); err != nil {
		return nil, err
	}
	if err := validateID(artifactID); err != nil {
		return nil, err
	}
	if err := validateID(targetRevID); err != nil {
		return nil, err
	}
	if err := validateID(expectedRevID); err != nil {
		return nil, err
	}
	if err := artifact.ValidateKey(idempotencyKey); err != nil {
		return nil, err
	}
	if err := artifact.ValidateText(req.ChangeSummary, 1000, false); err != nil {
		return nil, err
	}

	input := artifact.RestoreRevisionInput{
		TargetRevisionID:   targetRevID,
		ExpectedRevisionID: expectedRevID,
		ChangeSummary:      strings.TrimSpace(req.ChangeSummary),
	}
	write := artifact.WriteRequest{
		Key:         strings.TrimSpace(idempotencyKey),
		RequestHash: computeRequestHash(input),
		Resource:    fmt.Sprintf("/api/v2/projects/%s/artifacts/%s/restores", projectID, artifactID),
	}

	return service.repo.RestoreRevision(ctx, userID, projectID, artifactID, input, write)
}
