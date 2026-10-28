package repository

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/Bobby-P-dev/go-diagram.git/internal/modules/uidesign"
)

type PostgresJobBindingRepository struct {
	db *sql.DB
}

func NewPostgresJobBindingRepository(db *sql.DB) *PostgresJobBindingRepository {
	return &PostgresJobBindingRepository{db: db}
}

func (repository *PostgresJobBindingRepository) Create(ctx context.Context, binding uidesign.AsyncJobBinding) error {
	if strings.TrimSpace(binding.JobID) == "" || strings.TrimSpace(binding.ProjectID) == "" ||
		strings.TrimSpace(binding.UserID) == "" || strings.TrimSpace(binding.CorrelationID) == "" {
		return fmt.Errorf("job, project, owner, and correlation IDs are required")
	}
	_, err := repository.db.ExecContext(ctx, `
		INSERT INTO async_job_bindings (job_id, project_id, user_id, correlation_id)
		VALUES ($1, $2, $3, $4)
	`, binding.JobID, binding.ProjectID, binding.UserID, binding.CorrelationID)
	if err != nil {
		return fmt.Errorf("create async job binding: %w", err)
	}
	return nil
}

func (repository *PostgresJobBindingRepository) GetByID(ctx context.Context, jobID string) (*uidesign.AsyncJobBinding, error) {
	if strings.TrimSpace(jobID) == "" {
		return nil, sql.ErrNoRows
	}
	var binding uidesign.AsyncJobBinding
	err := repository.db.QueryRowContext(ctx, `
		SELECT job_id, project_id, user_id, correlation_id
		FROM async_job_bindings
		WHERE job_id = $1
	`, jobID).Scan(&binding.JobID, &binding.ProjectID, &binding.UserID, &binding.CorrelationID)
	if err != nil {
		return nil, fmt.Errorf("find async job binding: %w", err)
	}
	return &binding, nil
}
