package entities

import "time"

type AccessCredential struct {
	ID            string     `json:"id"`
	CredentialKey string     `json:"credential_key"`
	Name          string     `json:"name"`
	Role          string     `json:"role"` // "admin" | "user"
	IsActive           bool       `json:"is_active"`
	CanGenerateDiagram bool       `json:"can_generate_diagram"`
	CanGenerateUI      bool       `json:"can_generate_ui"`
	ExpiresAt          *time.Time `json:"expires_at"`
	LastLoginAt   *time.Time `json:"last_login_at"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
}
