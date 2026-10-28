package service

import (
	"fmt"
	"strings"
	"time"

	dto "github.com/Bobby-P-dev/go-diagram.git/internal/modules/diagram/dto"
)

func (service *DiagramService) RollbackToVersion(projectID, versionID string) (*dto.ProjectResponse, error) {
	return service.RollbackToVersionScoped(projectID, versionID, "")
}

func (service *DiagramService) RollbackToVersionScoped(projectID, versionID, userID string) (*dto.ProjectResponse, error) {
	projectID = strings.TrimSpace(projectID)
	versionID = strings.TrimSpace(versionID)
	if projectID == "" || versionID == "" {
		return nil, fmt.Errorf("project_id and version_id are required")
	}

	if userID != "" {
		if _, _, err := service.projects.GetByIDScoped(projectID, userID, ""); err != nil {
			return nil, fmt.Errorf("project not found or unauthorized: %w", err)
		}
	}

	if service.versions == nil {
		return nil, fmt.Errorf("version management is not configured")
	}

	targetVer, err := service.versions.GetVersionByID(versionID)
	if err != nil {
		return nil, fmt.Errorf("failed to find version: %w", err)
	}

	if targetVer.ProjectID != projectID {
		return nil, fmt.Errorf("version does not belong to the specified project")
	}

	// Update current graph to the target version
	err = service.projects.UpdateGraph(projectID, targetVer.Nodes, targetVer.Edges)
	if err != nil {
		return nil, fmt.Errorf("failed to update project graph on rollback: %w", err)
	}

	// Append assistant announcement message
	rollbackMsg := fmt.Sprintf("Diagram berhasil dikembalikan (rollback) ke Versi %d.", targetVer.VersionNumber)
	_, _ = service.messages.AppendMessage(projectID, "assistant", rollbackMsg, nil)

	// Create a new version record documenting the rollback
	latestVer, _ := service.versions.GetLatestVersionNumber(projectID)
	_, _ = service.versions.CreateVersion(
		projectID,
		latestVer+1,
		fmt.Sprintf("Rollback ke Versi %d", targetVer.VersionNumber),
		targetVer.Nodes,
		targetVer.Edges,
		nil,
	)

	return service.GetProjectByIDScoped(projectID, userID, "")
}

func (service *DiagramService) GetProjectVersions(projectID string) ([]dto.DiagramVersionDTO, error) {
	return service.GetProjectVersionsScoped(projectID, "")
}

func (service *DiagramService) GetProjectVersionsScoped(projectID, userID string) ([]dto.DiagramVersionDTO, error) {
	projectID = strings.TrimSpace(projectID)
	if projectID == "" {
		return nil, fmt.Errorf("project id is required")
	}

	if userID != "" {
		if _, _, err := service.projects.GetByIDScoped(projectID, userID, ""); err != nil {
			return nil, fmt.Errorf("project not found or unauthorized: %w", err)
		}
	}

	if service.versions == nil {
		return nil, fmt.Errorf("version management is not configured")
	}

	versions, err := service.versions.GetVersionsByProjectID(projectID)
	if err != nil {
		return nil, err
	}

	result := make([]dto.DiagramVersionDTO, 0, len(versions))
	for _, v := range versions {
		result = append(result, dto.DiagramVersionDTO{
			ID:            v.ID,
			ProjectID:     v.ProjectID,
			VersionNumber: v.VersionNumber,
			ChangeSummary: v.ChangeSummary,
			Nodes:         v.Nodes,
			Edges:         v.Edges,
			CreatedAt:     v.CreatedAt.Format(time.RFC3339),
		})
	}
	return result, nil
}
