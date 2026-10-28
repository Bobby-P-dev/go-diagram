package repository

import (
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/Bobby-P-dev/go-diagram.git/internal/modules/workspace"
)

type FoundationRepository interface {
	GetAll() ([]workspace.DesignFoundation, error)
	Create(name, category, description string, tokens json.RawMessage) (*workspace.DesignFoundation, error)
}

type foundationRepository struct {
	db *sql.DB
}

func NewFoundationRepository(db *sql.DB) FoundationRepository {
	return &foundationRepository{db: db}
}

func (m *foundationRepository) GetAll() ([]workspace.DesignFoundation, error) {
	query := `
		SELECT id, name, category, COALESCE(description, ''), tokens, is_default, created_at, updated_at
		FROM design_foundations
		ORDER BY is_default DESC, name ASC
	`
	rows, err := m.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to query design foundations: %w", err)
	}
	defer rows.Close()

	foundations := make([]workspace.DesignFoundation, 0)
	for rows.Next() {
		var f workspace.DesignFoundation
		var rawTokens []byte
		err := rows.Scan(
			&f.ID,
			&f.Name,
			&f.Category,
			&f.Description,
			&rawTokens,
			&f.IsDefault,
			&f.CreatedAt,
			&f.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan foundation row: %w", err)
		}
		f.Tokens = rawTokens
		foundations = append(foundations, f)
	}

	return foundations, nil
}

func (m *foundationRepository) Create(name, category, description string, tokens json.RawMessage) (*workspace.DesignFoundation, error) {
	if len(tokens) == 0 {
		tokens = json.RawMessage("{}")
	}
	if category == "" {
		category = "custom"
	}

	query := `
		INSERT INTO design_foundations (name, category, description, tokens, is_default, created_at, updated_at)
		VALUES ($1, $2, $3, $4, false, NOW(), NOW())
		RETURNING id, name, category, COALESCE(description, ''), tokens, is_default, created_at, updated_at
	`

	var f workspace.DesignFoundation
	var rawTokens []byte
	err := m.db.QueryRow(query, name, category, description, tokens).Scan(
		&f.ID,
		&f.Name,
		&f.Category,
		&f.Description,
		&rawTokens,
		&f.IsDefault,
		&f.CreatedAt,
		&f.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create foundation: %w", err)
	}
	f.Tokens = rawTokens
	return &f, nil
}
