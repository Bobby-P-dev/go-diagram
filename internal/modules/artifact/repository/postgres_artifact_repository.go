package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/Bobby-P-dev/go-diagram.git/internal/modules/artifact"
	"github.com/lib/pq"
)

func (repository *PostgresRepository) CreateArtifact(
	ctx context.Context,
	userID, projectID string,
	input artifact.CreateArtifactInput,
	write artifact.WriteRequest,
) (*artifact.ArtifactDetail, error) {
	tx, err := repository.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	var dummy string
	err = tx.QueryRowContext(ctx, "SELECT id FROM projects WHERE id = $1 AND user_id = $2 FOR SHARE", projectID, userID).Scan(&dummy)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, artifact.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("verify project: %w", err)
	}

	var cached artifact.ArtifactDetail
	cachedHit, err := repository.checkOrRecordIdempotency(ctx, tx, userID, projectID, write, &cached)
	if err != nil {
		return nil, err
	}
	if cachedHit {
		return &cached, nil
	}

	format, err := artifact.ResolveFormat(input.Type)
	if err != nil {
		return nil, err
	}

	slug := input.Slug
	if slug == "" {
		slug = artifact.Slugify(input.Title)
	}
	if err := artifact.ValidateSlug(slug); err != nil {
		return nil, err
	}

	if err := artifact.ValidateContent(format, input.DocumentContent, input.GraphNodes, input.GraphEdges, input.FrameData); err != nil {
		return nil, err
	}

	schemaVersion := input.SchemaVersion
	if schemaVersion == 0 {
		schemaVersion = 1
	}

	if input.TriggerMessageID != nil {
		var msgID string
		err = tx.QueryRowContext(ctx, "SELECT id FROM chat_messages_v2 WHERE id = $1 AND project_id = $2", *input.TriggerMessageID, projectID).Scan(&msgID)
		if errors.Is(err, sql.ErrNoRows) {
			return nil, artifact.ErrNotFound
		}
		if err != nil {
			return nil, fmt.Errorf("verify trigger message: %w", err)
		}
	}

	var artifactID, revisionID string
	if err := tx.QueryRowContext(ctx, "SELECT uuid_generate_v4(), uuid_generate_v4()").Scan(&artifactID, &revisionID); err != nil {
		return nil, err
	}

	var art artifact.Artifact
	err = tx.QueryRowContext(ctx, `
		INSERT INTO artifacts (id, project_id, type, title, slug, format, current_version_number, current_revision_id, metadata, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, 1, $7, '{}'::jsonb, NOW(), NOW())
		RETURNING id, project_id, type, title, slug, format, current_version_number, current_revision_id, COALESCE(metadata, '{}'::jsonb), archived_at, created_at, updated_at
	`, artifactID, projectID, input.Type, input.Title, slug, format, revisionID).Scan(
		&art.ID, &art.ProjectID, &art.Type, &art.Title, &art.Slug, &art.Format,
		&art.CurrentVersionNumber, &art.CurrentRevisionID, &art.Metadata, &art.ArchivedAt, &art.CreatedAt, &art.UpdatedAt,
	)
	if err != nil {
		var pqErr *pq.Error
		if errors.As(err, &pqErr) && pqErr.Code == "23505" {
			return nil, artifact.ErrSlugConflict
		}
		return nil, fmt.Errorf("insert artifact: %w", err)
	}

	summary := input.ChangeSummary
	if summary == "" {
		summary = "Initial revision"
	}

	var rev artifact.ArtifactRevision
	err = tx.QueryRowContext(ctx, `
		INSERT INTO artifact_revisions (id, project_id, artifact_id, version_number, change_summary, content_format, schema_version, document_content, graph_nodes, graph_edges, frame_data, trigger_message_id, author_id, author_type, metadata, created_at)
		VALUES ($1, $2, $3, 1, $4, $5, $6, $7, $8, $9, $10, $11, $12, 'manual', '{}'::jsonb, NOW())
		RETURNING id, project_id, artifact_id, version_number, change_summary, content_format, schema_version, document_content, graph_nodes, graph_edges, frame_data, trigger_message_id, author_id, author_type, COALESCE(metadata, '{}'::jsonb), created_at
	`, revisionID, projectID, artifactID, summary, format, schemaVersion, input.DocumentContent, jsonParameter(input.GraphNodes), jsonParameter(input.GraphEdges), jsonParameter(input.FrameData), input.TriggerMessageID, userID).Scan(
		&rev.ID, &rev.ProjectID, &rev.ArtifactID, &rev.VersionNumber, &rev.ChangeSummary, &rev.ContentFormat,
		&rev.SchemaVersion, &rev.DocumentContent, nullableJSON{&rev.GraphNodes}, nullableJSON{&rev.GraphEdges}, nullableJSON{&rev.FrameData},
		&rev.TriggerMessageID, &rev.AuthorID, &rev.AuthorType, &rev.Metadata, &rev.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("insert initial revision: %w", err)
	}

	if err := insertContextLinks(ctx, tx, projectID, artifactID, 1, input.Sources); err != nil {
		return nil, err
	}
	rev.Sources = input.Sources
	if rev.Sources == nil {
		rev.Sources = []artifact.RevisionRef{}
	}

	detail := &artifact.ArtifactDetail{
		Artifact:        art,
		CurrentRevision: &rev,
	}

	if err := repository.completeIdempotency(ctx, tx, userID, projectID, write, 201, detail); err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit tx: %w", err)
	}
	return detail, nil
}

func (repository *PostgresRepository) GetArtifact(
	ctx context.Context,
	userID, projectID, artifactID string,
) (*artifact.ArtifactDetail, error) {
	var art artifact.Artifact
	var rev artifact.ArtifactRevision

	err := repository.db.QueryRowContext(ctx, `
		SELECT a.id, a.project_id, a.type, a.title, a.slug, a.format, a.current_version_number, a.current_revision_id, COALESCE(a.metadata, '{}'::jsonb), a.archived_at, a.created_at, a.updated_at,
		       r.id, r.project_id, r.artifact_id, r.version_number, r.change_summary, r.content_format, r.schema_version, r.document_content, r.graph_nodes, r.graph_edges, r.frame_data, r.trigger_message_id, r.author_id, r.author_type, COALESCE(r.metadata, '{}'::jsonb), r.created_at
		FROM artifacts a
		JOIN projects p ON p.id = a.project_id
		JOIN artifact_revisions r ON r.id = a.current_revision_id AND r.artifact_id = a.id AND r.project_id = a.project_id
		WHERE a.id = $1 AND a.project_id = $2 AND p.user_id = $3 AND a.archived_at IS NULL
	`, artifactID, projectID, userID).Scan(
		&art.ID, &art.ProjectID, &art.Type, &art.Title, &art.Slug, &art.Format,
		&art.CurrentVersionNumber, &art.CurrentRevisionID, &art.Metadata, &art.ArchivedAt, &art.CreatedAt, &art.UpdatedAt,
		&rev.ID, &rev.ProjectID, &rev.ArtifactID, &rev.VersionNumber, &rev.ChangeSummary, &rev.ContentFormat,
		&rev.SchemaVersion, &rev.DocumentContent, nullableJSON{&rev.GraphNodes}, nullableJSON{&rev.GraphEdges}, nullableJSON{&rev.FrameData},
		&rev.TriggerMessageID, &rev.AuthorID, &rev.AuthorType, &rev.Metadata, &rev.CreatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, artifact.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("query artifact detail: %w", err)
	}

	sources, err := loadContextLinks(ctx, repository.db, projectID, artifactID, rev.VersionNumber)
	if err != nil {
		return nil, err
	}
	rev.Sources = sources

	return &artifact.ArtifactDetail{
		Artifact:        art,
		CurrentRevision: &rev,
	}, nil
}

func (repository *PostgresRepository) ListArtifacts(
	ctx context.Context,
	userID, projectID string,
	query artifact.PageQuery,
) (*artifact.PageResult[*artifact.Artifact], error) {
	var dummy string
	err := repository.db.QueryRowContext(ctx, "SELECT id FROM projects WHERE id = $1 AND user_id = $2", projectID, userID).Scan(&dummy)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, artifact.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("verify project: %w", err)
	}

	limit := query.Limit
	if limit <= 0 || limit > 100 {
		limit = 20
	}

	scope := userID + "/" + projectID + "/artifacts/" + query.TypeFilter
	var afterTime *time.Time
	var afterID *string
	if query.Cursor != "" {
		timestamp, id, err := decodeTimeIDCursor(scope, query.Cursor)
		if err != nil {
			return nil, artifact.ErrInvalidInput
		}
		afterTime = &timestamp
		afterID = &id
	}

	rows, err := repository.db.QueryContext(ctx, `
		SELECT a.id, a.project_id, a.type, a.title, a.slug, a.format, a.current_version_number, a.current_revision_id, COALESCE(a.metadata, '{}'::jsonb), a.archived_at, a.created_at, a.updated_at
		FROM artifacts a
		JOIN projects p ON p.id = a.project_id
		WHERE a.project_id = $1 AND p.user_id = $2 AND a.archived_at IS NULL
		  AND ($3 = '' OR a.type = $3)
		  AND ($4::timestamptz IS NULL OR (a.created_at, a.id) < ($4::timestamptz, $5::uuid))
		ORDER BY a.created_at DESC, a.id DESC
		LIMIT $6
	`, projectID, userID, query.TypeFilter, afterTime, afterID, limit+1)
	if err != nil {
		return nil, fmt.Errorf("query artifacts: %w", err)
	}
	defer rows.Close()

	var items []*artifact.Artifact
	for rows.Next() {
		var record artifact.Artifact
		if err := rows.Scan(
			&record.ID, &record.ProjectID, &record.Type, &record.Title, &record.Slug, &record.Format,
			&record.CurrentVersionNumber, &record.CurrentRevisionID, &record.Metadata, &record.ArchivedAt, &record.CreatedAt, &record.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan artifact: %w", err)
		}
		items = append(items, &record)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	var nextCursor *string
	if len(items) > limit {
		last := items[limit-1]
		cursor := encodeTimeIDCursor(scope, last.CreatedAt, last.ID)
		nextCursor = &cursor
		items = items[:limit]
	}

	if items == nil {
		items = []*artifact.Artifact{}
	}

	return &artifact.PageResult[*artifact.Artifact]{
		Items:      items,
		NextCursor: nextCursor,
	}, nil
}
