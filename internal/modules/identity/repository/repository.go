package repository

import (
	"context"
	"time"
)

type Credential struct {
	ID                 string
	KeyPrefix          string
	Name               string
	Role               string
	IsActive           bool
	CanGenerateDiagram bool
	CanGenerateUI      bool
	ExpiresAt          *time.Time
	LastLoginAt        *time.Time
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

type CredentialStats struct {
	Credential
	ProjectCount int
}

type NewCredential struct {
	KeyHash            string
	KeyPrefix          string
	Name               string
	Role               string
	ExpiresAt          *time.Time
	CanGenerateDiagram bool
	CanGenerateUI      bool
}

type UpdateCredential struct {
	ID                 string
	Name               string
	Role               string
	IsActive           bool
	ExpiresAt          *time.Time
	CanGenerateDiagram bool
	CanGenerateUI      bool
}

type Repository interface {
	MigrateCredentialKeys(ctx context.Context) error
	EnsureMasterAdmin(ctx context.Context, bootstrapKey string) error
	FindByKeyHash(ctx context.Context, keyHash string) (*Credential, error)
	FindByID(ctx context.Context, id string) (*Credential, error)
	Create(ctx context.Context, credential NewCredential) (*Credential, error)
	Update(ctx context.Context, credential UpdateCredential) (*Credential, error)
	Delete(ctx context.Context, id string) error
	UpdateLastLogin(ctx context.Context, id string) error
	GetAllWithStats(ctx context.Context) ([]CredentialStats, error)
}
