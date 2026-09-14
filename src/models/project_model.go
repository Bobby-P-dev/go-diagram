package models

import (
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/Bobby-P-dev/go-diagram.git/src/entities"
)

type ProjectModelInterface interface {
	Create(title, diagramType string, nodes, edges json.RawMessage) (*entities.Project, error)
	GetByID(id string) (*entities.Project, []entities.ChatMessage, error)
	UpdateGraph(id string, nodes, edges json.RawMessage) error
	GetAll() ([]entities.Project, error)
}

type projectModel struct {
	db *sql.DB
}

func NewProjectModel(db *sql.DB) ProjectModelInterface {
	return &projectModel{db: db}
}

func (m *projectModel) Create(title, diagramType string, nodes, edges json.RawMessage) (*entities.Project, error) {
	if len(nodes) == 0 {
		nodes = json.RawMessage("[]")
	}
	if len(edges) == 0 {
		edges = json.RawMessage("[]")
	}

	query := `
		INSERT INTO projects (title, diagram_type, current_nodes, current_edges, created_at, updated_at)
		VALUES ($1, $2, $3, $4, NOW(), NOW())
		RETURNING id, title, diagram_type, current_nodes, current_edges, created_at, updated_at
	`

	var p entities.Project
	err := m.db.QueryRow(query, title, diagramType, nodes, edges).Scan(
		&p.ID,
		&p.Title,
		&p.DiagramType,
		&p.CurrentNodes,
		&p.CurrentEdges,
		&p.CreatedAt,
		&p.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create project: %w", err)
	}

	return &p, nil
}

func (m *projectModel) GetByID(id string) (*entities.Project, []entities.ChatMessage, error) {
	queryProject := `
		SELECT id, title, diagram_type, current_nodes, current_edges, created_at, updated_at
		FROM projects
		WHERE id = $1
	`

	var p entities.Project
	err := m.db.QueryRow(queryProject, id).Scan(
		&p.ID,
		&p.Title,
		&p.DiagramType,
		&p.CurrentNodes,
		&p.CurrentEdges,
		&p.CreatedAt,
		&p.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil, fmt.Errorf("project not found: %s", id)
		}
		return nil, nil, fmt.Errorf("failed to find project: %w", err)
	}

	queryMessages := `
		SELECT id, project_id, role, content, target_node_ids, created_at
		FROM chat_messages
		WHERE project_id = $1
		ORDER BY created_at ASC
	`
	rows, err := m.db.Query(queryMessages, id)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to query project chat messages: %w", err)
	}
	defer rows.Close()

	var messages []entities.ChatMessage
	for rows.Next() {
		var msg entities.ChatMessage
		var rawTarget []byte
		err := rows.Scan(
			&msg.ID,
			&msg.ProjectID,
			&msg.Role,
			&msg.Content,
			&rawTarget,
			&msg.CreatedAt,
		)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to scan message row: %w", err)
		}
		if rawTarget != nil {
			msg.TargetNodeIDs = rawTarget
		}
		messages = append(messages, msg)
	}

	if err := rows.Err(); err != nil {
		return nil, nil, fmt.Errorf("error iterating message rows: %w", err)
	}

	return &p, messages, nil
}

func (m *projectModel) UpdateGraph(id string, nodes, edges json.RawMessage) error {
	if len(nodes) == 0 {
		nodes = json.RawMessage("[]")
	}
	if len(edges) == 0 {
		edges = json.RawMessage("[]")
	}

	query := `
		UPDATE projects
		SET current_nodes = $1, current_edges = $2, updated_at = NOW()
		WHERE id = $3
	`

	res, err := m.db.Exec(query, nodes, edges, id)
	if err != nil {
		return fmt.Errorf("failed to update project graph: %w", err)
	}

	rowsAffected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to check rows affected: %w", err)
	}
	if rowsAffected == 0 {
		return fmt.Errorf("project not found: %s", id)
	}

	return nil
}

func (m *projectModel) GetAll() ([]entities.Project, error) {
	query := `
		SELECT id, title, diagram_type, current_nodes, current_edges, created_at, updated_at
		FROM projects
		ORDER BY updated_at DESC
	`

	rows, err := m.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to query projects: %w", err)
	}
	defer rows.Close()

	var projects []entities.Project
	for rows.Next() {
		var p entities.Project
		err := rows.Scan(
			&p.ID,
			&p.Title,
			&p.DiagramType,
			&p.CurrentNodes,
			&p.CurrentEdges,
			&p.CreatedAt,
			&p.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan project row: %w", err)
		}
		projects = append(projects, p)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating project rows: %w", err)
	}

	return projects, nil
}
