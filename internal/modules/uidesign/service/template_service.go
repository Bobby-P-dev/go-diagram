package service

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/Bobby-P-dev/go-diagram.git/internal/modules/uidesign"
	dtos "github.com/Bobby-P-dev/go-diagram.git/internal/modules/uidesign/dto"
)

func (s *uiDesignService) GetTemplates(ctx context.Context) ([]uidesign.UITemplate, error) {
	return s.uiTemplateModel.GetAllUITemplates(ctx)
}

func (s *uiDesignService) CreateProjectFromTemplate(ctx context.Context, templateID string) (*dtos.ProjectResponse, error) {
	tpl, err := s.uiTemplateModel.GetUITemplateByID(ctx, templateID)
	if err != nil {
		return nil, fmt.Errorf("failed to get ui template: %w", err)
	}

	width := 1024
	height := 720
	if tpl.Device == "mobile" {
		width = 375
		height = 812
	} else if tpl.Device == "desktop" {
		width = 1100
		height = 740
	}

	var themeMap map[string]interface{}
	_ = json.Unmarshal(tpl.Theme, &themeMap)

	var sectionsList []interface{}
	_ = json.Unmarshal(tpl.Sections, &sectionsList)

	var codeExportMap map[string]string
	_ = json.Unmarshal(tpl.CodeExport, &codeExportMap)

	nodeData := map[string]interface{}{
		"device":      tpl.Device,
		"title":       tpl.Name,
		"width":       width,
		"height":      height,
		"theme":       themeMap,
		"sections":    sectionsList,
		"code_export": codeExportMap,
	}

	frameNode := map[string]interface{}{
		"id":       "ui-frame-1",
		"type":     "ui_frame",
		"position": map[string]float64{"x": 60, "y": 60},
		"data":     nodeData,
	}

	nodesBytes, _ := json.Marshal([]interface{}{frameNode})
	edgesBytes := json.RawMessage("[]")

	metaBytes, _ := json.Marshal(map[string]interface{}{
		"target_device": tpl.Device,
		"category":      tpl.Category,
		"template_id":   tpl.ID,
	})

	userID := s.userID(ctx)

	project, err := s.projects.CreateProject(ctx, uidesign.CreateProjectInput{UserID: userID, Title: tpl.Name, DiagramType: "ui_design", ProjectMode: "ui_design", Metadata: metaBytes, Nodes: nodesBytes, Edges: edgesBytes})
	if err != nil {
		return nil, fmt.Errorf("failed to create ui design project: %w", err)
	}

	// Inisialisasi Version 1
	_ = s.projects.CreateVersion(ctx, uidesign.CreateVersionInput{ProjectID: project.ID, VersionNumber: 1, ChangeSummary: "Initial UI Design from template: " + tpl.Name, Nodes: nodesBytes, Edges: edgesBytes, TriggerMessageID: nil})

	// Chat history
	msg1, _ := s.projects.AppendMessage(ctx, project.ID, "user", "Membuat UI Design dari template: "+tpl.Name, nil)
	msg2, _ := s.projects.AppendMessage(ctx, project.ID, "assistant", "UI Design berhasil dibuat dari template '"+tpl.Name+"'. Anda dapat melihat pratinjau komponen, menyalin kode Tailwind/Vue, atau meminta AI untuk memodifikasi section!", nil)

	return &dtos.ProjectResponse{
		ID:           project.ID,
		Title:        project.Title,
		DiagramType:  project.DiagramType,
		CurrentNodes: project.CurrentNodes,
		CurrentEdges: project.CurrentEdges,
		Nodes:        project.CurrentNodes,
		Edges:        project.CurrentEdges,
		Messages: []dtos.ChatMessageDTO{
			{
				ID:        msg1.ID,
				ProjectID: msg1.ProjectID,
				Role:      msg1.Role,
				Content:   msg1.Content,
				CreatedAt: msg1.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
			},
			{
				ID:        msg2.ID,
				ProjectID: msg2.ProjectID,
				Role:      msg2.Role,
				Content:   msg2.Content,
				CreatedAt: msg2.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
			},
		},
		CreatedAt: project.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		UpdatedAt: project.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}, nil
}
