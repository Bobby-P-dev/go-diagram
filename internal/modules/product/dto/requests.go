package dto

type CreateProjectRequest struct {
	Title       string `json:"title"`
	Description string `json:"description,omitempty"`
}

type CreateThreadRequest struct {
	Title            string  `json:"title"`
	ModeHint         string  `json:"mode_hint,omitempty"`
	ActiveArtifactID *string `json:"active_artifact_id,omitempty"`
}

type PostMessageRequest struct {
	Content         string `json:"content"`
	ClientMessageID string `json:"client_message_id"`
}
