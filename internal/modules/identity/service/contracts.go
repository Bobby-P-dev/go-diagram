package service

import (
	"context"
	"time"

	"github.com/Bobby-P-dev/go-diagram.git/internal/modules/identity/repository"
)

type CredentialRecord struct {
	ID                 string
	CredentialKey      string
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

type Summary struct {
	ID                 string
	CredentialKey      string
	Name               string
	Role               string
	IsActive           bool
	CanGenerateDiagram bool
	CanGenerateUI      bool
	ExpiresAt          *string
	IsExpired          bool
	ExpiresInHuman     string
	ProjectCount       int
	LastLoginAt        *string
	CreatedAt          string
	UpdatedAt          string
}

type CreateInput struct {
	Name               string
	Role               string
	ExpiresAt          *time.Time
	CanGenerateDiagram bool
	CanGenerateUI      bool
}

type UpdateInput struct {
	ID                 string
	Name               string
	Role               string
	IsActive           *bool
	ExpiresAt          *time.Time
	UpdateExpiresAt    bool
	CanGenerateDiagram *bool
	CanGenerateUI      *bool
}

type CredentialService interface {
	MigrateCredentialKeys(ctx context.Context) error
	EnsureMasterAdmin(ctx context.Context, bootstrapKey string) error
	FindByKey(ctx context.Context, key string) (*CredentialRecord, error)
	FindByID(ctx context.Context, id string) (*CredentialRecord, error)
	Create(ctx context.Context, input CreateInput) (*CredentialRecord, error)
	Update(ctx context.Context, input UpdateInput) (*CredentialRecord, error)
	Delete(ctx context.Context, id string) error
	UpdateLastLogin(ctx context.Context, id string) error
	GetAllWithStats(ctx context.Context) ([]Summary, error)
}

type CredentialRepository interface {
	MigrateCredentialKeys(ctx context.Context) error
	EnsureMasterAdmin(ctx context.Context, bootstrapKey string) error
	FindByKeyHash(ctx context.Context, keyHash string) (*repository.Credential, error)
	FindByID(ctx context.Context, id string) (*repository.Credential, error)
	Create(ctx context.Context, credential repository.NewCredential) (*repository.Credential, error)
	Update(ctx context.Context, credential repository.UpdateCredential) (*repository.Credential, error)
	Delete(ctx context.Context, id string) error
	UpdateLastLogin(ctx context.Context, id string) error
	GetAllWithStats(ctx context.Context) ([]repository.CredentialStats, error)
}
