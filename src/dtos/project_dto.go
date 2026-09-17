package dtos

import "encoding/json"

type CreateProjectRequest struct {
	Prompt      string `json:"prompt"`
	DiagramType string `json:"diagram_type"`
	TemplateID  string `json:"template_id,omitempty"`
}

type TargetElementRefDTO struct {
	Type      string `json:"type"`                 // "component", "section", "page"
	ID        string `json:"id"`                   // "cmp-order-button", "sec-header"
	SectionID string `json:"section_id,omitempty"` // "sec-header"
}

type SelectionContextDTO struct {
	Tag  string `json:"tag,omitempty"`  // "button", "header", "div"
	Text string `json:"text,omitempty"` // "Pesan Online"
	Role string `json:"role,omitempty"` // "button", "banner"
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

type TableColumn struct {
	Name       string `json:"name"`
	Type       string `json:"type"`
	IsPK       bool   `json:"is_pk,omitempty"`
	IsFK       bool   `json:"is_fk,omitempty"`
	Constraint string `json:"constraint,omitempty"`
}

type NodeData struct {
	Label   string        `json:"label"`
	SubText string        `json:"subText,omitempty"`
	Columns []TableColumn `json:"columns,omitempty"`
	Lane    string        `json:"lane,omitempty"`
	Icon    string        `json:"icon,omitempty"`
}

type GraphNode struct {
	ID   string   `json:"id"`
	Type string   `json:"type"`
	Data NodeData `json:"data"`
}

type GraphEdge struct {
	ID     string `json:"id"`
	Source string `json:"source"`
	Target string `json:"target"`
	Label  string `json:"label,omitempty"`
	Dashed bool   `json:"dashed,omitempty"`
}

type GraphPayload struct {
	Nodes []GraphNode `json:"nodes"`
	Edges []GraphEdge `json:"edges"`
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

type ProjectSidebarItem struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	DiagramType string `json:"diagram_type"`
	ProjectMode string `json:"project_mode,omitempty"`
	IsPinned    bool   `json:"is_pinned"`
	UpdatedAt   string `json:"updated_at"`
	CreatedAt   string `json:"created_at"`
}

type PaginatedProjectsResponse struct {
	Items      []ProjectSidebarItem `json:"items"`
	TotalCount int                  `json:"total_count"`
	Limit      int                  `json:"limit"`
	Offset     int                  `json:"offset"`
	HasMore    bool                 `json:"has_more"`
}

type TogglePinResponse struct {
	ID       string `json:"id"`
	IsPinned bool   `json:"is_pinned"`
}

type MessageOpenAI struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}
type ResponseFormatOpenAI struct {
	Type string `json:"type"`
}

type ChatRequestOpenAI struct {
	Model          string                `json:"model"`
	Messages       []MessageOpenAI       `json:"messages"`
	Stream         bool                  `json:"stream"`
	MaxTokens      int                   `json:"max_tokens,omitempty"`
	ResponseFormat *ResponseFormatOpenAI `json:"response_format,omitempty"`
}

type OpenAIError struct {
	Message string `json:"message"`
	Type    string `json:"type"`
	Code    string `json:"code"`
}

type ChatResponseOpenAI struct {
	Choices []struct {
		Message struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Error *OpenAIError `json:"error,omitempty"`
}


type DiagramVersionDTO struct {
	ID            string          `json:"id"`
	ProjectID     string          `json:"project_id"`
	VersionNumber int             `json:"version_number"`
	ChangeSummary string          `json:"change_summary"`
	Nodes         json.RawMessage `json:"nodes"`
	Edges         json.RawMessage `json:"edges"`
	CreatedAt     string          `json:"created_at"`
}

type TemplateDTO struct {
	ID          string          `json:"id"`
	Name        string          `json:"name"`
	Category    string          `json:"category"`
	Description string          `json:"description"`
	DiagramType string          `json:"diagram_type"`
	Nodes       json.RawMessage `json:"nodes"`
	Edges       json.RawMessage `json:"edges"`
	IsFeatured  bool            `json:"is_featured"`
}
