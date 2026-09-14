package entities

import (
	"encoding/json"
	"time"
)

type Project struct {
	ID           string          `json:"id"`
	Title        string          `json:"title"`
	DiagramType  string          `json:"diagram_type"`
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
