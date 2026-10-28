package uidesign

import (
	"encoding/json"
	"time"
)

type AsyncJobBinding struct {
	JobID         string
	ProjectID     string
	UserID        string
	CorrelationID string
}

type Project struct {
	ID           string          `json:"id"`
	UserID       string          `json:"user_id,omitempty"`
	Title        string          `json:"title"`
	DiagramType  string          `json:"diagram_type"`
	ProjectMode  string          `json:"project_mode,omitempty"`
	Metadata     json.RawMessage `json:"metadata,omitempty"`
	IsPinned     bool            `json:"is_pinned"`
	CurrentNodes json.RawMessage `json:"current_nodes"`
	CurrentEdges json.RawMessage `json:"current_edges"`
	CreatedAt    time.Time       `json:"created_at"`
	UpdatedAt    time.Time       `json:"updated_at"`
}

type ChatMessage struct {
	ID            string          `json:"id"`
	ProjectID     string          `json:"project_id"`
	Role          string          `json:"role"`
	Content       string          `json:"content"`
	TargetNodeIDs json.RawMessage `json:"target_node_ids,omitempty"`
	CreatedAt     time.Time       `json:"created_at"`
}

type CreateProjectInput struct {
	UserID      string
	Title       string
	DiagramType string
	ProjectMode string
	Metadata    json.RawMessage
	Nodes       json.RawMessage
	Edges       json.RawMessage
}

type CreateVersionInput struct {
	ProjectID        string
	VersionNumber    int
	ChangeSummary    string
	Nodes            json.RawMessage
	Edges            json.RawMessage
	TriggerMessageID *string
}
