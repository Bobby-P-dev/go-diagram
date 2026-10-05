package dtos

type VerifyCredentialRequest struct {
	CredentialKey string `json:"credential_key"`
}

type AuthUserDTO struct {
	ID                 string  `json:"id"`
	CredentialKey      string  `json:"credential_key"`
	Name               string  `json:"name"`
	Role               string  `json:"role"`
	IsActive           bool    `json:"is_active"`
	CanGenerateDiagram bool    `json:"can_generate_diagram"`
	CanGenerateUI      bool    `json:"can_generate_ui"`
	ExpiresAt          *string `json:"expires_at"`
	IsExpired          bool    `json:"is_expired"`
	ExpiresInHuman     string  `json:"expires_in_human"`
}

type LoginResponse struct {
	Token string      `json:"token"`
	User  AuthUserDTO `json:"user"`
}

type CreateCredentialRequest struct {
	Name               string `json:"name"`
	DurationMinutes    int    `json:"duration_minutes"` // e.g. 15, 30, 45, 60
	DurationHours      int    `json:"duration_hours"`   // e.g. 1, 3, 12
	DurationDays       int    `json:"duration_days"`    // e.g. 1, 7, 30, or 0 for unlimited
	Role               string `json:"role"`             // "user" | "admin"
	ExpiresAt          string `json:"expires_at"`       // optional custom ISO date
	CanGenerateDiagram *bool  `json:"can_generate_diagram"`
	CanGenerateUI      *bool  `json:"can_generate_ui"`
}

type UpdateCredentialRequest struct {
	Name               string  `json:"name"`
	Role               string  `json:"role"`
	IsActive           *bool   `json:"is_active"`
	ExtendMinutes      int     `json:"extend_minutes"`
	ExtendHours        int     `json:"extend_hours"`
	ExtendDays         int     `json:"extend_days"`
	DurationMinutes    int     `json:"duration_minutes"`
	DurationHours      int     `json:"duration_hours"`
	DurationDays       int     `json:"duration_days"`
	ExpiresAt          *string `json:"expires_at"`
	SetPermanent       bool    `json:"set_permanent"`
	CanGenerateDiagram *bool   `json:"can_generate_diagram"`
	CanGenerateUI      *bool   `json:"can_generate_ui"`
}

type CredentialSummaryDTO struct {
	ID                 string  `json:"id"`
	CredentialKey      string  `json:"credential_key"`
	Name               string  `json:"name"`
	Role               string  `json:"role"`
	IsActive           bool    `json:"is_active"`
	CanGenerateDiagram bool    `json:"can_generate_diagram"`
	CanGenerateUI      bool    `json:"can_generate_ui"`
	ExpiresAt          *string `json:"expires_at"`
	IsExpired          bool    `json:"is_expired"`
	ExpiresInHuman     string  `json:"expires_in_human"`
	ProjectCount       int     `json:"project_count"`
	LastLoginAt        *string `json:"last_login_at"`
	CreatedAt          string  `json:"created_at"`
	UpdatedAt          string  `json:"updated_at"`
}
