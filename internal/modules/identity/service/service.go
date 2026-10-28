package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/Bobby-P-dev/go-diagram.git/internal/modules/identity"
	"github.com/Bobby-P-dev/go-diagram.git/internal/modules/identity/repository"
)

type credentialService struct {
	repository CredentialRepository
}

func NewCredentialService(repository CredentialRepository) CredentialService {
	return &credentialService{repository: repository}
}

func (service *credentialService) MigrateCredentialKeys(ctx context.Context) error {
	return service.repository.MigrateCredentialKeys(ctx)
}

func (service *credentialService) EnsureMasterAdmin(ctx context.Context, bootstrapKey string) error {
	return service.repository.EnsureMasterAdmin(ctx, bootstrapKey)
}

func (service *credentialService) FindByKey(ctx context.Context, key string) (*CredentialRecord, error) {
	key = strings.TrimSpace(key)
	if key == "" {
		return nil, fmt.Errorf("credential key is required")
	}
	record, err := service.repository.FindByKeyHash(ctx, identity.HashCredentialKey(key))
	if err != nil {
		return nil, err
	}
	return mapRecord(record, identity.MaskCredentialKey(record.KeyPrefix)), nil
}

func (service *credentialService) FindByID(ctx context.Context, id string) (*CredentialRecord, error) {
	record, err := service.repository.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return mapRecord(record, identity.MaskCredentialKey(record.KeyPrefix)), nil
}

func (service *credentialService) Create(ctx context.Context, input CreateInput) (*CredentialRecord, error) {
	name := strings.TrimSpace(input.Name)
	if name == "" {
		name = "Pengguna Baru"
	}
	role := input.Role
	if role != "admin" {
		role = "user"
	}
	key, err := identity.GenerateRandomKey()
	if err != nil {
		return nil, err
	}
	record, err := service.repository.Create(ctx, repository.NewCredential{
		KeyHash:            identity.HashCredentialKey(key),
		KeyPrefix:          identity.CredentialKeyPrefix(key),
		Name:               name,
		Role:               role,
		ExpiresAt:          input.ExpiresAt,
		CanGenerateDiagram: input.CanGenerateDiagram,
		CanGenerateUI:      input.CanGenerateUI,
	})
	if err != nil {
		return nil, err
	}
	return mapRecord(record, key), nil
}

func (service *credentialService) Update(ctx context.Context, input UpdateInput) (*CredentialRecord, error) {
	current, err := service.repository.FindByID(ctx, input.ID)
	if err != nil {
		return nil, err
	}

	update := repository.UpdateCredential{
		ID:                 input.ID,
		Name:               current.Name,
		Role:               current.Role,
		IsActive:           current.IsActive,
		ExpiresAt:          current.ExpiresAt,
		CanGenerateDiagram: current.CanGenerateDiagram,
		CanGenerateUI:      current.CanGenerateUI,
	}

	if strings.TrimSpace(input.Name) != "" {
		update.Name = strings.TrimSpace(input.Name)
	}
	if input.Role == "admin" || input.Role == "user" {
		if input.ID != identity.MasterAdminID || input.Role == "admin" {
			update.Role = input.Role
		}
	}
	if input.IsActive != nil {
		if input.ID != identity.MasterAdminID || *input.IsActive {
			update.IsActive = *input.IsActive
		}
	}
	if input.UpdateExpiresAt {
		if input.ID != identity.MasterAdminID {
			update.ExpiresAt = input.ExpiresAt
		}
	}
	if input.ID != identity.MasterAdminID {
		if input.CanGenerateDiagram != nil {
			update.CanGenerateDiagram = *input.CanGenerateDiagram
		}
		if input.CanGenerateUI != nil {
			update.CanGenerateUI = *input.CanGenerateUI
		}
	} else {
		update.CanGenerateDiagram = true
		update.CanGenerateUI = true
	}

	record, err := service.repository.Update(ctx, update)
	if err != nil {
		return nil, err
	}
	return mapRecord(record, identity.MaskCredentialKey(record.KeyPrefix)), nil
}

func (service *credentialService) Delete(ctx context.Context, id string) error {
	if id == identity.MasterAdminID {
		return fmt.Errorf("kredensial Master Admin tidak dapat dihapus")
	}
	return service.repository.Delete(ctx, id)
}

func (service *credentialService) UpdateLastLogin(ctx context.Context, id string) error {
	return service.repository.UpdateLastLogin(ctx, id)
}

func (service *credentialService) GetAllWithStats(ctx context.Context) ([]Summary, error) {
	rows, err := service.repository.GetAllWithStats(ctx)
	if err != nil {
		return nil, err
	}
	var result []Summary
	for _, row := range rows {
		var expiresAtStr *string
		var lastLoginStr *string
		humanText, isExpired := "Permanen (Tanpa Batas)", false

		if row.ExpiresAt != nil {
			formatted := row.ExpiresAt.Format(time.RFC3339)
			expiresAtStr = &formatted
			humanText, isExpired = identity.FormatExpiresInHuman(row.ExpiresAt)
		}
		if row.LastLoginAt != nil {
			formatted := row.LastLoginAt.Format(time.RFC3339)
			lastLoginStr = &formatted
		}

		result = append(result, Summary{
			ID:                 row.ID,
			CredentialKey:      identity.MaskCredentialKey(row.KeyPrefix),
			Name:               row.Name,
			Role:               row.Role,
			IsActive:           row.IsActive,
			CanGenerateDiagram: row.CanGenerateDiagram,
			CanGenerateUI:      row.CanGenerateUI,
			ExpiresAt:          expiresAtStr,
			IsExpired:          isExpired,
			ExpiresInHuman:     humanText,
			ProjectCount:       row.ProjectCount,
			LastLoginAt:        lastLoginStr,
			CreatedAt:          row.CreatedAt.Format(time.RFC3339),
			UpdatedAt:          row.UpdatedAt.Format(time.RFC3339),
		})
	}
	return result, nil
}

func mapRecord(record *repository.Credential, credentialKeyValue string) *CredentialRecord {
	if record == nil {
		return nil
	}
	return &CredentialRecord{
		ID:                 record.ID,
		CredentialKey:      credentialKeyValue,
		Name:               record.Name,
		Role:               record.Role,
		IsActive:           record.IsActive,
		CanGenerateDiagram: record.CanGenerateDiagram,
		CanGenerateUI:      record.CanGenerateUI,
		ExpiresAt:          record.ExpiresAt,
		LastLoginAt:        record.LastLoginAt,
		CreatedAt:          record.CreatedAt,
		UpdatedAt:          record.UpdatedAt,
	}
}
