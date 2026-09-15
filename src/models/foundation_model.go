package models

import (
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/Bobby-P-dev/go-diagram.git/src/entities"
)

type FoundationModelInterface interface {
	GetAll() ([]entities.DesignFoundation, error)
	Create(name, category, description string, tokens json.RawMessage) (*entities.DesignFoundation, error)
}

type foundationModel struct {
	db *sql.DB
}

func NewFoundationModel(db *sql.DB) FoundationModelInterface {
	return &foundationModel{db: db}
}

func (m *foundationModel) GetAll() ([]entities.DesignFoundation, error) {
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

	foundations := make([]entities.DesignFoundation, 0)
	for rows.Next() {
		var f entities.DesignFoundation
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

func (m *foundationModel) Create(name, category, description string, tokens json.RawMessage) (*entities.DesignFoundation, error) {
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

	var f entities.DesignFoundation
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
