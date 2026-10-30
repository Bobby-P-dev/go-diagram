package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/Bobby-P-dev/go-diagram.git/internal/modules/generation"
)

func (repository *PostgresRunRepository) Commit(
	ctx context.Context,
	scope generation.Scope,
	id string,
	candidate *generation.ArtifactCandidate,
) (*generation.Run, error) {
	if err := candidate.Validate(); err != nil {
		return nil, err
	}
	var document, nodes, edges any
	metadata := "{}"
	if candidate.Graph == nil {
		document = candidate.DocumentContent
	} else {
		nodeJSON, err := json.Marshal(candidate.Graph.Nodes)
		if err != nil {
			return nil, err
		}
		edgeJSON, err := json.Marshal(candidate.Graph.Edges)
		if err != nil {
			return nil, err
		}
		metaJSON, err := json.Marshal(candidate.Graph.Metadata)
		if err != nil {
			return nil, err
		}
		nodes, edges, metadata = string(nodeJSON), string(edgeJSON), string(metaJSON)
	}
	tx, err := repository.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	var runID, projectID, threadID, userID, triggerID, outputType, outputTitle, targetArtifactID, targetExpectedRevID, status string
	var manifestBytes []byte
	var baseGraphJSON, mutationJSON []byte
	err = tx.QueryRowContext(ctx, `
		SELECT r.id, r.project_id, r.thread_id, r.user_id, r.trigger_message_id, r.output_type, r.output_title,
		       COALESCE(r.target_artifact_id::text, ''), COALESCE(r.target_expected_revision_id::text, ''),
		       r.status, r.frozen_manifest, r.base_graph, r.mutation
		FROM generation_runs r
		JOIN projects p ON p.id = r.project_id
		WHERE r.id = $1 AND r.project_id = $2 AND r.user_id = $3 AND p.user_id = $3
		FOR UPDATE OF r
	`, id, scope.ProjectID, scope.UserID).Scan(
		&runID, &projectID, &threadID, &userID, &triggerID, &outputType, &outputTitle,
		&targetArtifactID, &targetExpectedRevID, &status, &manifestBytes, &baseGraphJSON, &mutationJSON,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, generation.ErrNotFound
	}
	if err != nil {
		return nil, err
	}

	if status == "cancel_requested" || status == "canceled" {
		_, cancelErr := tx.ExecContext(ctx, `
			UPDATE generation_runs
			SET status = 'canceled', completed_at = COALESCE(completed_at, NOW()), updated_at = NOW()
			WHERE id = $1
		`, runID)
		if cancelErr != nil {
			return nil, cancelErr
		}
		if err := tx.Commit(); err != nil {
			return nil, err
		}
		return repository.Get(ctx, scope, runID)
	}
	if status != "running" {
		return nil, generation.ErrRunTerminal
	}
	if candidate.Type != outputType || (outputTitle != "" && candidate.Title != outputTitle) {
		return nil, generation.ErrInvalidInput
	}
	if len(mutationJSON) != 0 {
		var mutation *generation.DiagramMutation
		if json.Unmarshal(mutationJSON, &mutation) != nil || mutation == nil || candidate.Graph == nil || targetArtifactID == "" {
			return nil, generation.ErrInvalidInput
		}
		preserved, err := generation.PreserveDiagramMutation(baseGraphJSON, *candidate.Graph, mutation)
		if err != nil {
			return nil, err
		}
		nodeJSON, _ := json.Marshal(preserved.Nodes)
		edgeJSON, _ := json.Marshal(preserved.Edges)
		nodes, edges, metadata = string(nodeJSON), string(edgeJSON), string(preserved.Metadata)
	}

	var manifest generation.Manifest
	if err := json.Unmarshal(manifestBytes, &manifest); err != nil {
		return nil, fmt.Errorf("unmarshal frozen manifest: %w", err)
	}
	if manifest.SchemaVersion != 1 || manifest.Project.ID != projectID || manifest.ThreadID != threadID {
		return nil, generation.ErrInvalidInput
	}

	committedArtifactID := targetArtifactID
	var committedRevisionID string
	var versionNumber int

	title := candidate.Title
	if title == "" {
		title = outputTitle
	}

	if committedArtifactID == "" {
		var newArtifactID, newRevisionID string
		if err := tx.QueryRowContext(ctx, "SELECT uuid_generate_v4(), uuid_generate_v4()").Scan(&newArtifactID, &newRevisionID); err != nil {
			return nil, err
		}
		committedArtifactID = newArtifactID
		committedRevisionID = newRevisionID
		versionNumber = 1
		slug := fmt.Sprintf("%s-%s", outputType, committedArtifactID)

		_, err = tx.ExecContext(ctx, `
			INSERT INTO artifacts (id, project_id, type, title, slug, format, current_version_number, current_revision_id)
			VALUES ($1, $2, $3, $4, $5, $6, 1, $7)
		`, committedArtifactID, projectID, outputType, title, slug, candidate.Format, committedRevisionID)
		if err != nil {
			return nil, fmt.Errorf("create artifact: %w", err)
		}

	} else {
		var currentVer int
		var currentRevID, artType, artFormat string
		err = tx.QueryRowContext(ctx, `
			SELECT current_version_number, current_revision_id, type, format
			FROM artifacts
			WHERE id = $1 AND project_id = $2 AND archived_at IS NULL
			FOR UPDATE
		`, committedArtifactID, projectID).Scan(&currentVer, &currentRevID, &artType, &artFormat)
		if errors.Is(err, sql.ErrNoRows) {
			return nil, generation.ErrNotFound
		}
		if err != nil {
			return nil, fmt.Errorf("lock target artifact: %w", err)
		}
		if artFormat != candidate.Format || artType != outputType {
			return nil, generation.ErrInvalidInput
		}
		if currentRevID != targetExpectedRevID {
			return nil, generation.ErrRevisionConflict
		}

		if err := tx.QueryRowContext(ctx, "SELECT uuid_generate_v4()").Scan(&committedRevisionID); err != nil {
			return nil, err
		}
		versionNumber = currentVer + 1

		_, err = tx.ExecContext(ctx, `
			UPDATE artifacts
			SET current_version_number = $1, current_revision_id = $2, title = $3, updated_at = NOW()
			WHERE id = $4 AND project_id = $5
		`, versionNumber, committedRevisionID, title, committedArtifactID, projectID)
		if err != nil {
			return nil, fmt.Errorf("update artifact head: %w", err)
		}
	}
	_, err = tx.ExecContext(ctx, `
		INSERT INTO artifact_revisions (
			id, project_id, artifact_id, version_number, change_summary,
			content_format, schema_version, document_content, graph_nodes, graph_edges, metadata,
			trigger_message_id, author_id, author_type
		) VALUES ($1, $2, $3, $4, $5, $6, 1, $7, $8, $9, $10, $11, $12, 'ai')
	`, committedRevisionID, projectID, committedArtifactID, versionNumber, candidate.ChangeSummary,
		candidate.Format, document, nodes, edges, metadata, triggerID, userID)
	if err != nil {
		return nil, fmt.Errorf("insert generated revision: %w", err)
	}

	msgMeta, err := json.Marshal(map[string]interface{}{
		"generation_run_id":  runID,
		"trigger_message_id": triggerID,
		"artifact_id":        committedArtifactID,
		"revision_id":        committedRevisionID,
		"version_number":     versionNumber,
	})
	if err != nil {
		return nil, fmt.Errorf("marshal message metadata: %w", err)
	}

	var committedMessageID string
	err = tx.QueryRowContext(ctx, `
		INSERT INTO chat_messages_v2 (project_id, thread_id, role, content, metadata)
		VALUES ($1, $2, 'assistant', $3, $4)
		RETURNING id
	`, projectID, threadID, candidate.AssistantMessage, string(msgMeta)).Scan(&committedMessageID)
	if err != nil {
		return nil, fmt.Errorf("insert assistant message: %w", err)
	}

	_, err = tx.ExecContext(ctx, `
		UPDATE chat_threads
		SET active_artifact_id = $1, updated_at = NOW()
		WHERE id = $2 AND project_id = $3
	`, committedArtifactID, threadID, projectID)
	if err != nil {
		return nil, fmt.Errorf("update thread active artifact: %w", err)
	}

	for _, source := range manifest.Sources {
		_, err = tx.ExecContext(ctx, `
			INSERT INTO artifact_context_links (
				project_id, downstream_artifact_id, downstream_revision_number,
				upstream_artifact_id, upstream_revision_id, link_type
			) VALUES ($1, $2, $3, $4, $5, 'context_source')
		`, projectID, committedArtifactID, versionNumber, source.ArtifactID, source.RevisionID)
		if err != nil {
			return nil, fmt.Errorf("insert pinned context link: %w", err)
		}
	}

	var committedRun *generation.Run
	committedRun, err = scanRun(tx.QueryRowContext(ctx, `
		UPDATE generation_runs AS r
		SET status = 'succeeded',
		    committed_artifact_id = $1,
		    committed_revision_id = $2,
		    committed_message_id = $3,
		    completed_at = NOW(),
		    updated_at = NOW()
		WHERE r.id = $4
		RETURNING `+runColumns, committedArtifactID, committedRevisionID, committedMessageID, runID))
	if err != nil {
		return nil, fmt.Errorf("update run status: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	return committedRun, nil
}
