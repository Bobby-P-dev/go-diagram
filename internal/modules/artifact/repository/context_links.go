package repository

import (
	"context"
	"database/sql"
	"errors"

	"github.com/Bobby-P-dev/go-diagram.git/internal/modules/artifact"
)

type queryer interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
}

func insertContextLinks(ctx context.Context, tx *sql.Tx, projectID, artifactID string, version int, sources []artifact.RevisionRef) error {
	for _, source := range sources {
		var sourceID string
		err := tx.QueryRowContext(ctx, `SELECT r.id FROM artifact_revisions r JOIN artifacts a ON a.id = r.artifact_id AND a.project_id = r.project_id
			WHERE r.id = $1 AND r.artifact_id = $2 AND r.project_id = $3 AND a.archived_at IS NULL`, source.RevisionID, source.ArtifactID, projectID).Scan(&sourceID)
		if errors.Is(err, sql.ErrNoRows) {
			return artifact.ErrNotFound
		}
		if err != nil {
			return err
		}
		if source.ArtifactID == artifactID {
			return artifact.ErrInvalidInput
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO artifact_context_links(project_id, downstream_artifact_id, downstream_revision_number, upstream_artifact_id, upstream_revision_id, link_type)
			VALUES ($1,$2,$3,$4,$5,$6)`, projectID, artifactID, version, source.ArtifactID, source.RevisionID, source.LinkType); err != nil {
			return err
		}
	}
	return nil
}

func loadContextLinks(ctx context.Context, connection queryer, projectID, artifactID string, version int) ([]artifact.RevisionRef, error) {
	rows, err := connection.QueryContext(ctx, `SELECT upstream_artifact_id, upstream_revision_id, link_type FROM artifact_context_links
		WHERE project_id = $1 AND downstream_artifact_id = $2 AND downstream_revision_number = $3 ORDER BY upstream_artifact_id`, projectID, artifactID, version)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	sources := []artifact.RevisionRef{}
	for rows.Next() {
		var source artifact.RevisionRef
		if err := rows.Scan(&source.ArtifactID, &source.RevisionID, &source.LinkType); err != nil {
			return nil, err
		}
		sources = append(sources, source)
	}
	return sources, rows.Err()
}
