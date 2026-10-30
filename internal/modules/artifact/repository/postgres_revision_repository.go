package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/Bobby-P-dev/go-diagram.git/internal/modules/artifact"
)

func (repository *PostgresRepository) GetRevision(
	ctx context.Context,
	userID, projectID, artifactID, revisionID string,
) (*artifact.ArtifactRevision, error) {
	var rev artifact.ArtifactRevision
	err := repository.db.QueryRowContext(ctx, `
		SELECT r.id, r.project_id, r.artifact_id, r.version_number, r.change_summary, r.content_format, r.schema_version, r.document_content, r.graph_nodes, r.graph_edges, r.frame_data, r.trigger_message_id, r.author_id, r.author_type, COALESCE(r.metadata, '{}'::jsonb), r.created_at
		FROM artifact_revisions r
		JOIN artifacts a ON a.id = r.artifact_id AND a.project_id = r.project_id
		JOIN projects p ON p.id = a.project_id
		WHERE r.id = $1 AND r.artifact_id = $2 AND r.project_id = $3 AND p.user_id = $4
	`, revisionID, artifactID, projectID, userID).Scan(
		&rev.ID, &rev.ProjectID, &rev.ArtifactID, &rev.VersionNumber, &rev.ChangeSummary, &rev.ContentFormat,
		&rev.SchemaVersion, &rev.DocumentContent, nullableJSON{&rev.GraphNodes}, nullableJSON{&rev.GraphEdges}, nullableJSON{&rev.FrameData},
		&rev.TriggerMessageID, &rev.AuthorID, &rev.AuthorType, &rev.Metadata, &rev.CreatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, artifact.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("query revision: %w", err)
	}

	sources, err := loadContextLinks(ctx, repository.db, projectID, artifactID, rev.VersionNumber)
	if err != nil {
		return nil, err
	}
	rev.Sources = sources

	return &rev, nil
}

func (repository *PostgresRepository) ListRevisions(
	ctx context.Context,
	userID, projectID, artifactID string,
	query artifact.PageQuery,
) (*artifact.PageResult[*artifact.RevisionSummary], error) {
	var dummy string
	err := repository.db.QueryRowContext(ctx, `
		SELECT a.id FROM artifacts a
		JOIN projects p ON p.id = a.project_id
		WHERE a.id = $1 AND a.project_id = $2 AND p.user_id = $3
	`, artifactID, projectID, userID).Scan(&dummy)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, artifact.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("verify artifact: %w", err)
	}

	limit := query.Limit
	if limit <= 0 || limit > 100 {
		limit = 20
	}

	scope := userID + "/" + projectID + "/" + artifactID + "/revisions"
	var afterVer int
	if query.Cursor != "" {
		value, err := decodeVerCursor(scope, query.Cursor)
		if err != nil {
			return nil, artifact.ErrInvalidInput
		}
		afterVer = value
	}

	rows, err := repository.db.QueryContext(ctx, `
		SELECT r.id, r.artifact_id, r.version_number, r.change_summary, r.content_format, r.author_id, r.author_type, r.created_at
		FROM artifact_revisions r
		JOIN artifacts a ON a.id = r.artifact_id AND a.project_id = r.project_id
		JOIN projects p ON p.id = a.project_id
		WHERE r.artifact_id = $1 AND r.project_id = $2 AND p.user_id = $3
		  AND ($4 = 0 OR r.version_number < $4)
		ORDER BY r.version_number DESC
		LIMIT $5
	`, artifactID, projectID, userID, afterVer, limit+1)
	if err != nil {
		return nil, fmt.Errorf("query revisions: %w", err)
	}
	defer rows.Close()

	var items []*artifact.RevisionSummary
	for rows.Next() {
		var service artifact.RevisionSummary
		if err := rows.Scan(
			&service.ID, &service.ArtifactID, &service.VersionNumber, &service.ChangeSummary, &service.ContentFormat,
			&service.AuthorID, &service.AuthorType, &service.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan revision summary: %w", err)
		}
		items = append(items, &service)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	var nextCursor *string
	if len(items) > limit {
		last := items[limit-1]
		cursor := encodeVerCursor(scope, last.VersionNumber)
		nextCursor = &cursor
		items = items[:limit]
	}

	if items == nil {
		items = []*artifact.RevisionSummary{}
	}

	return &artifact.PageResult[*artifact.RevisionSummary]{
		Items:      items,
		NextCursor: nextCursor,
	}, nil
}
