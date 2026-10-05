package entities

import "time"

type ProjectShare struct {
	ID         string    `json:"id"`
	ShareToken string    `json:"share_token"`
	ProjectID  string    `json:"project_id"`
	Title      string    `json:"title"`
	IsActive   bool      `json:"is_active"`
	ViewCount  int       `json:"view_count"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}
