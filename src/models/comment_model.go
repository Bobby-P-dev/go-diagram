package models

import (
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/Bobby-P-dev/go-diagram.git/src/entities"
)

type CommentModelInterface interface {
	Create(projectID, nodeID, author, content string, posX, posY float64, metadata []byte) (*entities.DesignComment, error)
	GetByProjectID(projectID string) ([]entities.DesignComment, error)
	UpdateStatus(id, status string) error
	Delete(id string) error
}

type commentModel struct {
	db *sql.DB
}

func NewCommentModel(db *sql.DB) CommentModelInterface {
	return &commentModel{db: db}
}

func (m *commentModel) Create(projectID, nodeID, author, content string, posX, posY float64, metadata []byte) (*entities.DesignComment, error) {
	if len(metadata) == 0 {
		metadata = []byte("{}")
	}

	query := `
		INSERT INTO design_comments (project_id, node_id, author, content, status, position_x, position_y, metadata, created_at, updated_at)
		VALUES ($1, $2, $3, $4, 'open', $5, $6, $7, NOW(), NOW())
		RETURNING id, project_id, node_id, author, content, status, position_x, position_y, metadata, created_at, updated_at
	`

	var nodeParam *string
	if nodeID != "" {
		nodeParam = &nodeID
	}

	var c entities.DesignComment
	var metadataRaw []byte
	err := m.db.QueryRow(query, projectID, nodeParam, author, content, posX, posY, metadata).Scan(
		&c.ID,
		&c.ProjectID,
		&c.NodeID,
		&c.Author,
		&c.Content,
		&c.Status,
		&c.PosX,
		&c.PosY,
		&metadataRaw,
		&c.CreatedAt,
		&c.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create comment: %w", err)
	}
	c.Metadata = json.RawMessage(metadataRaw)

	return &c, nil
}

func (m *commentModel) GetByProjectID(projectID string) ([]entities.DesignComment, error) {
	query := `
		SELECT id, project_id, node_id, author, content, status, position_x, position_y, COALESCE(metadata, '{}'::jsonb), created_at, updated_at
		FROM design_comments
		WHERE project_id = $1
		ORDER BY created_at ASC
	`
	rows, err := m.db.Query(query, projectID)
	if err != nil {
		return nil, fmt.Errorf("failed to query design comments: %w", err)
	}
	defer rows.Close()

	comments := make([]entities.DesignComment, 0)
	for rows.Next() {
		var c entities.DesignComment
		var metadataRaw []byte
		err := rows.Scan(
			&c.ID,
			&c.ProjectID,
			&c.NodeID,
			&c.Author,
			&c.Content,
			&c.Status,
			&c.PosX,
			&c.PosY,
			&metadataRaw,
			&c.CreatedAt,
			&c.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan comment row: %w", err)
		}
		c.Metadata = json.RawMessage(metadataRaw)
		comments = append(comments, c)
	}

	return comments, nil
}

func (m *commentModel) UpdateStatus(id, status string) error {
	query := `
		UPDATE design_comments
		SET status = $1, updated_at = NOW()
		WHERE id = $2
	`
	_, err := m.db.Exec(query, status, id)
	if err != nil {
		return fmt.Errorf("failed to update comment status: %w", err)
	}
	return nil
}

func (m *commentModel) Delete(id string) error {
	query := `DELETE FROM design_comments WHERE id = $1`
	_, err := m.db.Exec(query, id)
	if err != nil {
		return fmt.Errorf("failed to delete comment: %w", err)
	}
	return nil
}
