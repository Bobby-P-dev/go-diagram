package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/Bobby-P-dev/go-diagram.git/internal/modules/artifact"
)

func (repository *PostgresRepository) checkOrRecordIdempotency(
	ctx context.Context,
	tx *sql.Tx,
	userID, projectID string,
	write artifact.WriteRequest,
	target interface{},
) (bool, error) {
	if write.Key == "" {
		return false, artifact.ErrInvalidInput
	}
	lockID := userID + ":" + write.Resource + ":" + write.Key
	if _, err := tx.ExecContext(ctx, "SELECT pg_advisory_xact_lock(hashtextextended($1, 0))", lockID); err != nil {
		return false, fmt.Errorf("lock idempotency key: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM idempotency_keys WHERE user_id = $1 AND resource = $2 AND key = $3 AND expires_at <= NOW()`, userID, write.Resource, write.Key); err != nil {
		return false, fmt.Errorf("expire idempotency key: %w", err)
	}
	var existingHash string
	var state string
	var statusCode sql.NullInt64
	var responseData []byte

	err := tx.QueryRowContext(ctx, `
		SELECT request_hash, state, status_code, response
		FROM idempotency_keys
		WHERE user_id = $1 AND resource = $2 AND key = $3
		FOR UPDATE
	`, userID, write.Resource, write.Key).Scan(&existingHash, &state, &statusCode, &responseData)

	if err == nil {
		if existingHash != write.RequestHash {
			return false, artifact.ErrIdempotencyConflict
		}
		if state == "processing" {
			return false, artifact.ErrIdempotencyInFlight
		}
		if state == "completed" && len(responseData) > 0 {
			if err := json.Unmarshal(responseData, target); err != nil {
				return false, fmt.Errorf("unmarshal cached response: %w", err)
			}
			return true, nil
		}
	} else if !errors.Is(err, sql.ErrNoRows) {
		return false, fmt.Errorf("check idempotency key: %w", err)
	}

	var projIDVal *string
	if projectID != "" {
		projIDVal = &projectID
	}
	_, err = tx.ExecContext(ctx, `
		INSERT INTO idempotency_keys (key, user_id, project_id, resource, request_hash, state, expires_at)
		VALUES ($1, $2, $3, $4, $5, 'processing', NOW() + INTERVAL '24 hours')
	`, write.Key, userID, projIDVal, write.Resource, write.RequestHash)
	if err != nil {
		return false, fmt.Errorf("insert idempotency key: %w", err)
	}
	return false, nil
}

func (repository *PostgresRepository) completeIdempotency(
	ctx context.Context,
	tx *sql.Tx,
	userID, projectID string,
	write artifact.WriteRequest,
	statusCode int,
	payload interface{},
) error {
	if write.Key == "" {
		return nil
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal idempotency payload: %w", err)
	}
	var projIDVal *string
	if projectID != "" {
		projIDVal = &projectID
	}
	_, err = tx.ExecContext(ctx, `
		UPDATE idempotency_keys
		SET state = 'completed', status_code = $1, response = $2, project_id = COALESCE(project_id, $3)
		WHERE user_id = $4 AND resource = $5 AND key = $6
	`, statusCode, string(data), projIDVal, userID, write.Resource, write.Key)
	if err != nil {
		return fmt.Errorf("complete idempotency: %w", err)
	}
	return nil
}
