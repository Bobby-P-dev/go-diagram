package contextgateway

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/Bobby-P-dev/go-diagram.git/internal/modules/generation"
	"github.com/lib/pq"
)

type PostgresReader struct{ db *sql.DB }

var _ generation.ContextReader = (*PostgresReader)(nil)

func NewPostgresReader(db *sql.DB) *PostgresReader { return &PostgresReader{db: db} }

const sourceSelect = `
	SELECT a.id, r.id, r.version_number, a.title, a.type, a.format,
	       octet_length(payload.body::text),
	       CASE WHEN octet_length(payload.body::text) <= $3 THEN payload.body ELSE NULL END
	FROM artifacts a
	JOIN artifact_revisions r ON r.artifact_id = a.id AND r.project_id = a.project_id
	JOIN projects p ON p.id = a.project_id
	CROSS JOIN LATERAL (
		SELECT CASE a.format
			WHEN 'markdown' THEN jsonb_build_object('markdown', r.document_content)
			WHEN 'graph' THEN jsonb_build_object('nodes', r.graph_nodes, 'edges', r.graph_edges)
			ELSE r.frame_data
		END AS body
	) payload
	WHERE a.project_id = $1 AND p.user_id = $2 AND a.archived_at IS NULL
`

func (reader *PostgresReader) ReadSnapshot(ctx context.Context, scope generation.Scope, input generation.ResolveInput) (*generation.Snapshot, error) {
	tx, err := reader.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	if err != nil {
		return nil, fmt.Errorf("begin context snapshot: %w", err)
	}
	defer tx.Rollback()
	snapshot := &generation.Snapshot{}
	err = tx.QueryRowContext(ctx, `
		SELECT p.id, p.title, p.description FROM projects p
		JOIN chat_threads t ON t.project_id = p.id
		WHERE p.id = $1 AND p.user_id = $2 AND t.id = $3
	`, scope.ProjectID, scope.UserID, scope.ThreadID).Scan(&snapshot.Project.ID, &snapshot.Project.Title, &snapshot.Project.Description)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, generation.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("read context scope: %w", err)
	}
	for _, threadID := range append([]string{scope.ThreadID}, input.ThreadIDs...) {
		if threadID != scope.ThreadID {
			var found string
			err = tx.QueryRowContext(ctx, `SELECT id FROM chat_threads WHERE id = $1 AND project_id = $2`, threadID, scope.ProjectID).Scan(&found)
			if errors.Is(err, sql.ErrNoRows) {
				return nil, generation.ErrNotFound
			}
			if err != nil {
				return nil, fmt.Errorf("read context thread: %w", err)
			}
		}
		messages, limited, err := readMessages(ctx, tx, scope, threadID, input.MaxContentBytes)
		if err != nil {
			return nil, err
		}
		snapshot.Messages = append(snapshot.Messages, messages...)
		if limited {
			snapshot.HistoryLimitedThreadIDs = append(snapshot.HistoryLimitedThreadIDs, threadID)
		}
	}
	if input.Mode == "explicit" {
		for _, ref := range input.Sources {
			source, err := scanSource(tx.QueryRowContext(ctx, sourceSelect+`
				AND a.id = $4 AND r.id = COALESCE(NULLIF($5, '')::uuid, a.current_revision_id)
			`, scope.ProjectID, scope.UserID, input.MaxContentBytes, ref.ArtifactID, ref.RevisionID))
			if errors.Is(err, sql.ErrNoRows) {
				return nil, generation.ErrNotFound
			}
			if err != nil {
				return nil, fmt.Errorf("read context revision: %w", err)
			}
			snapshot.Sources = append(snapshot.Sources, source)
		}
	} else {
		rows, err := tx.QueryContext(ctx, sourceSelect+`
			AND r.id = a.current_revision_id AND NOT (a.id = ANY(COALESCE($4::uuid[], '{}'::uuid[])))
			ORDER BY a.updated_at DESC, a.id DESC LIMIT $5
		`, scope.ProjectID, scope.UserID, input.MaxContentBytes, pq.Array(input.ExcludedArtifactIDs), generation.CandidateLimit+1)
		if err != nil {
			return nil, fmt.Errorf("read context candidates: %w", err)
		}
		for rows.Next() {
			source, err := scanSource(rows)
			if err != nil {
				rows.Close()
				return nil, fmt.Errorf("scan context candidate: %w", err)
			}
			snapshot.Sources = append(snapshot.Sources, source)
		}
		readErr := rows.Err()
		closeErr := rows.Close()
		if readErr != nil {
			return nil, fmt.Errorf("read context candidates: %w", readErr)
		}
		if closeErr != nil {
			return nil, closeErr
		}
		if len(snapshot.Sources) > generation.CandidateLimit {
			snapshot.CandidateLimitReached = true
			snapshot.Sources = snapshot.Sources[:generation.CandidateLimit]
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit context snapshot: %w", err)
	}
	return snapshot, nil
}

type scanner interface{ Scan(...interface{}) error }

func scanSource(row scanner) (generation.Source, error) {
	var source generation.Source
	var raw []byte
	err := row.Scan(&source.ArtifactID, &source.RevisionID, &source.VersionNumber, &source.Title, &source.Type, &source.Format, &source.ContentBytes, &raw)
	if err == nil && len(raw) > 0 {
		source.Content = raw
	}
	return source, err
}

func readMessages(ctx context.Context, tx *sql.Tx, scope generation.Scope, threadID string, budget int) ([]generation.Message, bool, error) {
	rows, err := tx.QueryContext(ctx, `
		SELECT m.id, m.thread_id, m.role, m.sequence_number, octet_length(m.content),
		       CASE WHEN octet_length(m.content) <= $4 THEN m.content ELSE '' END
		FROM chat_messages_v2 m JOIN projects p ON p.id = m.project_id
		WHERE m.project_id = $1 AND p.user_id = $2 AND m.thread_id = $3
		  AND m.role IN ('user', 'assistant')
		ORDER BY m.sequence_number DESC LIMIT $5
	`, scope.ProjectID, scope.UserID, threadID, budget, generation.MessagesPerThread+1)
	if err != nil {
		return nil, false, fmt.Errorf("read context messages: %w", err)
	}
	defer rows.Close()
	messages := []generation.Message{}
	for rows.Next() {
		var message generation.Message
		if err := rows.Scan(&message.ID, &message.ThreadID, &message.Role, &message.SequenceNumber, &message.ContentBytes, &message.Content); err != nil {
			return nil, false, fmt.Errorf("scan context message: %w", err)
		}
		messages = append(messages, message)
	}
	if err := rows.Err(); err != nil {
		return nil, false, err
	}
	limited := len(messages) > generation.MessagesPerThread
	if limited {
		messages = messages[:generation.MessagesPerThread]
	}
	return messages, limited, nil
}
