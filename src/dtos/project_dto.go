package dtos

import "encoding/json"

type CreateProjectRequest struct {
	Prompt      string `json:"prompt"`
	DiagramType string `json:"diagram_type"`
}

type ChatRequest struct {
	Prompt          string   `json:"prompt"`
	TargetedNodeIDs []string `json:"targeted_node_ids,omitempty"`
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
	ID           string           `json:"id"`
	Title        string           `json:"title"`
	DiagramType  string           `json:"diagram_type"`
	CurrentNodes json.RawMessage  `json:"current_nodes"`
	CurrentEdges json.RawMessage  `json:"current_edges"`
	Nodes        json.RawMessage  `json:"nodes"`
	Edges        json.RawMessage  `json:"edges"`
	Messages     []ChatMessageDTO `json:"messages"`
	CreatedAt    string           `json:"created_at"`
	UpdatedAt    string           `json:"updated_at"`
}

type ProjectSidebarItem struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	DiagramType string `json:"diagram_type"`
	UpdatedAt   string `json:"updated_at"`
	CreatedAt   string `json:"created_at"`
}

type MessageOpenAI struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}
type ResponseFormatOpenAI struct {
	Type string `json:"type"`
}

type ChatRequestOpenAI struct {
	Model          string               `json:"model"`
	Messages       []MessageOpenAI      `json:"messages"`
	Stream         bool                 `json:"stream"`
	ResponseFormat ResponseFormatOpenAI `json:"response_format"`
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
