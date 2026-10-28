package aiusage

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

type Entry struct {
	ID               string    `json:"id"`
	ProjectID        *string   `json:"project_id,omitempty"`
	Provider         string    `json:"provider"`
	Model            string    `json:"model"`
	PromptTokens     int       `json:"prompt_tokens"`
	CompletionTokens int       `json:"completion_tokens"`
	TotalTokens      int       `json:"total_tokens"`
	LatencyMS        int       `json:"latency_ms"`
	CreatedAt        time.Time `json:"created_at"`
}

type Recorder interface {
	LogAIUsage(ctx context.Context, entry Entry) error
}

type PostgresRepository struct {
	db *sql.DB
}

func NewPostgresRepository(db *sql.DB) *PostgresRepository {
	return &PostgresRepository{db: db}
}

func (repository *PostgresRepository) LogAIUsage(ctx context.Context, entry Entry) error {
	_, err := repository.db.ExecContext(ctx, `
		INSERT INTO ai_usage_logs (project_id, provider, model, prompt_tokens, completion_tokens, total_tokens, latency_ms, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`, entry.ProjectID, entry.Provider, entry.Model, entry.PromptTokens, entry.CompletionTokens, entry.TotalTokens, entry.LatencyMS, time.Now())
	if err != nil {
		return fmt.Errorf("failed to insert ai usage log: %w", err)
	}
	return nil
}
