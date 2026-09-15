package models

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/Bobby-P-dev/go-diagram.git/src/entities"
)

type LogModelInterface interface {
	LogAIUsage(log entities.AIUsageLog) error
}

type LogModel struct {
	db *sql.DB
}

func NewLogModel(db *sql.DB) *LogModel {
	return &LogModel{db: db}
}

func (m *LogModel) LogAIUsage(entry entities.AIUsageLog) error {
	query := `
		INSERT INTO ai_usage_logs (project_id, provider, model, prompt_tokens, completion_tokens, total_tokens, latency_ms, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`
	now := time.Now()
	_, err := m.db.Exec(
		query,
		entry.ProjectID,
		entry.Provider,
		entry.Model,
		entry.PromptTokens,
		entry.CompletionTokens,
		entry.TotalTokens,
		entry.LatencyMS,
		now,
	)
	if err != nil {
		return fmt.Errorf("failed to insert ai usage log: %w", err)
	}
	return nil
}
