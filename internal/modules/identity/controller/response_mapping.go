package controller

import (
	"time"

	"github.com/Bobby-P-dev/go-diagram.git/internal/modules/identity"
	"github.com/Bobby-P-dev/go-diagram.git/internal/modules/identity/dto"
	"github.com/Bobby-P-dev/go-diagram.git/internal/modules/identity/service"
)

func authUserDTO(user *service.CredentialRecord) dto.AuthUserDTO {
	var expiresAt *string
	if user.ExpiresAt != nil {
		formatted := user.ExpiresAt.Format(time.RFC3339)
		expiresAt = &formatted
	}
	humanText, isExpired := identity.FormatExpiresInHuman(user.ExpiresAt)
	return dto.AuthUserDTO{
		ID: user.ID, CredentialKey: user.CredentialKey, Name: user.Name, Role: user.Role,
		IsActive: user.IsActive, CanGenerateDiagram: user.CanGenerateDiagram, CanGenerateUI: user.CanGenerateUI,
		ExpiresAt: expiresAt, IsExpired: isExpired, ExpiresInHuman: humanText,
	}
}

func credentialSummaryDTO(user *service.CredentialRecord) dto.CredentialSummaryDTO {
	profile := authUserDTO(user)
	return dto.CredentialSummaryDTO{
		ID: profile.ID, CredentialKey: profile.CredentialKey, Name: profile.Name, Role: profile.Role,
		IsActive: profile.IsActive, CanGenerateDiagram: profile.CanGenerateDiagram, CanGenerateUI: profile.CanGenerateUI,
		ExpiresAt: profile.ExpiresAt, IsExpired: profile.IsExpired, ExpiresInHuman: profile.ExpiresInHuman,
		CreatedAt: user.CreatedAt.Format(time.RFC3339), UpdatedAt: user.UpdatedAt.Format(time.RFC3339),
	}
}
