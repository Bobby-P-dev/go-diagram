package models

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/Bobby-P-dev/go-diagram.git/src/entities"
)

type VersionModelInterface interface {
	CreateVersion(projectID string, versionNumber int, changeSummary string, nodes, edges json.RawMessage, triggerMessageID *string) (*entities.DiagramVersion, error)
	GetVersionsByProjectID(projectID string) ([]entities.DiagramVersion, error)
	GetVersionByID(versionID string) (*entities.DiagramVersion, error)
	GetLatestVersionNumber(projectID string) (int, error)
}

type VersionModel struct {
	db *sql.DB
}

func NewVersionModel(db *sql.DB) *VersionModel {
	return &VersionModel{db: db}
}

func (m *VersionModel) CreateVersion(
	projectID string,
	versionNumber int,
	changeSummary string,
	nodes, edges json.RawMessage,
	triggerMessageID *string,
) (*entities.DiagramVersion, error) {
	query := `
		INSERT INTO diagram_versions (project_id, version_number, change_summary, nodes, edges, trigger_message_id, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, created_at
	`
	now := time.Now()
	var id string
	var createdAt time.Time

	err := m.db.QueryRow(query, projectID, versionNumber, changeSummary, nodes, edges, triggerMessageID, now).
		Scan(&id, &createdAt)
	if err != nil {
		return nil, fmt.Errorf("failed to create diagram version: %w", err)
	}

	return &entities.DiagramVersion{
		ID:               id,
		ProjectID:        projectID,
		VersionNumber:    versionNumber,
		ChangeSummary:    changeSummary,
		Nodes:            nodes,
		Edges:            edges,
		TriggerMessageID: triggerMessageID,
		CreatedAt:        createdAt,
	}, nil
}

func (m *VersionModel) GetVersionsByProjectID(projectID string) ([]entities.DiagramVersion, error) {
	query := `
		SELECT id, project_id, version_number, change_summary, nodes, edges, trigger_message_id, created_at
		FROM diagram_versions
		WHERE project_id = $1
		ORDER BY version_number DESC
	`
	rows, err := m.db.Query(query, projectID)
	if err != nil {
		return nil, fmt.Errorf("failed to query diagram versions: %w", err)
	}
	defer rows.Close()

	var versions []entities.DiagramVersion
	for rows.Next() {
		var v entities.DiagramVersion
		var triggerMsgID sql.NullString
		err := rows.Scan(
			&v.ID,
			&v.ProjectID,
			&v.VersionNumber,
			&v.ChangeSummary,
			&v.Nodes,
			&v.Edges,
			&triggerMsgID,
			&v.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan diagram version: %w", err)
		}
		if triggerMsgID.Valid {
			v.TriggerMessageID = &triggerMsgID.String
		}
		versions = append(versions, v)
	}

	return versions, nil
}

func (m *VersionModel) GetVersionByID(versionID string) (*entities.DiagramVersion, error) {
	query := `
		SELECT id, project_id, version_number, change_summary, nodes, edges, trigger_message_id, created_at
		FROM diagram_versions
		WHERE id = $1
	`
	var v entities.DiagramVersion
	var triggerMsgID sql.NullString
	err := m.db.QueryRow(query, versionID).Scan(
		&v.ID,
		&v.ProjectID,
		&v.VersionNumber,
		&v.ChangeSummary,
		&v.Nodes,
		&v.Edges,
		&triggerMsgID,
		&v.CreatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("version not found")
		}
		return nil, fmt.Errorf("failed to get diagram version: %w", err)
	}
	if triggerMsgID.Valid {
		v.TriggerMessageID = &triggerMsgID.String
	}

	return &v, nil
}

func (m *VersionModel) GetLatestVersionNumber(projectID string) (int, error) {
	query := `
		SELECT COALESCE(MAX(version_number), 0)
		FROM diagram_versions
		WHERE project_id = $1
	`
	var maxVer int
	err := m.db.QueryRow(query, projectID).Scan(&maxVer)
	if err != nil {
		return 0, fmt.Errorf("failed to get latest version number: %w", err)
	}
	return maxVer, nil
}
