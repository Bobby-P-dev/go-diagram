package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/Bobby-P-dev/go-diagram.git/internal/modules/diagram"
)

type PostgresCloneRepository struct {
	db *sql.DB
}

func NewCloneRepository(db *sql.DB) *PostgresCloneRepository {
	return &PostgresCloneRepository{db: db}
}

func (g *PostgresCloneRepository) LoadProjectForClone(ctx context.Context, projectID string) (*diagram.Project, []diagram.ChatMessage, error) {
	queryProject := `
		SELECT id, title, diagram_type, COALESCE(project_mode, 'diagram'), COALESCE(metadata, '{}'::jsonb), is_pinned, current_nodes, current_edges, created_at, updated_at
		FROM projects
		WHERE id = $1
	`
	var p diagram.Project
	err := g.db.QueryRowContext(ctx, queryProject, projectID).Scan(
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
		return nil, nil, fmt.Errorf("associated project not found: %w", err)
	}

	queryMessages := `
		SELECT id, project_id, role, content, target_node_ids, created_at
		FROM chat_messages
		WHERE project_id = $1
		ORDER BY created_at ASC
	`
	rows, err := g.db.QueryContext(ctx, queryMessages, projectID)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to query chat messages: %w", err)
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
		if len(rawTarget) > 0 {
			msg.TargetNodeIDs = json.RawMessage(rawTarget)
		}
		messages = append(messages, msg)
	}
	if err := rows.Err(); err != nil {
		return nil, nil, fmt.Errorf("failed to read chat messages: %w", err)
	}

	return &p, messages, nil
}

func (g *PostgresCloneRepository) CloneProject(ctx context.Context, input diagram.CloneInput) (*diagram.Project, error) {
	tx, err := g.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	insertProjectQuery := `
		INSERT INTO projects (user_id, title, diagram_type, project_mode, metadata, current_nodes, current_edges, is_pinned, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, false, NOW(), NOW())
		RETURNING id, COALESCE(user_id, 'a0000000-0000-0000-0000-000000000001'::uuid), title, diagram_type, project_mode, metadata, is_pinned, current_nodes, current_edges, created_at, updated_at
	`
	var newProj diagram.Project
	err = tx.QueryRowContext(
		ctx,
		insertProjectQuery,
		input.UserID,
		input.Title,
		input.Original.DiagramType,
		input.Original.ProjectMode,
		input.Original.Metadata,
		input.Original.CurrentNodes,
		input.Original.CurrentEdges,
	).Scan(
		&newProj.ID,
		&newProj.UserID,
		&newProj.Title,
		&newProj.DiagramType,
		&newProj.ProjectMode,
		&newProj.Metadata,
		&newProj.IsPinned,
		&newProj.CurrentNodes,
		&newProj.CurrentEdges,
		&newProj.CreatedAt,
		&newProj.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to fork project: %w", err)
	}

	if len(input.Messages) > 0 {
		insertMsgQuery := `
			INSERT INTO chat_messages (project_id, role, content, target_node_ids, created_at)
			VALUES ($1, $2, $3, $4, $5)
		`
		for _, msg := range input.Messages {
			_, err = tx.ExecContext(ctx, insertMsgQuery, newProj.ID, msg.Role, msg.Content, msg.TargetNodeIDs, msg.CreatedAt)
			if err != nil {
				return nil, fmt.Errorf("failed to clone chat message: %w", err)
			}
		}
	}

	insertVerQuery := `
		INSERT INTO diagram_versions (project_id, version_number, change_summary, nodes, edges, created_at)
		VALUES ($1, 1, $2, $3, $4, NOW())
	`
	summary := fmt.Sprintf("Cloned from shared project: %s", input.Original.Title)
	if _, err = tx.ExecContext(ctx, insertVerQuery, newProj.ID, summary, newProj.CurrentNodes, newProj.CurrentEdges); err != nil {
		return nil, fmt.Errorf("failed to create fork snapshot: %w", err)
	}

	if err = tx.Commit(); err != nil {
		return nil, fmt.Errorf("failed to commit fork transaction: %w", err)
	}

	return &newProj, nil
}
