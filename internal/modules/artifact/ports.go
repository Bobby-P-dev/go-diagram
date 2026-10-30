package artifact

import (
	"context"
	"encoding/json"
	"errors"
)

var (
	ErrNotFound            = errors.New("resource not found")
	ErrUnauthenticated     = errors.New("authentication required")
	ErrInvalidInput        = errors.New("invalid request")
	ErrRevisionConflict    = errors.New("artifact changed; reload before applying this revision")
	ErrIdempotencyConflict = errors.New("idempotency key reused with different input")
	ErrIdempotencyInFlight = errors.New("request already in progress")
	ErrSlugConflict        = errors.New("artifact slug already in use in this project")
	ErrContentInvalid      = errors.New("invalid artifact content")
)

type PageQuery struct {
	Limit      int
	Cursor     string
	TypeFilter string
}

type PageResult[T any] struct {
	Items      []T     `json:"items"`
	NextCursor *string `json:"next_cursor"`
}

type WriteRequest struct {
	Key         string
	RequestHash string
	Resource    string
}

type CreateArtifactInput struct {
	SchemaVersion    int             `json:"schema_version"`
	TriggerMessageID *string         `json:"trigger_message_id,omitempty"`
	Sources          []RevisionRef   `json:"sources"`
	Type             string          `json:"type"`
	Title            string          `json:"title"`
	Slug             string          `json:"slug"`
	DocumentContent  *string         `json:"document_content,omitempty"`
	GraphNodes       json.RawMessage `json:"graph_nodes,omitempty"`
	GraphEdges       json.RawMessage `json:"graph_edges,omitempty"`
	FrameData        json.RawMessage `json:"frame_data,omitempty"`
	ChangeSummary    string          `json:"change_summary"`
}

type AppendRevisionInput struct {
	SchemaVersion      int             `json:"schema_version"`
	TriggerMessageID   *string         `json:"trigger_message_id,omitempty"`
	Sources            []RevisionRef   `json:"sources"`
	ExpectedRevisionID string          `json:"expected_revision_id"`
	ChangeSummary      string          `json:"change_summary"`
	DocumentContent    *string         `json:"document_content,omitempty"`
	GraphNodes         json.RawMessage `json:"graph_nodes,omitempty"`
	GraphEdges         json.RawMessage `json:"graph_edges,omitempty"`
	FrameData          json.RawMessage `json:"frame_data,omitempty"`
}

type RestoreRevisionInput struct {
	TargetRevisionID   string `json:"target_revision_id"`
	ExpectedRevisionID string `json:"expected_revision_id"`
	ChangeSummary      string `json:"change_summary"`
}

type Repository interface {
	CreateArtifact(ctx context.Context, userID, projectID string, input CreateArtifactInput, write WriteRequest) (*ArtifactDetail, error)
	GetArtifact(ctx context.Context, userID, projectID, artifactID string) (*ArtifactDetail, error)
	ListArtifacts(ctx context.Context, userID, projectID string, query PageQuery) (*PageResult[*Artifact], error)
	GetRevision(ctx context.Context, userID, projectID, artifactID, revisionID string) (*ArtifactRevision, error)
	ListRevisions(ctx context.Context, userID, projectID, artifactID string, query PageQuery) (*PageResult[*RevisionSummary], error)
	AppendRevision(ctx context.Context, userID, projectID, artifactID string, input AppendRevisionInput, write WriteRequest) (*ArtifactRevision, error)
	RestoreRevision(ctx context.Context, userID, projectID, artifactID string, input RestoreRevisionInput, write WriteRequest) (*ArtifactRevision, error)
}
