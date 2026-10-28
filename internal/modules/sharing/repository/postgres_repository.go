package repository

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/Bobby-P-dev/go-diagram.git/internal/modules/sharing"
	"github.com/Bobby-P-dev/go-diagram.git/internal/modules/sharing/service"
)

type postgresShareRepository struct {
	db *sql.DB
}

func NewPostgresRepository(db *sql.DB) service.ShareRepository {
	return &postgresShareRepository{db: db}
}

func (repo *postgresShareRepository) FindByProjectID(ctx context.Context, projectID string) (*sharing.Share, error) {
	query := `
		SELECT id, share_token, project_id, title, is_active, view_count, created_at, updated_at
		FROM project_shares
		WHERE project_id = $1
	`
	var s sharing.Share
	err := repo.db.QueryRowContext(ctx, query, projectID).Scan(
		&s.ID,
		&s.ShareToken,
		&s.ProjectID,
		&s.Title,
		&s.IsActive,
		&s.ViewCount,
		&s.CreatedAt,
		&s.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &s, nil
}

func (repo *postgresShareRepository) FindActiveByToken(ctx context.Context, token string) (*sharing.Share, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return nil, fmt.Errorf("share token is required")
	}

	query := `
		SELECT id, share_token, project_id, title, is_active, view_count, created_at, updated_at
		FROM project_shares
		WHERE share_token = $1 AND is_active = true
	`
	var s sharing.Share
	err := repo.db.QueryRowContext(ctx, query, token).Scan(
		&s.ID,
		&s.ShareToken,
		&s.ProjectID,
		&s.Title,
		&s.IsActive,
		&s.ViewCount,
		&s.CreatedAt,
		&s.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("shared project not found or sharing has been disabled")
		}
		return nil, fmt.Errorf("failed to query share: %w", err)
	}
	return &s, nil
}

func (repo *postgresShareRepository) Create(ctx context.Context, input sharing.NewShare) (*sharing.Share, error) {
	insertQuery := `
		INSERT INTO project_shares (share_token, project_id, title, is_active, view_count, created_at, updated_at)
		VALUES ($1, $2, $3, true, 0, NOW(), NOW())
		RETURNING id, share_token, project_id, title, is_active, view_count, created_at, updated_at
	`
	var s sharing.Share
	err := repo.db.QueryRowContext(ctx, insertQuery, input.Token, input.ProjectID, input.Title).Scan(
		&s.ID,
		&s.ShareToken,
		&s.ProjectID,
		&s.Title,
		&s.IsActive,
		&s.ViewCount,
		&s.CreatedAt,
		&s.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create project share: %w", err)
	}
	return &s, nil
}

func (repo *postgresShareRepository) Reactivate(ctx context.Context, id, title string) (*sharing.Share, error) {
	updateQuery := `
		UPDATE project_shares
		SET is_active = true, title = $1, updated_at = NOW()
		WHERE id = $2
		RETURNING id, share_token, project_id, title, is_active, view_count, created_at, updated_at
	`
	var s sharing.Share
	err := repo.db.QueryRowContext(ctx, updateQuery, title, id).Scan(
		&s.ID,
		&s.ShareToken,
		&s.ProjectID,
		&s.Title,
		&s.IsActive,
		&s.ViewCount,
		&s.CreatedAt,
		&s.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to reactivate existing share: %w", err)
	}
	return &s, nil
}

func (repo *postgresShareRepository) Revoke(ctx context.Context, projectID string) error {
	query := `
		UPDATE project_shares
		SET is_active = false, updated_at = NOW()
		WHERE project_id = $1
	`
	res, err := repo.db.ExecContext(ctx, query, projectID)
	if err != nil {
		return fmt.Errorf("failed to revoke share: %w", err)
	}
	rowsAffected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to count revoked shares: %w", err)
	}
	if rowsAffected == 0 {
		return fmt.Errorf("no share record found for project %s", projectID)
	}
	return nil
}

func (repo *postgresShareRepository) IncrementViewCount(ctx context.Context, shareID string) error {
	_, err := repo.db.ExecContext(ctx, "UPDATE project_shares SET view_count = view_count + 1 WHERE id = $1", shareID)
	return err
}
