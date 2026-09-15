package models

import (
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/Bobby-P-dev/go-diagram.git/src/entities"
)

type ExportModelInterface interface {
	SaveExport(projectID, exportType, fileName, content string, metadata json.RawMessage) (*entities.ProjectExport, error)
	GetByProjectID(projectID string) ([]entities.ProjectExport, error)
}

type exportModel struct {
	db *sql.DB
}

func NewExportModel(db *sql.DB) ExportModelInterface {
	return &exportModel{db: db}
}

func (m *exportModel) SaveExport(projectID, exportType, fileName, content string, metadata json.RawMessage) (*entities.ProjectExport, error) {
	if len(metadata) == 0 {
		metadata = json.RawMessage("{}")
	}

	query := `
		INSERT INTO project_exports (project_id, export_type, file_name, content, file_size_bytes, metadata, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, NOW())
		RETURNING id, project_id, export_type, file_name, content, file_size_bytes, metadata, created_at
	`

	var exp entities.ProjectExport
	var rawMeta []byte
	err := m.db.QueryRow(query, projectID, exportType, fileName, content, len(content), metadata).Scan(
		&exp.ID,
		&exp.ProjectID,
		&exp.ExportType,
		&exp.FileName,
		&exp.Content,
		&exp.FileSizeBytes,
		&rawMeta,
		&exp.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to save project export: %w", err)
	}
	exp.Metadata = rawMeta
	return &exp, nil
}

func (m *exportModel) GetByProjectID(projectID string) ([]entities.ProjectExport, error) {
	query := `
		SELECT id, project_id, export_type, file_name, content, file_size_bytes, metadata, created_at
		FROM project_exports
		WHERE project_id = $1
		ORDER BY created_at DESC
	`
	rows, err := m.db.Query(query, projectID)
	if err != nil {
		return nil, fmt.Errorf("failed to query project exports: %w", err)
	}
	defer rows.Close()

	exports := make([]entities.ProjectExport, 0)
	for rows.Next() {
		var exp entities.ProjectExport
		var rawMeta []byte
		err := rows.Scan(
			&exp.ID,
			&exp.ProjectID,
			&exp.ExportType,
			&exp.FileName,
			&exp.Content,
			&exp.FileSizeBytes,
			&rawMeta,
			&exp.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan export row: %w", err)
		}
		exp.Metadata = rawMeta
		exports = append(exports, exp)
	}

	return exports, nil
}
