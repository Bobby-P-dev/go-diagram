package artifact

import (
	"encoding/json"
	"time"
)

type Artifact struct {
	ID                   string          `json:"id"`
	ProjectID            string          `json:"project_id"`
	Type                 string          `json:"type"`
	Title                string          `json:"title"`
	Slug                 string          `json:"slug"`
	Format               string          `json:"format"`
	CurrentVersionNumber int             `json:"current_version_number"`
	CurrentRevisionID    string          `json:"current_revision_id"`
	Metadata             json.RawMessage `json:"metadata"`
	ArchivedAt           *time.Time      `json:"archived_at,omitempty"`
	CreatedAt            time.Time       `json:"created_at"`
	UpdatedAt            time.Time       `json:"updated_at"`
}

type ArtifactRevision struct {
	ID               string          `json:"id"`
	ProjectID        string          `json:"project_id"`
	ArtifactID       string          `json:"artifact_id"`
	VersionNumber    int             `json:"version_number"`
	ChangeSummary    string          `json:"change_summary"`
	ContentFormat    string          `json:"content_format"`
	SchemaVersion    int             `json:"schema_version"`
	DocumentContent  *string         `json:"document_content,omitempty"`
	GraphNodes       json.RawMessage `json:"graph_nodes,omitempty"`
	GraphEdges       json.RawMessage `json:"graph_edges,omitempty"`
	FrameData        json.RawMessage `json:"frame_data,omitempty"`
	TriggerMessageID *string         `json:"trigger_message_id,omitempty"`
	AuthorID         *string         `json:"author_id,omitempty"`
	AuthorType       string          `json:"author_type"`
	Metadata         json.RawMessage `json:"metadata"`
	CreatedAt        time.Time       `json:"created_at"`
	Sources          []RevisionRef   `json:"sources"`
}

type RevisionRef struct {
	ArtifactID string `json:"artifact_id"`
	RevisionID string `json:"revision_id"`
	LinkType   string `json:"link_type"`
}

type ArtifactDetail struct {
	Artifact
	CurrentRevision *ArtifactRevision `json:"current_revision,omitempty"`
}

type RevisionSummary struct {
	ID            string    `json:"id"`
	ArtifactID    string    `json:"artifact_id"`
	VersionNumber int       `json:"version_number"`
	ChangeSummary string    `json:"change_summary"`
	ContentFormat string    `json:"content_format"`
	AuthorID      *string   `json:"author_id,omitempty"`
	AuthorType    string    `json:"author_type"`
	CreatedAt     time.Time `json:"created_at"`
}
