package product

import (
	"encoding/json"
	"time"
)

type Project struct {
	ID          string          `json:"id"`
	UserID      string          `json:"user_id"`
	Title       string          `json:"title"`
	Description string          `json:"description"`
	IsPinned    bool            `json:"is_pinned"`
	Metadata    json.RawMessage `json:"metadata"`
	CreatedAt   time.Time       `json:"created_at"`
	UpdatedAt   time.Time       `json:"updated_at"`
}

type ChatThread struct {
	ID               string          `json:"id"`
	ProjectID        string          `json:"project_id"`
	Title            string          `json:"title"`
	ModeHint         string          `json:"mode_hint"`
	ActiveArtifactID *string         `json:"active_artifact_id,omitempty"`
	Metadata         json.RawMessage `json:"metadata"`
	CreatedAt        time.Time       `json:"created_at"`
	UpdatedAt        time.Time       `json:"updated_at"`
}

type ChatMessage struct {
	ID               string          `json:"id"`
	ProjectID        string          `json:"project_id"`
	ThreadID         string          `json:"thread_id"`
	Role             string          `json:"role"`
	Content          string          `json:"content"`
	ClientMessageID  *string         `json:"client_message_id,omitempty"`
	TargetElementIDs json.RawMessage `json:"target_element_ids,omitempty"`
	Metadata         json.RawMessage `json:"metadata"`
	SequenceNumber   int64           `json:"sequence_number"`
	CreatedAt        time.Time       `json:"created_at"`
}
