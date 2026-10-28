package repository

import (
	"database/sql"
	"fmt"

	"github.com/Bobby-P-dev/go-diagram.git/internal/modules/diagram"
)

type TemplateRepository interface {
	GetAllTemplates() ([]diagram.DiagramTemplate, error)
	GetTemplateByID(id string) (*diagram.DiagramTemplate, error)
}

type PostgresTemplateRepository struct {
	db *sql.DB
}

func NewTemplateRepository(db *sql.DB) *PostgresTemplateRepository {
	return &PostgresTemplateRepository{db: db}
}

func (m *PostgresTemplateRepository) GetAllTemplates() ([]diagram.DiagramTemplate, error) {
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

	var templates []diagram.DiagramTemplate
	for rows.Next() {
		var t diagram.DiagramTemplate
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

func (m *PostgresTemplateRepository) GetTemplateByID(id string) (*diagram.DiagramTemplate, error) {
	query := `
		SELECT id, name, category, description, diagram_type, nodes, edges, is_featured, created_at
		FROM diagram_templates
		WHERE id = $1
	`
	var t diagram.DiagramTemplate
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
