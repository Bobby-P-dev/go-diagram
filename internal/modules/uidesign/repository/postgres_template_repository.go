package repository

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/Bobby-P-dev/go-diagram.git/internal/modules/uidesign"
)

type PostgresTemplateRepository struct {
	db *sql.DB
}

func NewPostgresTemplateRepository(db *sql.DB) *PostgresTemplateRepository {
	return &PostgresTemplateRepository{db: db}
}

func (r *PostgresTemplateRepository) GetAllUITemplates(ctx context.Context) ([]uidesign.UITemplate, error) {
	query := `
		SELECT id, name, category, device, description, theme, sections, code_export, is_featured, created_at
		FROM ui_templates
		ORDER BY is_featured DESC, name ASC
	`
	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query ui templates: %w", err)
	}
	defer rows.Close()

	var templates []uidesign.UITemplate
	for rows.Next() {
		var t uidesign.UITemplate
		err := rows.Scan(
			&t.ID,
			&t.Name,
			&t.Category,
			&t.Device,
			&t.Description,
			&t.Theme,
			&t.Sections,
			&t.CodeExport,
			&t.IsFeatured,
			&t.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan ui template: %w", err)
		}
		templates = append(templates, t)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate ui template rows: %w", err)
	}

	return templates, nil
}

func (r *PostgresTemplateRepository) GetUITemplateByID(ctx context.Context, id string) (*uidesign.UITemplate, error) {
	query := `
		SELECT id, name, category, device, description, theme, sections, code_export, is_featured, created_at
		FROM ui_templates
		WHERE id = $1
	`
	var t uidesign.UITemplate
	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&t.ID,
		&t.Name,
		&t.Category,
		&t.Device,
		&t.Description,
		&t.Theme,
		&t.Sections,
		&t.CodeExport,
		&t.IsFeatured,
		&t.CreatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("ui template not found")
		}
		return nil, fmt.Errorf("failed to get ui template: %w", err)
	}
	return &t, nil
}
