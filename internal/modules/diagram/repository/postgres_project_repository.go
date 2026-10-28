package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/Bobby-P-dev/go-diagram.git/internal/modules/diagram"
)

type ProjectRepository interface {
	Create(title, diagramType string, nodes, edges json.RawMessage) (*diagram.Project, error)
	CreateWithMode(title, diagramType, projectMode string, metadata, nodes, edges json.RawMessage) (*diagram.Project, error)
	CreateWithUser(userID, title, diagramType, projectMode string, metadata, nodes, edges json.RawMessage) (*diagram.Project, error)
	GetByID(id string) (*diagram.Project, []diagram.ChatMessage, error)
	GetByIDScoped(id, userID, userRole string) (*diagram.Project, []diagram.ChatMessage, error)
	UpdateGraph(id string, nodes, edges json.RawMessage) error
	GetAll() ([]diagram.Project, error)
	GetAllScoped(userID string) ([]diagram.Project, error)
	GetPaginated(limit, offset int) ([]diagram.ProjectSummary, int, error)
	GetPaginatedScoped(userID string, limit, offset int) ([]diagram.ProjectSummary, int, error)
	TogglePin(id string) (bool, error)
}

type PostgresProjectRepository struct {
	db *sql.DB
	fallbackOwnerID string
}

func NewProjectRepository(db *sql.DB, fallbackOwnerID string) *PostgresProjectRepository {
	return &PostgresProjectRepository{db: db, fallbackOwnerID: fallbackOwnerID}
}

func (m *PostgresProjectRepository) Create(title, diagramType string, nodes, edges json.RawMessage) (*diagram.Project, error) {
	return m.CreateWithUser("", title, diagramType, "diagram", json.RawMessage("{}"), nodes, edges)
}

func (m *PostgresProjectRepository) CreateWithMode(title, diagramType, projectMode string, metadata, nodes, edges json.RawMessage) (*diagram.Project, error) {
	return m.CreateWithUser("", title, diagramType, projectMode, metadata, nodes, edges)
}

func (m *PostgresProjectRepository) CreateWithUser(userID, title, diagramType, projectMode string, metadata, nodes, edges json.RawMessage) (*diagram.Project, error) {
	return m.CreateWithUserContext(context.Background(), userID, title, diagramType, projectMode, metadata, nodes, edges)
}

func (m *PostgresProjectRepository) CreateWithUserContext(ctx context.Context, userID, title, diagramType, projectMode string, metadata, nodes, edges json.RawMessage) (*diagram.Project, error) {
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
	if userID == "" {
		userID = m.fallbackOwnerID
	}

	query := `
		INSERT INTO projects (user_id, title, diagram_type, project_mode, metadata, current_nodes, current_edges, is_pinned, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, false, NOW(), NOW())
		RETURNING id, COALESCE(user_id, 'a0000000-0000-0000-0000-000000000001'::uuid), title, diagram_type, project_mode, metadata, is_pinned, current_nodes, current_edges, created_at, updated_at
	`

	var p diagram.Project
	err := m.db.QueryRowContext(ctx, query, userID, title, diagramType, projectMode, metadata, nodes, edges).Scan(
		&p.ID,
		&p.UserID,
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

func (m *PostgresProjectRepository) GetByID(id string) (*diagram.Project, []diagram.ChatMessage, error) {
	return m.GetByIDScoped(id, "", "")
}

func (m *PostgresProjectRepository) GetByIDScoped(id, userID, userRole string) (*diagram.Project, []diagram.ChatMessage, error) {
	return m.GetByIDScopedContext(context.Background(), id, userID, userRole)
}

func (m *PostgresProjectRepository) GetByIDScopedContext(ctx context.Context, id, userID, userRole string) (*diagram.Project, []diagram.ChatMessage, error) {
	queryProject := `
		SELECT id, COALESCE(user_id, 'a0000000-0000-0000-0000-000000000001'::uuid), title, diagram_type, COALESCE(project_mode, 'diagram'), COALESCE(metadata, '{}'::jsonb), is_pinned, current_nodes, current_edges, created_at, updated_at
		FROM projects
		WHERE id = $1
	`

	var p diagram.Project
	err := m.db.QueryRowContext(ctx, queryProject, id).Scan(
		&p.ID,
		&p.UserID,
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

	if userID != "" && p.UserID != userID {
		return nil, nil, fmt.Errorf("project not found or unauthorized: %s", id)
	}

	queryMessages := `
		SELECT id, project_id, role, content, target_node_ids, created_at
		FROM chat_messages
		WHERE project_id = $1
		ORDER BY created_at ASC
	`
	rows, err := m.db.QueryContext(ctx, queryMessages, id)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to query project chat messages: %w", err)
	}
	defer rows.Close()

	var messages []diagram.ChatMessage
	for rows.Next() {
		var msg diagram.ChatMessage
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
		if rawTarget != nil {
			msg.TargetNodeIDs = rawTarget
		}
		messages = append(messages, msg)
	}

	if err := rows.Err(); err != nil {
		return nil, nil, fmt.Errorf("error iterating chat messages: %w", err)
	}

	return &p, messages, nil
}

func (m *PostgresProjectRepository) UpdateGraph(id string, nodes, edges json.RawMessage) error {
	return m.UpdateGraphContext(context.Background(), id, nodes, edges)
}

func (m *PostgresProjectRepository) UpdateGraphContext(ctx context.Context, id string, nodes, edges json.RawMessage) error {
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

	res, err := m.db.ExecContext(ctx, query, nodes, edges, id)
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

func (m *PostgresProjectRepository) GetAll() ([]diagram.Project, error) {
	return m.GetAllScoped("")
}

func (m *PostgresProjectRepository) GetAllScoped(userID string) ([]diagram.Project, error) {
	var query string
	var rows *sql.Rows
	var err error

	if userID != "" {
		query = `
			SELECT id, COALESCE(user_id, 'a0000000-0000-0000-0000-000000000001'::uuid), title, diagram_type, COALESCE(project_mode, 'diagram'), COALESCE(metadata, '{}'::jsonb), is_pinned, current_nodes, current_edges, created_at, updated_at
			FROM projects
			WHERE user_id = $1
			ORDER BY is_pinned DESC, updated_at DESC
		`
		rows, err = m.db.Query(query, userID)
	} else {
		query = `
			SELECT id, COALESCE(user_id, 'a0000000-0000-0000-0000-000000000001'::uuid), title, diagram_type, COALESCE(project_mode, 'diagram'), COALESCE(metadata, '{}'::jsonb), is_pinned, current_nodes, current_edges, created_at, updated_at
			FROM projects
			ORDER BY is_pinned DESC, updated_at DESC
		`
		rows, err = m.db.Query(query)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to query projects: %w", err)
	}
	defer rows.Close()

	var projects []diagram.Project
	for rows.Next() {
		var p diagram.Project
		err := rows.Scan(
			&p.ID,
			&p.UserID,
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

func (m *PostgresProjectRepository) GetPaginated(limit, offset int) ([]diagram.ProjectSummary, int, error) {
	return m.GetPaginatedScoped("", limit, offset)
}

func (m *PostgresProjectRepository) GetPaginatedScoped(userID string, limit, offset int) ([]diagram.ProjectSummary, int, error) {
	if limit <= 0 {
		limit = 15
	}
	if limit > 100 {
		limit = 100
	}
	if offset < 0 {
		offset = 0
	}

	var totalCount int
	var countQuery string
	var selectQuery string
	var countArgs []interface{}
	var selectArgs []interface{}

	if userID != "" {
		countQuery = "SELECT COUNT(*) FROM projects WHERE user_id = $1"
		countArgs = []interface{}{userID}
		selectQuery = `
			SELECT id, COALESCE(user_id, 'a0000000-0000-0000-0000-000000000001'::uuid), title, diagram_type, COALESCE(project_mode, 'diagram'), is_pinned, created_at, updated_at
			FROM projects
			WHERE user_id = $1
			ORDER BY is_pinned DESC, updated_at DESC
			LIMIT $2 OFFSET $3
		`
		selectArgs = []interface{}{userID, limit, offset}
	} else {
		countQuery = "SELECT COUNT(*) FROM projects"
		selectQuery = `
			SELECT id, COALESCE(user_id, 'a0000000-0000-0000-0000-000000000001'::uuid), title, diagram_type, COALESCE(project_mode, 'diagram'), is_pinned, created_at, updated_at
			FROM projects
			ORDER BY is_pinned DESC, updated_at DESC
			LIMIT $1 OFFSET $2
		`
		selectArgs = []interface{}{limit, offset}
	}

	err := m.db.QueryRow(countQuery, countArgs...).Scan(&totalCount)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count projects: %w", err)
	}

	rows, err := m.db.Query(selectQuery, selectArgs...)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to query paginated projects: %w", err)
	}
	defer rows.Close()

	items := make([]diagram.ProjectSummary, 0, limit)
	for rows.Next() {
		var s diagram.ProjectSummary
		err := rows.Scan(
			&s.ID,
			&s.UserID,
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

func (m *PostgresProjectRepository) TogglePin(id string) (bool, error) {
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
