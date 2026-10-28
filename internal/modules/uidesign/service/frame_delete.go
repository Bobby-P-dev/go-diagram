package service

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/Bobby-P-dev/go-diagram.git/internal/modules/uidesign"
	dtos "github.com/Bobby-P-dev/go-diagram.git/internal/modules/uidesign/dto"
)

// deleteFrame removes an existing frame from the project canvas
func (s *uiDesignService) deleteFrame(
	ctx context.Context,
	project *uidesign.Project,
	messages []uidesign.ChatMessage,
	currentNodes []map[string]interface{},
	targetNodeIdx int,
	currentFrame *dtos.UIFrameData,
	userPrompt string,
) (*dtos.ProjectResponse, error) {
	if len(currentNodes) <= 1 {
		return nil, fmt.Errorf("tidak dapat menghapus screen: proyek harus memiliki minimal satu screen di kanvas")
	}
	if targetNodeIdx < 0 || targetNodeIdx >= len(currentNodes) {
		return nil, fmt.Errorf("target screen yang ingin dihapus tidak ditemukan")
	}

	deletedTitle := "Screen"
	if currentFrame != nil && currentFrame.Title != "" {
		deletedTitle = currentFrame.Title
	}

	var updatedNodes []map[string]interface{}
	deletedNodeID := fmt.Sprintf("%v", currentNodes[targetNodeIdx]["id"])
	for i, n := range currentNodes {
		if i != targetNodeIdx {
			updatedNodes = append(updatedNodes, n)
		}
	}

	nodesBytes, _ := json.Marshal(updatedNodes)
	var currentEdges []map[string]interface{}
	_ = json.Unmarshal(project.CurrentEdges, &currentEdges)
	var updatedEdges []map[string]interface{}
	for _, e := range currentEdges {
		src := fmt.Sprintf("%v", e["source"])
		tgt := fmt.Sprintf("%v", e["target"])
		if src != deletedNodeID && tgt != deletedNodeID {
			updatedEdges = append(updatedEdges, e)
		}
	}
	edgesBytes, _ := json.Marshal(updatedEdges)

	latestVer, _ := s.projects.GetLatestVersionNumber(ctx, project.ID)
	if latestVer <= 0 {
		latestVer = 1
	}
	versionNum := latestVer + 1
	if err := s.projects.UpdateGraph(ctx, project.ID, nodesBytes, edgesBytes); err != nil {
		return nil, fmt.Errorf("failed to update graph: %w", err)
	}

	summary := fmt.Sprintf("🗑️ **[DELETE_FRAME]** Screen '%s' berhasil dihapus dari kanvas.", deletedTitle)
	_ = s.projects.CreateVersion(ctx, uidesign.CreateVersionInput{ProjectID: project.ID, VersionNumber: versionNum, ChangeSummary: summary, Nodes: nodesBytes, Edges: edgesBytes, TriggerMessageID: nil})

	msg1, _ := s.projects.AppendMessage(ctx, project.ID, "user", userPrompt, nil)
	msg2, _ := s.projects.AppendMessage(ctx, project.ID, "assistant", summary, nil)

	updatedMessages := append(messages, *msg1, *msg2)
	var messageDTOs []dtos.ChatMessageDTO
	for _, m := range updatedMessages {
		messageDTOs = append(messageDTOs, dtos.ChatMessageDTO{
			ID:        m.ID,
			ProjectID: m.ProjectID,
			Role:      m.Role,
			Content:   m.Content,
			CreatedAt: m.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		})
	}

	return &dtos.ProjectResponse{
		ID:            project.ID,
		Title:         project.Title,
		DiagramType:   project.DiagramType,
		CurrentNodes:  nodesBytes,
		CurrentEdges:  edgesBytes,
		Nodes:         nodesBytes,
		Edges:         edgesBytes,
		Messages:      messageDTOs,
		Version:       versionNum,
		VersionNumber: versionNum,
		CreatedAt:     project.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		UpdatedAt:     project.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}, nil
}
