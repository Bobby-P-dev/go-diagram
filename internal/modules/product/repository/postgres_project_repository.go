package repository

import (
	"context"
	"fmt"
	"github.com/Bobby-P-dev/go-diagram.git/internal/modules/product"
	"time"
)

func (repository *PostgresRepository) CreateProject(
	ctx context.Context,
	userID string,
	input product.ProjectInput,
	write product.WriteRequest,
) (*product.Project, error) {
	tx, err := repository.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	var cached product.Project
	cachedHit, err := repository.checkOrRecordIdempotency(ctx, tx, userID, "", write, &cached)
	if err != nil {
		return nil, err
	}
	if cachedHit {
		return &cached, nil
	}

	var project product.Project
	err = tx.QueryRowContext(ctx, `
		INSERT INTO projects (user_id, title, description, diagram_type, project_mode, is_pinned, metadata, current_nodes, current_edges, created_at, updated_at)
		VALUES ($1, $2, $3, 'none', 'connected', false, '{}'::jsonb, '[]'::jsonb, '[]'::jsonb, NOW(), NOW())
		RETURNING id, user_id, title, description, is_pinned, metadata, created_at, updated_at
	`, userID, input.Title, input.Description).Scan(
		&project.ID, &project.UserID, &project.Title, &project.Description, &project.IsPinned, &project.Metadata, &project.CreatedAt, &project.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("insert project: %w", err)
	}

	if err := repository.completeIdempotency(ctx, tx, userID, project.ID, write, 201, &project); err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit tx: %w", err)
	}
	return &project, nil
}

func (repository *PostgresRepository) ListProjects(
	ctx context.Context,
	userID string,
	query product.PageQuery,
) (*product.PageResult[*product.Project], error) {
	limit := query.Limit
	if limit <= 0 || limit > 100 {
		limit = 20
	}

	var afterTime *time.Time
	var afterID *string
	if query.Cursor != "" {
		timestamp, id, err := decodeTimeIDCursor(userID+"/projects", query.Cursor)
		if err != nil {
			return nil, product.ErrInvalidInput
		}
		afterTime = &timestamp
		afterID = &id
	}

	rows, err := repository.db.QueryContext(ctx, `
		SELECT id, user_id, title, description, is_pinned, COALESCE(metadata, '{}'::jsonb), created_at, updated_at
		FROM projects
		WHERE user_id = $1
		  AND ($2::timestamptz IS NULL OR (created_at, id) < ($2::timestamptz, $3::uuid))
		ORDER BY created_at DESC, id DESC
		LIMIT $4
	`, userID, afterTime, afterID, limit+1)
	if err != nil {
		return nil, fmt.Errorf("query projects: %w", err)
	}
	defer rows.Close()

	var items []*product.Project
	for rows.Next() {
		var project product.Project
		if err := rows.Scan(&project.ID, &project.UserID, &project.Title, &project.Description, &project.IsPinned, &project.Metadata, &project.CreatedAt, &project.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan project: %w", err)
		}
		items = append(items, &project)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	var nextCursor *string
	if len(items) > limit {
		last := items[limit-1]
		cursor := encodeTimeIDCursor(userID+"/projects", last.CreatedAt, last.ID)
		nextCursor = &cursor
		items = items[:limit]
	}

	if items == nil {
		items = []*product.Project{}
	}

	return &product.PageResult[*product.Project]{
		Items:      items,
		NextCursor: nextCursor,
	}, nil
}
