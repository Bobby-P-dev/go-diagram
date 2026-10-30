package dto

import (
	"encoding/json"
	"github.com/Bobby-P-dev/go-diagram.git/internal/modules/artifact"
)

type CreateArtifactRequest struct {
	SchemaVersion    int                    `json:"schema_version,omitempty"`
	TriggerMessageID *string                `json:"trigger_message_id,omitempty"`
	Sources          []artifact.RevisionRef `json:"sources,omitempty"`
	Type             string                 `json:"type"`
	Title            string                 `json:"title"`
	Slug             string                 `json:"slug,omitempty"`
	DocumentContent  *string                `json:"document_content,omitempty"`
	GraphNodes       json.RawMessage        `json:"graph_nodes,omitempty"`
	GraphEdges       json.RawMessage        `json:"graph_edges,omitempty"`
	FrameData        json.RawMessage        `json:"frame_data,omitempty"`
	ChangeSummary    string                 `json:"change_summary,omitempty"`
}

type AppendRevisionRequest struct {
	SchemaVersion      int                    `json:"schema_version,omitempty"`
	TriggerMessageID   *string                `json:"trigger_message_id,omitempty"`
	Sources            []artifact.RevisionRef `json:"sources,omitempty"`
	ExpectedRevisionID string                 `json:"expected_revision_id"`
	ChangeSummary      string                 `json:"change_summary"`
	DocumentContent    *string                `json:"document_content,omitempty"`
	GraphNodes         json.RawMessage        `json:"graph_nodes,omitempty"`
	GraphEdges         json.RawMessage        `json:"graph_edges,omitempty"`
	FrameData          json.RawMessage        `json:"frame_data,omitempty"`
}

type RestoreRevisionRequest struct {
	TargetRevisionID   string `json:"target_revision_id"`
	ExpectedRevisionID string `json:"expected_revision_id"`
	ChangeSummary      string `json:"change_summary,omitempty"`
}
