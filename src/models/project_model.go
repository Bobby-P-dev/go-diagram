package models

import (
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/Bobby-P-dev/go-diagram.git/src/entities"
)

type ProjectModelInterface interface {
	Create(title, diagramType string, nodes, edges json.RawMessage) (*entities.Project, error)
	CreateWithMode(title, diagramType, projectMode string, metadata, nodes, edges json.RawMessage) (*entities.Project, error)
	GetByID(id string) (*entities.Project, []entities.ChatMessage, error)
	UpdateGraph(id string, nodes, edges json.RawMessage) error
	GetAll() ([]entities.Project, error)
	GetPaginated(limit, offset int) ([]entities.ProjectSummary, int, error)
	TogglePin(id string) (bool, error)
}

type projectModel struct {
	db *sql.DB
}

func NewProjectModel(db *sql.DB) ProjectModelInterface {
	return &projectModel{db: db}
}

func (m *projectModel) Create(title, diagramType string, nodes, edges json.RawMessage) (*entities.Project, error) {
	return m.CreateWithMode(title, diagramType, "diagram", json.RawMessage("{}"), nodes, edges)
}

func (m *projectModel) CreateWithMode(title, diagramType, projectMode string, metadata, nodes, edges json.RawMessage) (*entities.Project, error) {
	if len(nodes) == 0 {
		nodes = json.RawMessage("[]")
	}
	if len(edges) == 0 {
		edges = json.RawMessage("[]")
	}
	if len(metadata) == 0 {
		metadata = json.RawMessage("{}")
	}
	if projectMode == "" {
		projectMode = "diagram"
	}

	query := `
		INSERT INTO projects (title, diagram_type, project_mode, metadata, current_nodes, current_edges, is_pinned, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, false, NOW(), NOW())
		RETURNING id, title, diagram_type, project_mode, metadata, is_pinned, current_nodes, current_edges, created_at, updated_at
	`

	var p entities.Project
	err := m.db.QueryRow(query, title, diagramType, projectMode, metadata, nodes, edges).Scan(
		&p.ID,
		&p.Title,
		&p.DiagramType,
		&p.ProjectMode,
		&p.Metadata,
		&p.IsPinned,
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
		SELECT id, title, diagram_type, COALESCE(project_mode, 'diagram'), COALESCE(metadata, '{}'::jsonb), is_pinned, current_nodes, current_edges, created_at, updated_at
		FROM projects
		WHERE id = $1
	`

	var p entities.Project
	err := m.db.QueryRow(queryProject, id).Scan(
		&p.ID,
		&p.Title,
		&p.DiagramType,
		&p.ProjectMode,
		&p.Metadata,
		&p.IsPinned,
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
			return nil, nil, fmt.Errorf("failed to scan chat message: %w", err)
		}
		if len(rawTarget) > 0 {
			msg.TargetNodeIDs = json.RawMessage(rawTarget)
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
		SELECT id, title, diagram_type, COALESCE(project_mode, 'diagram'), COALESCE(metadata, '{}'::jsonb), is_pinned, current_nodes, current_edges, created_at, updated_at
		FROM projects
		ORDER BY is_pinned DESC, updated_at DESC
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
			&p.ProjectMode,
			&p.Metadata,
			&p.IsPinned,
			&p.CurrentNodes,
			&p.CurrentEdges,
			&p.CreatedAt,
			&p.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan project: %w", err)
		}
		projects = append(projects, p)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating project rows: %w", err)
	}

	return projects, nil
}

func (m *projectModel) GetPaginated(limit, offset int) ([]entities.ProjectSummary, int, error) {
	if limit <= 0 {
		limit = 15
	}
	if limit > 100 {
		limit = 100
	}
	if offset < 0 {
		offset = 0
	}

	// 1. Get total count
	var totalCount int
	err := m.db.QueryRow("SELECT COUNT(*) FROM projects").Scan(&totalCount)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count projects: %w", err)
	}

	// 2. Query ONLY essential fields (NO current_nodes/current_edges for maximum performance)
	query := `
		SELECT id, title, diagram_type, COALESCE(project_mode, 'diagram'), is_pinned, created_at, updated_at
		FROM projects
		ORDER BY is_pinned DESC, updated_at DESC
		LIMIT $1 OFFSET $2
	`

	rows, err := m.db.Query(query, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to query paginated projects: %w", err)
	}
	defer rows.Close()

	items := make([]entities.ProjectSummary, 0, limit)
	for rows.Next() {
		var s entities.ProjectSummary
		err := rows.Scan(
			&s.ID,
			&s.Title,
			&s.DiagramType,
			&s.ProjectMode,
			&s.IsPinned,
			&s.CreatedAt,
			&s.UpdatedAt,
		)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to scan project summary: %w", err)
		}
		items = append(items, s)
	}

	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("error iterating paginated project rows: %w", err)
	}

	return items, totalCount, nil
}

func (m *projectModel) TogglePin(id string) (bool, error) {
	query := `
		UPDATE projects
		SET is_pinned = NOT is_pinned, updated_at = NOW()
		WHERE id = $1
		RETURNING is_pinned
	`
	var isPinned bool
	err := m.db.QueryRow(query, id).Scan(&isPinned)
	if err != nil {
		if err == sql.ErrNoRows {
			return false, fmt.Errorf("project not found: %s", id)
		}
		return false, fmt.Errorf("failed to toggle project pin: %w", err)
	}
	return isPinned, nil
}
