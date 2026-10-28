package dto

import "encoding/json"

type CreateCommentRequest struct {
	NodeID   string          `json:"node_id"`
	Author   string          `json:"author"`
	Content  string          `json:"content"`
	PosX     float64         `json:"position_x"`
	PosY     float64         `json:"position_y"`
	Metadata json.RawMessage `json:"metadata,omitempty"`
}

type UpdateCommentStatusRequest struct {
	Status string `json:"status"`
}

type UpdateSettingsRequest struct {
	DefaultMode       string          `json:"default_mode"`
	Theme             string          `json:"theme"`
	PreferredAIModel  string          `json:"preferred_ai_model"`
	CanvasPreferences json.RawMessage `json:"canvas_preferences"`
}
