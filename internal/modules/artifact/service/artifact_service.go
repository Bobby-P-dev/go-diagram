package service

import (
	"context"
	"fmt"
	"strings"

	"github.com/Bobby-P-dev/go-diagram.git/internal/modules/artifact"
	"github.com/Bobby-P-dev/go-diagram.git/internal/modules/artifact/dto"
)

func (service *Service) CreateArtifact(
	ctx context.Context,
	userID, projectID string,
	req dto.CreateArtifactRequest,
	idempotencyKey string,
) (*artifact.ArtifactDetail, error) {
	if err := validateActor(userID); err != nil {
		return nil, err
	}
	projectID = strings.ToLower(strings.TrimSpace(projectID))
	if err := validateID(projectID); err != nil {
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
	title := strings.TrimSpace(req.Title)
	if err := artifact.ValidateText(title, 255, true); err != nil {
		return nil, err
	}
	artType := strings.TrimSpace(strings.ToLower(req.Type))
	format, err := artifact.ResolveFormat(artType)
	if err != nil {
		return nil, err
	}

	slug := strings.TrimSpace(strings.ToLower(req.Slug))
	if slug == "" {
		slug = artifact.Slugify(title)
	}
	if err := artifact.ValidateSlug(slug); err != nil {
		return nil, err
	}

	if err := artifact.ValidateContent(format, req.DocumentContent, req.GraphNodes, req.GraphEdges, req.FrameData); err != nil {
		return nil, err
	}

	input := artifact.CreateArtifactInput{
		Type:             artType,
		Title:            title,
		Slug:             slug,
		DocumentContent:  req.DocumentContent,
		GraphNodes:       req.GraphNodes,
		GraphEdges:       req.GraphEdges,
		FrameData:        req.FrameData,
		ChangeSummary:    strings.TrimSpace(req.ChangeSummary),
		SchemaVersion:    schemaVersion,
		TriggerMessageID: triggerMessageID,
		Sources:          sources,
	}
	write := artifact.WriteRequest{
		Key:         strings.TrimSpace(idempotencyKey),
		RequestHash: computeRequestHash(input),
		Resource:    fmt.Sprintf("/api/v2/projects/%s/artifacts", projectID),
	}

	return service.repo.CreateArtifact(ctx, userID, projectID, input, write)
}

func (service *Service) GetArtifact(
	ctx context.Context,
	userID, projectID, artifactID string,
) (*artifact.ArtifactDetail, error) {
	if err := validateActor(userID); err != nil {
		return nil, err
	}
	projectID, artifactID = strings.ToLower(strings.TrimSpace(projectID)), strings.ToLower(strings.TrimSpace(artifactID))
	if err := validateID(projectID); err != nil {
		return nil, err
	}
	if err := validateID(artifactID); err != nil {
		return nil, err
	}
	return service.repo.GetArtifact(ctx, userID, projectID, artifactID)
}

func (service *Service) ListArtifacts(
	ctx context.Context,
	userID, projectID string,
	query artifact.PageQuery,
) (*artifact.PageResult[*artifact.Artifact], error) {
	if err := validateActor(userID); err != nil {
		return nil, err
	}
	projectID = strings.ToLower(strings.TrimSpace(projectID))
	if err := validateID(projectID); err != nil {
		return nil, err
	}
	query, err := pageQuery(query)
	if err != nil {
		return nil, err
	}
	if query.TypeFilter != "" {
		if _, err := artifact.ResolveFormat(query.TypeFilter); err != nil {
			return nil, err
		}
	}
	return service.repo.ListArtifacts(ctx, userID, projectID, query)
}
