package dto

import "encoding/json"

type ProjectShareDTO struct {
	ID         string `json:"id"`
	ShareToken string `json:"share_token"`
	ProjectID  string `json:"project_id"`
	Title      string `json:"title"`
	IsActive   bool   `json:"is_active"`
	ViewCount  int    `json:"view_count"`
	ShareURL   string `json:"share_url"`
	CreatedAt  string `json:"created_at"`
	UpdatedAt  string `json:"updated_at"`
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

type SharedProjectDetailResponse struct {
	Share    ProjectShareDTO  `json:"share"`
	Project  ProjectResponse  `json:"project"`
	Messages []ChatMessageDTO `json:"messages"`
}

type ForkSharedProjectRequest struct {
	Title string `json:"title,omitempty"`
}
