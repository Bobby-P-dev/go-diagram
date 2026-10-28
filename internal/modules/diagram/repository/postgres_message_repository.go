package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/Bobby-P-dev/go-diagram.git/internal/modules/diagram"
)

type MessageRepository interface {
	AppendMessage(projectID, role, content string, targetNodeIDs json.RawMessage) (*diagram.ChatMessage, error)
	GetByProjectID(projectID string) ([]diagram.ChatMessage, error)
}

type PostgresMessageRepository struct {
	db *sql.DB
}

func NewMessageRepository(db *sql.DB) *PostgresMessageRepository {
	return &PostgresMessageRepository{db: db}
}

func (m *PostgresMessageRepository) AppendMessage(projectID, role, content string, targetNodeIDs json.RawMessage) (*diagram.ChatMessage, error) {
	return m.AppendMessageContext(context.Background(), projectID, role, content, targetNodeIDs)
}

func (m *PostgresMessageRepository) AppendMessageContext(ctx context.Context, projectID, role, content string, targetNodeIDs json.RawMessage) (*diagram.ChatMessage, error) {
	query := `
		INSERT INTO chat_messages (project_id, role, content, target_node_ids, created_at)
		VALUES ($1, $2, $3, $4, NOW())
		RETURNING id, project_id, role, content, target_node_ids, created_at
	`

	var targetParam interface{}
	if len(targetNodeIDs) > 0 && string(targetNodeIDs) != "null" {
		targetParam = targetNodeIDs
	} else {
		targetParam = nil
	}

	var msg diagram.ChatMessage
	var rawTarget []byte
	err := m.db.QueryRowContext(ctx, query, projectID, role, content, targetParam).Scan(
		&msg.ID,
		&msg.ProjectID,
		&msg.Role,
		&msg.Content,
		&rawTarget,
		&msg.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to append message: %w", err)
	}
	if rawTarget != nil {
		msg.TargetNodeIDs = rawTarget
	}

	return &msg, nil
}

func (m *PostgresMessageRepository) GetByProjectID(projectID string) ([]diagram.ChatMessage, error) {
	query := `
		SELECT id, project_id, role, content, target_node_ids, created_at
		FROM chat_messages
		WHERE project_id = $1
		ORDER BY created_at ASC
	`
	rows, err := m.db.Query(query, projectID)
	if err != nil {
		return nil, fmt.Errorf("failed to query messages: %w", err)
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
			return nil, fmt.Errorf("failed to scan message row: %w", err)
		}
		if rawTarget != nil {
			msg.TargetNodeIDs = rawTarget
		}
		messages = append(messages, msg)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating message rows: %w", err)
	}

	return messages, nil
}
