package dtos

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

type SharedProjectDetailResponse struct {
	Share    ProjectShareDTO  `json:"share"`
	Project  ProjectResponse  `json:"project"`
	Messages []ChatMessageDTO `json:"messages"`
}

type ForkSharedProjectRequest struct {
	Title string `json:"title,omitempty"`
}
