package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/Bobby-P-dev/go-diagram.git/internal/modules/artifact"
)

func (repository *PostgresRepository) AppendRevision(
	ctx context.Context,
	userID, projectID, artifactID string,
	input artifact.AppendRevisionInput,
	write artifact.WriteRequest,
) (*artifact.ArtifactRevision, error) {
	tx, err := repository.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	var artFormat string
	var currentVer int
	var currentRevID string

	err = tx.QueryRowContext(ctx, `
		SELECT a.format, a.current_version_number, a.current_revision_id
		FROM artifacts a
		JOIN projects p ON p.id = a.project_id
		WHERE a.id = $1 AND a.project_id = $2 AND p.user_id = $3 AND a.archived_at IS NULL
		FOR UPDATE OF a
	`, artifactID, projectID, userID).Scan(&artFormat, &currentVer, &currentRevID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, artifact.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("lock artifact: %w", err)
	}

	var cached artifact.ArtifactRevision
	cachedHit, err := repository.checkOrRecordIdempotency(ctx, tx, userID, projectID, write, &cached)
	if err != nil {
		return nil, err
	}
	if cachedHit {
		return &cached, nil
	}
	if currentRevID != input.ExpectedRevisionID {
		return nil, artifact.ErrRevisionConflict
	}

	if err := artifact.ValidateContent(artFormat, input.DocumentContent, input.GraphNodes, input.GraphEdges, input.FrameData); err != nil {
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

	var newRevID string
	if err := tx.QueryRowContext(ctx, "SELECT uuid_generate_v4()").Scan(&newRevID); err != nil {
		return nil, err
	}

	nextVer := currentVer + 1
	summary := input.ChangeSummary
	if summary == "" {
		summary = fmt.Sprintf("Revision %d", nextVer)
	}

	var rev artifact.ArtifactRevision
	err = tx.QueryRowContext(ctx, `
		INSERT INTO artifact_revisions (id, project_id, artifact_id, version_number, change_summary, content_format, schema_version, document_content, graph_nodes, graph_edges, frame_data, trigger_message_id, author_id, author_type, metadata, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, 'manual', '{}'::jsonb, NOW())
		RETURNING id, project_id, artifact_id, version_number, change_summary, content_format, schema_version, document_content, graph_nodes, graph_edges, frame_data, trigger_message_id, author_id, author_type, COALESCE(metadata, '{}'::jsonb), created_at
	`, newRevID, projectID, artifactID, nextVer, summary, artFormat, schemaVersion, input.DocumentContent, jsonParameter(input.GraphNodes), jsonParameter(input.GraphEdges), jsonParameter(input.FrameData), input.TriggerMessageID, userID).Scan(
		&rev.ID, &rev.ProjectID, &rev.ArtifactID, &rev.VersionNumber, &rev.ChangeSummary, &rev.ContentFormat,
		&rev.SchemaVersion, &rev.DocumentContent, nullableJSON{&rev.GraphNodes}, nullableJSON{&rev.GraphEdges}, nullableJSON{&rev.FrameData},
		&rev.TriggerMessageID, &rev.AuthorID, &rev.AuthorType, &rev.Metadata, &rev.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("insert revision: %w", err)
	}

	if err := insertContextLinks(ctx, tx, projectID, artifactID, nextVer, input.Sources); err != nil {
		return nil, err
	}
	rev.Sources = input.Sources
	if rev.Sources == nil {
		rev.Sources = []artifact.RevisionRef{}
	}

	_, err = tx.ExecContext(ctx, `
		UPDATE artifacts
		SET current_version_number = $1, current_revision_id = $2, updated_at = NOW()
		WHERE id = $3 AND project_id = $4
	`, nextVer, newRevID, artifactID, projectID)
	if err != nil {
		return nil, fmt.Errorf("update artifact current pointer: %w", err)
	}

	if err := repository.completeIdempotency(ctx, tx, userID, projectID, write, 201, &rev); err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit tx: %w", err)
	}
	return &rev, nil
}
