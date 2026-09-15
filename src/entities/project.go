package entities

import (
	"encoding/json"
	"time"
)

type Project struct {
	ID           string          `json:"id"`
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

type ProjectSummary struct {
	ID          string    `json:"id"`
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

type AIUsageLog struct {
	ID               string    `json:"id"`
	ProjectID        *string   `json:"project_id,omitempty"`
	Provider         string    `json:"provider"`
	Model            string    `json:"model"`
	PromptTokens     int       `json:"prompt_tokens"`
	CompletionTokens int       `json:"completion_tokens"`
	TotalTokens      int       `json:"total_tokens"`
	LatencyMS        int       `json:"latency_ms"`
	CreatedAt        time.Time `json:"created_at"`
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


type DesignComment struct {
	ID        string    `json:"id"`
	ProjectID string    `json:"project_id"`
	NodeID    *string   `json:"node_id,omitempty"`
	Author    string    `json:"author"`
	Content   string    `json:"content"`
	Status    string    `json:"status"`
	PosX      float64   `json:"position_x"`
	PosY      float64   `json:"position_y"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type DesignFoundation struct {
	ID          string          `json:"id"`
	Name        string          `json:"name"`
	Category    string          `json:"category"`
	Description string          `json:"description"`
	Tokens      json.RawMessage `json:"tokens"`
	IsDefault   bool            `json:"is_default"`
	CreatedAt   time.Time       `json:"created_at"`
	UpdatedAt   time.Time       `json:"updated_at"`
}

type ProjectExport struct {
	ID            string          `json:"id"`
	ProjectID     string          `json:"project_id"`
	ExportType    string          `json:"export_type"`
	FileName      string          `json:"file_name"`
	Content       string          `json:"content"`
	FileSizeBytes int             `json:"file_size_bytes"`
	Metadata      json.RawMessage `json:"metadata,omitempty"`
	CreatedAt     time.Time       `json:"created_at"`
}

type WorkspaceSetting struct {
	ID                string          `json:"id"`
	DefaultMode       string          `json:"default_mode"`
	Theme             string          `json:"theme"`
	PreferredAIModel  string          `json:"preferred_ai_model"`
	CanvasPreferences json.RawMessage `json:"canvas_preferences"`
	UpdatedAt         time.Time       `json:"updated_at"`
}
