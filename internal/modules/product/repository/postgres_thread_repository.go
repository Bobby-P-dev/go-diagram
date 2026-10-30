package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"github.com/Bobby-P-dev/go-diagram.git/internal/modules/product"
	"time"
)

func (repository *PostgresRepository) CreateThread(
	ctx context.Context,
	userID, projectID string,
	input product.ThreadInput,
	write product.WriteRequest,
) (*product.ChatThread, error) {
	tx, err := repository.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	var existingProjID string
	err = tx.QueryRowContext(ctx, "SELECT id FROM projects WHERE id = $1 AND user_id = $2 FOR SHARE", projectID, userID).Scan(&existingProjID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, product.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("verify project: %w", err)
	}

	var cached product.ChatThread
	cachedHit, err := repository.checkOrRecordIdempotency(ctx, tx, userID, projectID, write, &cached)
	if err != nil {
		return nil, err
	}
	if cachedHit {
		return &cached, nil
	}

	if input.ActiveArtifactID != nil {
		var artifactID string
		err = tx.QueryRowContext(ctx, "SELECT id FROM artifacts WHERE id = $1 AND project_id = $2 AND archived_at IS NULL FOR SHARE", *input.ActiveArtifactID, projectID).Scan(&artifactID)
		if errors.Is(err, sql.ErrNoRows) {
			return nil, product.ErrNotFound
		}
		if err != nil {
			return nil, fmt.Errorf("verify active artifact: %w", err)
		}
	}

	modeHint := input.ModeHint
	if modeHint == "" {
		modeHint = "general"
	}

	var timestamp product.ChatThread
	err = tx.QueryRowContext(ctx, `
		INSERT INTO chat_threads (project_id, title, stage_hint, active_artifact_id, metadata, created_at, updated_at)
		VALUES ($1, $2, $3, $4, '{}'::jsonb, NOW(), NOW())
		RETURNING id, project_id, title, stage_hint, active_artifact_id, metadata, created_at, updated_at
	`, projectID, input.Title, modeHint, input.ActiveArtifactID).Scan(
		&timestamp.ID, &timestamp.ProjectID, &timestamp.Title, &timestamp.ModeHint, &timestamp.ActiveArtifactID, &timestamp.Metadata, &timestamp.CreatedAt, &timestamp.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("insert thread: %w", err)
	}

	if err := repository.completeIdempotency(ctx, tx, userID, projectID, write, 201, &timestamp); err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit tx: %w", err)
	}
	return &timestamp, nil
}

func (repository *PostgresRepository) ListThreads(
	ctx context.Context,
	userID, projectID string,
	query product.PageQuery,
) (*product.PageResult[*product.ChatThread], error) {
	var dummy string
	err := repository.db.QueryRowContext(ctx, "SELECT id FROM projects WHERE id = $1 AND user_id = $2", projectID, userID).Scan(&dummy)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, product.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("verify project: %w", err)
	}

	limit := query.Limit
	if limit <= 0 || limit > 100 {
		limit = 20
	}

	var afterTime *time.Time
	var afterID *string
	if query.Cursor != "" {
		timestamp, id, err := decodeTimeIDCursor(userID+"/"+projectID+"/threads", query.Cursor)
		if err != nil {
			return nil, product.ErrInvalidInput
		}
		afterTime = &timestamp
		afterID = &id
	}

	rows, err := repository.db.QueryContext(ctx, `
		SELECT t.id, t.project_id, t.title, t.stage_hint, t.active_artifact_id, t.metadata, t.created_at, t.updated_at
		FROM chat_threads t JOIN projects p ON p.id = t.project_id
		WHERE t.project_id = $1 AND p.user_id = $5
		  AND ($2::timestamptz IS NULL OR (t.created_at, t.id) < ($2::timestamptz, $3::uuid))
		ORDER BY t.created_at DESC, t.id DESC
		LIMIT $4
	`, projectID, afterTime, afterID, limit+1, userID)
	if err != nil {
		return nil, fmt.Errorf("query threads: %w", err)
	}
	defer rows.Close()

	var items []*product.ChatThread
	for rows.Next() {
		var timestamp product.ChatThread
		if err := rows.Scan(&timestamp.ID, &timestamp.ProjectID, &timestamp.Title, &timestamp.ModeHint, &timestamp.ActiveArtifactID, &timestamp.Metadata, &timestamp.CreatedAt, &timestamp.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan thread: %w", err)
		}
		items = append(items, &timestamp)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	var nextCursor *string
	if len(items) > limit {
		last := items[limit-1]
		cursor := encodeTimeIDCursor(userID+"/"+projectID+"/threads", last.CreatedAt, last.ID)
		nextCursor = &cursor
		items = items[:limit]
	}

	if items == nil {
		items = []*product.ChatThread{}
	}

	return &product.PageResult[*product.ChatThread]{
		Items:      items,
		NextCursor: nextCursor,
	}, nil
}
