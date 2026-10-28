package dto

import "encoding/json"

type TargetElementRefDTO struct {
	Type      string `json:"type"`                 // "component", "section", "page"
	ID        string `json:"id"`                   // "cmp-order-button", "sec-header"
	SectionID string `json:"section_id,omitempty"` // "sec-header"
	FrameID   string `json:"frame_id,omitempty"`   // "ui-frame-1", "ui-frame-2"
}

type SelectionContextDTO struct {
	Tag   string `json:"tag,omitempty"`   // "button", "header", "div"
	Text  string `json:"text,omitempty"`  // "Pesan Online"
	Role  string `json:"role,omitempty"`  // "button", "banner"
	Scope string `json:"scope,omitempty"` // "new_screen", "screen", "section", "component"
}

type ChatRequest struct {
	Prompt              string               `json:"prompt"`
	Message             string               `json:"message,omitempty"`
	Instruction         string               `json:"instruction,omitempty"`
	Foundation          string               `json:"foundation,omitempty"`
	TargetedNodeIDs     []string             `json:"targeted_node_ids,omitempty"`
	SelectedComponentID string               `json:"selected_component_id,omitempty"`
	Target              *TargetElementRefDTO `json:"target,omitempty"`
	SelectionContext    *SelectionContextDTO `json:"selection_context,omitempty"`
	VersionNumber       int                  `json:"version_number,omitempty"`
}

type ChatMessageDTO struct {
	ID            string          `json:"id"`
	ProjectID     string          `json:"project_id"`
	Role          string          `json:"role"`
	Content       string          `json:"content"`
	TargetNodeIDs json.RawMessage `json:"target_node_ids,omitempty"`
	CreatedAt     string          `json:"created_at"`
}

type ProjectResponse struct {
	ID            string           `json:"id"`
	Title         string           `json:"title"`
	DiagramType   string           `json:"diagram_type"`
	CurrentNodes  json.RawMessage  `json:"current_nodes"`
	CurrentEdges  json.RawMessage  `json:"current_edges"`
	Nodes         json.RawMessage  `json:"nodes"`
	Edges         json.RawMessage  `json:"edges"`
	Messages      []ChatMessageDTO `json:"messages"`
	Version       int              `json:"version,omitempty"`
	VersionNumber int              `json:"version_number,omitempty"`
	CreatedAt     string           `json:"created_at"`
	UpdatedAt     string           `json:"updated_at"`
}
