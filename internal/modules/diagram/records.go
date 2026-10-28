package diagram

import (
	"encoding/json"
	"time"
)

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

type CloneInput struct {
	Original Project
	Messages []ChatMessage
	Title string
	UserID string
}

type ProjectSummary struct {
	ID          string    `json:"id"`
	UserID      string    `json:"user_id,omitempty"`
	Title       string    `json:"title"`
	DiagramType string    `json:"diagram_type"`
	ProjectMode string    `json:"project_mode"`
	IsPinned    bool      `json:"is_pinned"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type ChatMessage struct {
	ID            string          `json:"id"`
	ProjectID     string          `json:"project_id"`
	Role          string          `json:"role"`
	Content       string          `json:"content"`
	TargetNodeIDs json.RawMessage `json:"target_node_ids,omitempty"`
	Metadata      json.RawMessage `json:"metadata,omitempty"`
	CreatedAt     time.Time       `json:"created_at"`
}

type DiagramVersion struct {
	ID               string          `json:"id"`
	ProjectID        string          `json:"project_id"`
	VersionNumber    int             `json:"version_number"`
	ChangeSummary    string          `json:"change_summary"`
	Nodes            json.RawMessage `json:"nodes"`
	Edges            json.RawMessage `json:"edges"`
	TriggerMessageID *string         `json:"trigger_message_id,omitempty"`
	CreatedAt        time.Time       `json:"created_at"`
}

type DiagramTemplate struct {
	ID          string          `json:"id"`
	Name        string          `json:"name"`
	Category    string          `json:"category"`
	Description string          `json:"description"`
	DiagramType string          `json:"diagram_type"`
	Nodes       json.RawMessage `json:"nodes"`
	Edges       json.RawMessage `json:"edges"`
	IsFeatured  bool            `json:"is_featured"`
	CreatedAt   time.Time       `json:"created_at"`
}
