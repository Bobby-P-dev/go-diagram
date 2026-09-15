package models

import (
	"database/sql"
	"fmt"

	"github.com/Bobby-P-dev/go-diagram.git/src/entities"
)

type TemplateModelInterface interface {
	GetAllTemplates() ([]entities.DiagramTemplate, error)
	GetTemplateByID(id string) (*entities.DiagramTemplate, error)
}

type TemplateModel struct {
	db *sql.DB
}

func NewTemplateModel(db *sql.DB) *TemplateModel {
	return &TemplateModel{db: db}
}

func (m *TemplateModel) GetAllTemplates() ([]entities.DiagramTemplate, error) {
	query := `
		SELECT id, name, category, description, diagram_type, nodes, edges, is_featured, created_at
		FROM diagram_templates
		ORDER BY is_featured DESC, name ASC
	`
	rows, err := m.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to query templates: %w", err)
	}
	defer rows.Close()

	var templates []entities.DiagramTemplate
	for rows.Next() {
		var t entities.DiagramTemplate
		err := rows.Scan(
			&t.ID,
			&t.Name,
			&t.Category,
			&t.Description,
			&t.DiagramType,
			&t.Nodes,
			&t.Edges,
			&t.IsFeatured,
			&t.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan template: %w", err)
		}
		templates = append(templates, t)
	}
	return templates, nil
}

func (m *TemplateModel) GetTemplateByID(id string) (*entities.DiagramTemplate, error) {
	query := `
		SELECT id, name, category, description, diagram_type, nodes, edges, is_featured, created_at
		FROM diagram_templates
		WHERE id = $1
	`
	var t entities.DiagramTemplate
	err := m.db.QueryRow(query, id).Scan(
		&t.ID,
		&t.Name,
		&t.Category,
		&t.Description,
		&t.DiagramType,
		&t.Nodes,
		&t.Edges,
		&t.IsFeatured,
		&t.CreatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("template not found")
		}
		return nil, fmt.Errorf("failed to get template: %w", err)
	}
	return &t, nil
}
