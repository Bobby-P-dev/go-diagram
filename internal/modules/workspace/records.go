package workspace

import (
	"encoding/json"
	"time"
)

type DesignComment struct {
	ID        string          `json:"id"`
	ProjectID string          `json:"project_id"`
	NodeID    *string         `json:"node_id,omitempty"`
	Author    string          `json:"author"`
	Content   string          `json:"content"`
	Status    string          `json:"status"`
	PosX      float64         `json:"position_x"`
	PosY      float64         `json:"position_y"`
	Metadata  json.RawMessage `json:"metadata,omitempty"`
	CreatedAt time.Time       `json:"created_at"`
	UpdatedAt time.Time       `json:"updated_at"`
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
