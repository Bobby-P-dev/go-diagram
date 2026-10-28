package service

import (
	"github.com/Bobby-P-dev/go-diagram.git/internal/modules/workspace"
	"github.com/Bobby-P-dev/go-diagram.git/internal/modules/workspace/dto"
)

func (s *WorkspaceService) GetFoundations() ([]workspace.DesignFoundation, error) {
	return s.foundations.GetAll()
}

func (s *WorkspaceService) GetSettings() (*workspace.WorkspaceSetting, error) {
	return s.settings.Get()
}

func (s *WorkspaceService) UpdateSettings(input dto.UpdateSettingsRequest) (*workspace.WorkspaceSetting, error) {
	return s.settings.Update(input.DefaultMode, input.Theme, input.PreferredAIModel, input.CanvasPreferences)
}
