package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/Bobby-P-dev/go-diagram.git/internal/modules/diagram"
)

type VersionRepository interface {
	CreateVersion(projectID string, versionNumber int, changeSummary string, nodes, edges json.RawMessage, triggerMessageID *string) (*diagram.DiagramVersion, error)
	GetVersionsByProjectID(projectID string) ([]diagram.DiagramVersion, error)
	GetVersionByID(versionID string) (*diagram.DiagramVersion, error)
	GetLatestVersionNumber(projectID string) (int, error)
}

type PostgresVersionRepository struct {
	db *sql.DB
}

func NewVersionRepository(db *sql.DB) *PostgresVersionRepository {
	return &PostgresVersionRepository{db: db}
}

func (m *PostgresVersionRepository) CreateVersion(projectID string, versionNumber int, changeSummary string, nodes, edges json.RawMessage, triggerMessageID *string) (*diagram.DiagramVersion, error) {
	return m.CreateVersionContext(context.Background(), projectID, versionNumber, changeSummary, nodes, edges, triggerMessageID)
}

func (m *PostgresVersionRepository) CreateVersionContext(ctx context.Context,
	projectID string,
	versionNumber int,
	changeSummary string,
	nodes, edges json.RawMessage,
	triggerMessageID *string,
) (*diagram.DiagramVersion, error) {
	query := `
		INSERT INTO diagram_versions (project_id, version_number, change_summary, nodes, edges, trigger_message_id, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, created_at
	`
	now := time.Now()
	var id string
	var createdAt time.Time

	err := m.db.QueryRowContext(ctx, query, projectID, versionNumber, changeSummary, nodes, edges, triggerMessageID, now).
		Scan(&id, &createdAt)
	if err != nil {
		return nil, fmt.Errorf("failed to create diagram version: %w", err)
	}

	return &diagram.DiagramVersion{
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

func (m *PostgresVersionRepository) GetVersionsByProjectID(projectID string) ([]diagram.DiagramVersion, error) {
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

	var versions []diagram.DiagramVersion
	for rows.Next() {
		var v diagram.DiagramVersion
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

func (m *PostgresVersionRepository) GetVersionByID(versionID string) (*diagram.DiagramVersion, error) {
	query := `
		SELECT id, project_id, version_number, change_summary, nodes, edges, trigger_message_id, created_at
		FROM diagram_versions
		WHERE id = $1
	`
	var v diagram.DiagramVersion
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

func (m *PostgresVersionRepository) GetLatestVersionNumber(projectID string) (int, error) {
	return m.GetLatestVersionNumberContext(context.Background(), projectID)
}

func (m *PostgresVersionRepository) GetLatestVersionNumberContext(ctx context.Context, projectID string) (int, error) {
	query := `
		SELECT COALESCE(MAX(version_number), 0)
		FROM diagram_versions
		WHERE project_id = $1
	`
	var maxVer int
	err := m.db.QueryRowContext(ctx, query, projectID).Scan(&maxVer)
	if err != nil {
		return 0, fmt.Errorf("failed to get latest version number: %w", err)
	}
	return maxVer, nil
}
