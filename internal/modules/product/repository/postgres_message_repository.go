package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"github.com/Bobby-P-dev/go-diagram.git/internal/modules/product"
)

func (repository *PostgresRepository) PostUserMessage(
	ctx context.Context,
	userID, projectID, threadID string,
	input product.MessageInput,
	write product.WriteRequest,
) (*product.ChatMessage, error) {
	tx, err := repository.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	var dummy string
	err = tx.QueryRowContext(ctx, `
		SELECT t.id
		FROM chat_threads t
		JOIN projects p ON p.id = t.project_id
		WHERE t.id = $1 AND t.project_id = $2 AND p.user_id = $3
		FOR UPDATE OF t
	`, threadID, projectID, userID).Scan(&dummy)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, product.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("verify thread: %w", err)
	}

	var cached product.ChatMessage
	cachedHit, err := repository.checkOrRecordIdempotency(ctx, tx, userID, projectID, write, &cached)
	if err != nil {
		return nil, err
	}
	if cachedHit {
		return &cached, nil
	}

	if input.ClientMessageID != "" {
		var existing product.ChatMessage
		err = tx.QueryRowContext(ctx, `
			SELECT id, project_id, thread_id, role, content, client_message_id, COALESCE(target_element_ids, '[]'::jsonb), metadata, sequence_number, created_at
			FROM chat_messages_v2
			WHERE thread_id = $1 AND client_message_id = $2
		`, threadID, input.ClientMessageID).Scan(
			&existing.ID, &existing.ProjectID, &existing.ThreadID, &existing.Role, &existing.Content,
			&existing.ClientMessageID, &existing.TargetElementIDs, &existing.Metadata, &existing.SequenceNumber, &existing.CreatedAt,
		)
		if err == nil {
			if existing.Role != "user" || existing.Content != input.Content {
				return nil, product.ErrMessageConflict
			}
			if err := repository.completeIdempotency(ctx, tx, userID, projectID, write, 201, &existing); err != nil {
				return nil, err
			}
			if err := tx.Commit(); err != nil {
				return nil, fmt.Errorf("commit tx: %w", err)
			}
			return &existing, nil
		} else if !errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("check existing client message id: %w", err)
		}
	}

	var msg product.ChatMessage
	err = tx.QueryRowContext(ctx, `
		INSERT INTO chat_messages_v2 (project_id, thread_id, role, content, client_message_id, target_element_ids, metadata, created_at)
		VALUES ($1, $2, 'user', $3, NULLIF($4, ''), NULL, '{}'::jsonb, NOW())
		RETURNING id, project_id, thread_id, role, content, client_message_id, COALESCE(target_element_ids, '[]'::jsonb), metadata, sequence_number, created_at
	`, projectID, threadID, input.Content, input.ClientMessageID).Scan(
		&msg.ID, &msg.ProjectID, &msg.ThreadID, &msg.Role, &msg.Content,
		&msg.ClientMessageID, &msg.TargetElementIDs, &msg.Metadata, &msg.SequenceNumber, &msg.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("insert message: %w", err)
	}

	_, err = tx.ExecContext(ctx, "UPDATE chat_threads SET updated_at = NOW() WHERE id = $1", threadID)
	if err != nil {
		return nil, fmt.Errorf("touch thread updated_at: %w", err)
	}

	if err := repository.completeIdempotency(ctx, tx, userID, projectID, write, 201, &msg); err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit tx: %w", err)
	}
	return &msg, nil
}

func (repository *PostgresRepository) ListMessages(
	ctx context.Context,
	userID, projectID, threadID string,
	query product.PageQuery,
) (*product.PageResult[*product.ChatMessage], error) {
	var dummy string
	err := repository.db.QueryRowContext(ctx, `
		SELECT t.id
		FROM chat_threads t
		JOIN projects p ON p.id = t.project_id
		WHERE t.id = $1 AND t.project_id = $2 AND p.user_id = $3
	`, threadID, projectID, userID).Scan(&dummy)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, product.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("verify thread: %w", err)
	}

	limit := query.Limit
	if limit <= 0 || limit > 100 {
		limit = 20
	}

	var afterSeq int64
	if query.Cursor != "" {
		seq, err := decodeSeqCursor(userID+"/"+projectID+"/"+threadID+"/messages", query.Cursor)
		if err != nil {
			return nil, product.ErrInvalidInput
		}
		afterSeq = seq
	}

	rows, err := repository.db.QueryContext(ctx, `
		SELECT m.id, m.project_id, m.thread_id, m.role, m.content, m.client_message_id, COALESCE(m.target_element_ids, '[]'::jsonb), m.metadata, m.sequence_number, m.created_at
		FROM chat_messages_v2 m JOIN projects p ON p.id = m.project_id
		WHERE m.thread_id = $1 AND m.project_id = $2 AND p.user_id = $5
		  AND ($3 = 0 OR m.sequence_number > $3)
		ORDER BY m.sequence_number ASC
		LIMIT $4
	`, threadID, projectID, afterSeq, limit+1, userID)
	if err != nil {
		return nil, fmt.Errorf("query messages: %w", err)
	}
	defer rows.Close()

	var items []*product.ChatMessage
	for rows.Next() {
		var message product.ChatMessage
		if err := rows.Scan(
			&message.ID, &message.ProjectID, &message.ThreadID, &message.Role, &message.Content,
			&message.ClientMessageID, &message.TargetElementIDs, &message.Metadata, &message.SequenceNumber, &message.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan message: %w", err)
		}
		items = append(items, &message)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	var nextCursor *string
	if len(items) > limit {
		last := items[limit-1]
		cursor := encodeSeqCursor(userID+"/"+projectID+"/"+threadID+"/messages", last.SequenceNumber)
		nextCursor = &cursor
		items = items[:limit]
	}

	if items == nil {
		items = []*product.ChatMessage{}
	}

	return &product.PageResult[*product.ChatMessage]{
		Items:      items,
		NextCursor: nextCursor,
	}, nil
}
