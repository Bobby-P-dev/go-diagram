package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/Bobby-P-dev/go-diagram.git/internal/modules/sharing"
)

type shareService struct {
	repository    ShareRepository
	projects      ProjectGateway
	legacyOwnerID string
}

func NewShareService(repository ShareRepository, projects ProjectGateway, legacyOwnerID string) ShareService {
	return &shareService{
		repository:    repository,
		projects:      projects,
		legacyOwnerID: legacyOwnerID,
	}
}

func (s *shareService) CreateOrGetShare(ctx context.Context, projectID, title string) (*sharing.Share, error) {
	title = strings.TrimSpace(title)
	if title == "" {
		title = "Shared Project"
	}
	existing, err := s.repository.FindByProjectID(ctx, projectID)
	if err != nil {
		return nil, fmt.Errorf("failed to check existing share: %w", err)
	}
	if existing != nil {
		return s.repository.Reactivate(ctx, existing.ID, title)
	}
	token, err := generateShareToken()
	if err != nil {
		return nil, err
	}
	return s.repository.Create(ctx, sharing.NewShare{Token: token, ProjectID: projectID, Title: title})
}

func (s *shareService) GetShareByProjectID(ctx context.Context, projectID string) (*sharing.Share, error) {
	share, err := s.repository.FindByProjectID(ctx, projectID)
	if err != nil {
		return nil, fmt.Errorf("failed to query project share: %w", err)
	}
	return share, nil
}

func (s *shareService) RevokeShare(ctx context.Context, projectID string) error {
	return s.repository.Revoke(ctx, projectID)
}

func (s *shareService) GetShareByToken(ctx context.Context, token string) (*sharing.Share, *sharing.Project, []sharing.Message, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return nil, nil, nil, fmt.Errorf("share token is required")
	}
	share, err := s.repository.FindActiveByToken(ctx, token)
	if err != nil {
		return nil, nil, nil, err
	}
	go func(shareID string) {
		viewContext, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancel()
		_ = s.repository.IncrementViewCount(viewContext, shareID)
	}(share.ID)
	if s.projects == nil {
		return nil, nil, nil, fmt.Errorf("project gateway is not configured")
	}
	project, messages, err := s.projects.LoadSharedProject(ctx, share.ProjectID)
	if err != nil {
		return nil, nil, nil, err
	}
	return share, project, messages, nil
}

func (s *shareService) ForkSharedProjectWithUser(ctx context.Context, token, newTitle, userID string) (*sharing.Project, error) {
	_, original, messages, err := s.GetShareByToken(ctx, token)
	if err != nil {
		return nil, err
	}
	title := strings.TrimSpace(newTitle)
	if title == "" {
		title = fmt.Sprintf("%s (Shared Copy)", original.Title)
	}
	if userID == "" {
		userID = s.legacyOwnerID
	}
	if s.projects == nil {
		return nil, fmt.Errorf("project gateway is not configured")
	}
	return s.projects.CloneSharedProject(ctx, sharing.CloneInput{
		Original: *original,
		Messages: messages,
		Title:    title,
		UserID:   userID,
	})
}

func generateShareToken() (string, error) {
	randomBytes := make([]byte, 16)
	if _, err := rand.Read(randomBytes); err != nil {
		return "", fmt.Errorf("failed to generate share token: %w", err)
	}
	return hex.EncodeToString(randomBytes), nil
}
