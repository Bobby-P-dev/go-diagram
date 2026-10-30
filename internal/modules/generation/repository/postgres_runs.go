package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"unicode/utf8"

	"github.com/Bobby-P-dev/go-diagram.git/internal/modules/generation"
)

type PostgresRunRepository struct{ db *sql.DB }

func NewPostgresRunRepository(db *sql.DB) *PostgresRunRepository {
	return &PostgresRunRepository{db: db}
}

var _ generation.RunRepository = (*PostgresRunRepository)(nil)

const runColumns = `r.id,r.project_id,r.thread_id,r.user_id,r.trigger_message_id,r.output_type,r.output_title,
 COALESCE(r.target_artifact_id::text,''),COALESCE(r.target_expected_revision_id::text,''),r.frozen_manifest,
 r.prompt,r.base_document,r.base_graph,r.mutation,r.status,COALESCE(r.error_code,''),COALESCE(r.error_message,''),
 COALESCE(r.committed_artifact_id::text,''),COALESCE(r.committed_revision_id::text,''),
 COALESCE(r.committed_message_id::text,''),r.created_at,r.updated_at,r.completed_at`

type scanner interface{ Scan(...any) error }

func scanRun(row scanner) (*generation.Run, error) {
	var run generation.Run
	var manifest []byte
	var baseGraph []byte
	var mutation []byte
	err := row.Scan(&run.ID, &run.ProjectID, &run.ThreadID, &run.UserID, &run.TriggerMessageID, &run.OutputType, &run.OutputTitle,
		&run.TargetArtifactID, &run.TargetExpectedRevisionID, &manifest, &run.Prompt, &run.BaseDocument, &baseGraph, &mutation, &run.Status,
		&run.ErrorCode, &run.ErrorMessage, &run.CommittedArtifactID, &run.CommittedRevisionID, &run.CommittedMessageID,
		&run.CreatedAt, &run.UpdatedAt, &run.CompletedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, generation.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if err = json.Unmarshal(manifest, &run.Manifest); err != nil {
		return nil, err
	}
	if len(baseGraph) != 0 {
		run.BaseGraphRaw = append(json.RawMessage(nil), baseGraph...)
		if err := json.Unmarshal(baseGraph, &run.BaseGraph); err != nil {
			return nil, err
		}
	}
	if len(mutation) != 0 {
		if err := json.Unmarshal(mutation, &run.Mutation); err != nil {
			return nil, err
		}
	}
	return &run, nil
}

func (repository *PostgresRunRepository) Get(ctx context.Context, scope generation.Scope, id string) (*generation.Run, error) {
	return scanRun(repository.db.QueryRowContext(ctx, `SELECT `+runColumns+` FROM generation_runs r
 JOIN projects p ON p.id=r.project_id WHERE r.id=$1 AND r.project_id=$2 AND p.user_id=$3 AND r.user_id=$3`, id, scope.ProjectID, scope.UserID))
}

func (repository *PostgresRunRepository) FindAccepted(ctx context.Context, scope generation.Scope, write generation.RunWrite) (*generation.Run, error) {
	var id, hash, threadID string
	err := repository.db.QueryRowContext(ctx, `SELECT r.id,r.request_hash,r.thread_id FROM generation_runs r
 JOIN projects p ON p.id=r.project_id WHERE r.project_id=$1 AND r.user_id=$2 AND p.user_id=$2 AND r.idempotency_key=$3`,
		scope.ProjectID, scope.UserID, write.Key).Scan(&id, &hash, &threadID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if hash != write.RequestHash || threadID != scope.ThreadID {
		return nil, generation.ErrIdempotencyConflict
	}
	return repository.Get(ctx, scope, id)
}

func (repository *PostgresRunRepository) Create(ctx context.Context, scope generation.Scope, input generation.StartInput, manifest *generation.Manifest, write generation.RunWrite) (*generation.Run, error) {
	if input.Mutation != nil && (input.Mutation.Validate() != nil || input.TargetArtifactID == "" || input.TargetExpectedRevisionID == "" || (input.OutputType != "erd" && input.OutputType != "flowchart" && input.OutputType != "sequence")) {
		return nil, generation.ErrInvalidInput
	}
	tx, err := repository.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, scope.UserID+":"+scope.ProjectID+":"+write.Key); err != nil {
		return nil, err
	}
	var prompt string
	err = tx.QueryRowContext(ctx, `SELECT m.content FROM chat_messages_v2 m JOIN projects p ON p.id=m.project_id
 WHERE m.id=$1 AND m.project_id=$2 AND m.thread_id=$3 AND m.role='user' AND p.user_id=$4 FOR SHARE OF p,m`,
		input.TriggerMessageID, scope.ProjectID, scope.ThreadID, scope.UserID).Scan(&prompt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, generation.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(prompt) == "" || len(prompt) > 16*1024 || !utf8.ValidString(prompt) {
		return nil, generation.ErrInvalidInput
	}
	var existingID, hash, threadID string
	err = tx.QueryRowContext(ctx, `SELECT id,request_hash,thread_id FROM generation_runs WHERE project_id=$1 AND user_id=$2 AND idempotency_key=$3`, scope.ProjectID, scope.UserID, write.Key).Scan(&existingID, &hash, &threadID)
	if err == nil {
		if hash != write.RequestHash || threadID != scope.ThreadID {
			return nil, generation.ErrIdempotencyConflict
		}
		return scanRun(tx.QueryRowContext(ctx, `SELECT `+runColumns+` FROM generation_runs r WHERE r.id=$1`, existingID))
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}
	var base *string
	var baseGraph any
	if input.TargetArtifactID != "" {
		format, err := generation.OutputFormat(input.OutputType)
		if err != nil {
			return nil, err
		}
		var revisionID, kind, title string
		var document *string
		var graphNodes, graphEdges, metadata []byte
		err = tx.QueryRowContext(ctx, `SELECT a.current_revision_id,a.type,v.document_content,v.graph_nodes,v.graph_edges,v.metadata,a.title FROM artifacts a
 JOIN artifact_revisions v ON v.id=a.current_revision_id AND v.artifact_id=a.id AND v.project_id=a.project_id
 WHERE a.id=$1 AND a.project_id=$2 AND a.format=$3 AND a.archived_at IS NULL FOR SHARE OF a`, input.TargetArtifactID, scope.ProjectID, format).Scan(&revisionID, &kind, &document, &graphNodes, &graphEdges, &metadata, &title)
		if errors.Is(err, sql.ErrNoRows) {
			return nil, generation.ErrNotFound
		}
		if err != nil {
			return nil, err
		}
		if kind != input.OutputType {
			return nil, generation.ErrInvalidInput
		}
		if revisionID != input.TargetExpectedRevisionID {
			return nil, generation.ErrRevisionConflict
		}
		if format == "markdown" {
			if document == nil || strings.TrimSpace(*document) == "" {
				return nil, generation.ErrInvalidInput
			}
			base = document
		} else {
			var graph generation.DiagramGraph
			if err := json.Unmarshal(graphNodes, &graph.Nodes); err != nil {
				return nil, generation.ErrInvalidInput
			}
			if err := json.Unmarshal(graphEdges, &graph.Edges); err != nil {
				return nil, generation.ErrInvalidInput
			}
			if err := json.Unmarshal(metadata, &graph.Metadata); err != nil {
				return nil, generation.ErrInvalidInput
			}
			graph.Metadata.DiagramType = kind
			if err := generation.ValidateDiagramGraph(graph, kind); err != nil {
				return nil, generation.ErrInvalidInput
			}
			if err := input.Mutation.ValidateBase(&graph); err != nil {
				return nil, err
			}
			encoded, err := json.Marshal(graph)
			if err != nil {
				return nil, err
			}
			if input.Mutation != nil {
				if input.OutputTitle != title {
					return nil, generation.ErrInvalidInput
				}
				var meta map[string]json.RawMessage
				if json.Unmarshal(metadata, &meta) != nil || meta == nil {
					return nil, generation.ErrInvalidInput
				}
				meta["diagram_type"], _ = json.Marshal(kind)
				metaJSON, _ := json.Marshal(meta)
				encoded, err = json.Marshal(map[string]json.RawMessage{"nodes": graphNodes, "edges": graphEdges, "metadata": metaJSON})
				if err != nil || len(encoded) > generation.MaxDiagramBytes {
					return nil, generation.ErrInvalidInput
				}
			}
			baseGraph = string(encoded)
		}
	}
	data, err := json.Marshal(manifest)
	if err != nil {
		return nil, err
	}
	var mutationJSON any
	if input.Mutation != nil {
		encoded, err := json.Marshal(input.Mutation)
		if err != nil {
			return nil, err
		}
		mutationJSON = string(encoded)
	}
	run, err := scanRun(tx.QueryRowContext(ctx, `INSERT INTO generation_runs AS r(project_id,thread_id,user_id,trigger_message_id,output_type,output_title,
 target_artifact_id,target_expected_revision_id,context_mode,frozen_manifest,prompt,base_document,base_graph,idempotency_key,request_hash,mutation)
 VALUES ($1,$2,$3,$4,$5,$6,NULLIF($7,'')::uuid,NULLIF($8,'')::uuid,$9,$10,$11,$12,$13,$14,$15,$16) RETURNING `+runColumns,
		scope.ProjectID, scope.ThreadID, scope.UserID, input.TriggerMessageID, input.OutputType, input.OutputTitle, input.TargetArtifactID, input.TargetExpectedRevisionID,
		manifest.Mode, string(data), prompt, base, baseGraph, write.Key, write.RequestHash, mutationJSON))
	if err != nil {
		return nil, err
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return run, nil
}

func (repository *PostgresRunRepository) Claim(ctx context.Context, scope generation.Scope, id string) (*generation.Run, bool, error) {
	run, err := scanRun(repository.db.QueryRowContext(ctx, `UPDATE generation_runs r SET status='running',updated_at=NOW(),lease_expires_at=NOW()+INTERVAL '6 minutes'
 FROM projects p WHERE r.id=$1 AND r.project_id=$2 AND r.user_id=$3 AND p.id=r.project_id AND p.user_id=$3 AND r.status='queued' RETURNING `+runColumns, id, scope.ProjectID, scope.UserID))
	if errors.Is(err, generation.ErrNotFound) {
		return nil, false, nil
	}
	return run, err == nil, err
}

func (repository *PostgresRunRepository) Cancel(ctx context.Context, scope generation.Scope, id string) (*generation.Run, error) {
	run, err := scanRun(repository.db.QueryRowContext(ctx, `UPDATE generation_runs r SET
 status=CASE WHEN r.status='queued' THEN 'canceled' WHEN r.status='running' THEN 'cancel_requested' ELSE r.status END,
 completed_at=CASE WHEN r.status='queued' THEN NOW() ELSE r.completed_at END,updated_at=NOW()
 FROM projects p WHERE r.id=$1 AND r.project_id=$2 AND r.user_id=$3 AND p.id=r.project_id AND p.user_id=$3
 AND r.status IN ('queued','running','cancel_requested','canceled') RETURNING `+runColumns, id, scope.ProjectID, scope.UserID))
	if !errors.Is(err, generation.ErrNotFound) {
		return run, err
	}
	run, err = repository.Get(ctx, scope, id)
	if err != nil {
		return nil, err
	}
	return run, generation.ErrRunTerminal
}

func (repository *PostgresRunRepository) Finish(ctx context.Context, scope generation.Scope, id, status, code, message string) error {
	_, err := repository.db.ExecContext(ctx, `UPDATE generation_runs r SET
 status=CASE WHEN status='cancel_requested' THEN 'canceled' ELSE $4 END,
 error_code=CASE WHEN status='cancel_requested' THEN NULL ELSE NULLIF($5,'') END,
 error_message=CASE WHEN status='cancel_requested' THEN NULL ELSE NULLIF($6,'') END,
 completed_at=NOW(),updated_at=NOW() FROM projects p
 WHERE r.id=$1 AND r.project_id=$2 AND r.user_id=$3 AND p.id=r.project_id AND p.user_id=$3
 AND status IN ('queued','running','cancel_requested')`, id, scope.ProjectID, scope.UserID, status, code, message)
	return err
}

func (repository *PostgresRunRepository) RecoverInterrupted(ctx context.Context) error {
	_, err := repository.db.ExecContext(ctx, `UPDATE generation_runs SET
 status=CASE WHEN status='cancel_requested' THEN 'canceled' ELSE 'failed' END,
 error_code=CASE WHEN status='cancel_requested' THEN NULL ELSE 'WORKER_INTERRUPTED' END,
 error_message=CASE WHEN status='cancel_requested' THEN NULL ELSE 'Generation worker lease expired; start a new run' END,
 completed_at=NOW(),updated_at=NOW() WHERE status IN ('running','cancel_requested') AND lease_expires_at < NOW()`)
	return err
}

func (repository *PostgresRunRepository) Queued(ctx context.Context) ([]generation.Run, error) {
	rows, err := repository.db.QueryContext(ctx, `SELECT r.id,r.project_id,r.thread_id,r.user_id FROM generation_runs r JOIN projects p ON p.id=r.project_id AND p.user_id=r.user_id
 WHERE r.status='queued' ORDER BY r.created_at,r.id LIMIT 8`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	runs := []generation.Run{}
	for rows.Next() {
		var run generation.Run
		if err := rows.Scan(&run.ID, &run.ProjectID, &run.ThreadID, &run.UserID); err != nil {
			return nil, err
		}
		runs = append(runs, run)
	}
	return runs, rows.Err()
}
