package models

import (
	"database/sql"
	"fmt"

	"github.com/Bobby-P-dev/go-diagram.git/src/entities"
)

type UITemplateModelInterface interface {
	GetAllUITemplates() ([]entities.UITemplate, error)
	GetUITemplateByID(id string) (*entities.UITemplate, error)
}

type UITemplateModel struct {
	db *sql.DB
}

func NewUITemplateModel(db *sql.DB) *UITemplateModel {
	return &UITemplateModel{db: db}
}

func (m *UITemplateModel) GetAllUITemplates() ([]entities.UITemplate, error) {
	query := `
		SELECT id, name, category, device, description, theme, sections, code_export, is_featured, created_at
		FROM ui_templates
		ORDER BY is_featured DESC, name ASC
	`
	rows, err := m.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to query ui templates: %w", err)
	}
	defer rows.Close()

	var templates []entities.UITemplate
	for rows.Next() {
		var t entities.UITemplate
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
	return templates, nil
}

func (m *UITemplateModel) GetUITemplateByID(id string) (*entities.UITemplate, error) {
	query := `
		SELECT id, name, category, device, description, theme, sections, code_export, is_featured, created_at
		FROM ui_templates
		WHERE id = $1
	`
	var t entities.UITemplate
	err := m.db.QueryRow(query, id).Scan(
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
