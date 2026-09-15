package services

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/Bobby-P-dev/go-diagram.git/src/dtos"
	"github.com/Bobby-P-dev/go-diagram.git/src/entities"
	"github.com/Bobby-P-dev/go-diagram.git/src/models"
)

type ProjectService struct {
	aiService     *AIService
	projectModel  models.ProjectModelInterface
	messageModel  models.MessageModelInterface
	versionModel  models.VersionModelInterface
	logModel      models.LogModelInterface
	templateModel models.TemplateModelInterface
}

func NewProjectService(
	ai *AIService,
	projectModel models.ProjectModelInterface,
	messageModel models.MessageModelInterface,
	versionModel models.VersionModelInterface,
	logModel models.LogModelInterface,
	templateModel models.TemplateModelInterface,
) *ProjectService {
	return &ProjectService{
		aiService:     ai,
		projectModel:  projectModel,
		messageModel:  messageModel,
		versionModel:  versionModel,
		logModel:      logModel,
		templateModel: templateModel,
	}
}

func generateTitle(prompt string) string {
	words := strings.Fields(strings.TrimSpace(prompt))
	if len(words) == 0 {
		return "Untitled Diagram"
	}
	if len(words) > 5 {
		words = words[:5]
	}
	return strings.Join(words, " ")
}

func (s *ProjectService) CreateNewProject(req dtos.CreateProjectRequest) (*dtos.ProjectResponse, error) {
	// 1. Instant creation from template if TemplateID is provided
	if req.TemplateID != "" && s.templateModel != nil {
		tpl, err := s.templateModel.GetTemplateByID(req.TemplateID)
		if err != nil {
			return nil, fmt.Errorf("template not found: %w", err)
		}

		title := tpl.Name
		if strings.TrimSpace(req.Prompt) != "" {
			title = generateTitle(req.Prompt)
		}

		project, err := s.projectModel.Create(title, tpl.DiagramType, tpl.Nodes, tpl.Edges)
		if err != nil {
			return nil, fmt.Errorf("failed to create project from template: %w", err)
		}

		userMsg, _ := s.messageModel.AppendMessage(project.ID, "user", "Membuat diagram dari template: "+tpl.Name, nil)
		assistantReply := fmt.Sprintf("Diagram berhasil dibuat menggunakan template '%s'. Anda dapat meminta AI untuk memodifikasi atau memperluas diagram ini!", tpl.Name)
		_, _ = s.messageModel.AppendMessage(project.ID, "assistant", assistantReply, nil)

		var triggerID *string
		if userMsg != nil {
			triggerID = &userMsg.ID
		}
		if s.versionModel != nil {
			_, _ = s.versionModel.CreateVersion(project.ID, 1, "Template: "+tpl.Name, tpl.Nodes, tpl.Edges, triggerID)
		}

		return s.GetProjectByID(project.ID)
	}

	prompt := strings.TrimSpace(req.Prompt)
	diagramType := strings.TrimSpace(req.DiagramType)
	if diagramType == "" {
		diagramType = "flowchart"
	}

	// Instant creation of blank canvas project if prompt is empty
	if prompt == "" {
		title := "Untitled Project"
		if diagramType == "ui_design" {
			title = "Untitled UI Canvas"
		}
		project, err := s.projectModel.Create(title, diagramType, json.RawMessage("[]"), json.RawMessage("[]"))
		if err != nil {
			return nil, fmt.Errorf("failed to create blank project: %w", err)
		}
		if s.versionModel != nil {
			_, _ = s.versionModel.CreateVersion(project.ID, 1, "Initial Blank Canvas", json.RawMessage("[]"), json.RawMessage("[]"), nil)
		}
		assistantReply := "✨ Kanvas kosong telah siap. Anda dapat menambahkan diagram alur kerja atau desain melalui AI copilot kapan saja!"
		_, _ = s.messageModel.AppendMessage(project.ID, "assistant", assistantReply, nil)
		return s.GetProjectByID(project.ID)
	}

	title := generateTitle(prompt)

	project, err := s.projectModel.Create(title, diagramType, json.RawMessage("[]"), json.RawMessage("[]"))
	if err != nil {
		return nil, fmt.Errorf("failed to create project: %w", err)
	}

	userMsg, err := s.messageModel.AppendMessage(project.ID, "user", prompt, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to append initial user message: %w", err)
	}

	startTime := time.Now()
	graph, assistantReply, err := s.aiService.GenerateDiagram(nil, "", prompt, diagramType, nil)
	elapsed := time.Since(startTime)

	if err != nil {
		return nil, fmt.Errorf("AI generation failed: %w", err)
	}

	// Non-blocking telemetry logging via Goroutine
	if s.logModel != nil {
		go func(pID string, elapsedMs int) {
			_ = s.logModel.LogAIUsage(entities.AIUsageLog{
				ProjectID: &pID,
				Provider:  s.aiService.provider,
				Model:     s.aiService.model,
				LatencyMS: elapsedMs,
			})
		}(project.ID, int(elapsed.Milliseconds()))
	}

	nodesJSON, err := json.Marshal(graph.Nodes)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal nodes: %w", err)
	}

	edgesJSON, err := json.Marshal(graph.Edges)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal edges: %w", err)
	}

	err = s.projectModel.UpdateGraph(project.ID, nodesJSON, edgesJSON)
	if err != nil {
		return nil, fmt.Errorf("failed to update project graph: %w", err)
	}

	_, err = s.messageModel.AppendMessage(project.ID, "assistant", assistantReply, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to append assistant message: %w", err)
	}

	// Record version 1 snapshot
	if s.versionModel != nil {
		var triggerID *string
		if userMsg != nil {
			triggerID = &userMsg.ID
		}
		_, _ = s.versionModel.CreateVersion(
			project.ID,
			1,
			"Initial diagram created: "+title,
			nodesJSON,
			edgesJSON,
			triggerID,
		)
	}

	return s.GetProjectByID(project.ID)
}

func (s *ProjectService) SendChatMessage(projectID string, req dtos.ChatRequest) (*dtos.ProjectResponse, error) {
	projectID = strings.TrimSpace(projectID)
	if projectID == "" {
		return nil, fmt.Errorf("project id is required")
	}

	prompt := strings.TrimSpace(req.Prompt)
	if prompt == "" {
		return nil, fmt.Errorf("prompt is required")
	}

	project, history, err := s.projectModel.GetByID(projectID)
	if err != nil {
		return nil, fmt.Errorf("project not found: %w", err)
	}

	var targetIDsJSON json.RawMessage
	if len(req.TargetedNodeIDs) > 0 {
		b, err := json.Marshal(req.TargetedNodeIDs)
		if err == nil {
			targetIDsJSON = json.RawMessage(b)
		}
	}

	userMsg, err := s.messageModel.AppendMessage(projectID, "user", prompt, targetIDsJSON)
	if err != nil {
		return nil, fmt.Errorf("failed to append user message: %w", err)
	}

	currentGraph := fmt.Sprintf(`{"nodes": %s, "edges": %s}`, string(project.CurrentNodes), string(project.CurrentEdges))

	startTime := time.Now()
	graph, assistantReply, err := s.aiService.GenerateDiagram(
		history,
		currentGraph,
		prompt,
		project.DiagramType,
		req.TargetedNodeIDs,
	)
	elapsed := time.Since(startTime)

	if err != nil {
		return nil, fmt.Errorf("AI generation failed: %w", err)
	}

	// Non-blocking telemetry logging via Goroutine
	if s.logModel != nil {
		go func(pID string, elapsedMs int) {
			_ = s.logModel.LogAIUsage(entities.AIUsageLog{
				ProjectID: &pID,
				Provider:  s.aiService.provider,
				Model:     s.aiService.model,
				LatencyMS: elapsedMs,
			})
		}(projectID, int(elapsed.Milliseconds()))
	}

	nodesJSON, err := json.Marshal(graph.Nodes)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal nodes: %w", err)
	}

	edgesJSON, err := json.Marshal(graph.Edges)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal edges: %w", err)
	}

	err = s.projectModel.UpdateGraph(projectID, nodesJSON, edgesJSON)
	if err != nil {
		return nil, fmt.Errorf("failed to update graph: %w", err)
	}

	_, err = s.messageModel.AppendMessage(projectID, "assistant", assistantReply, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to append assistant message: %w", err)
	}

	// Record version snapshot
	if s.versionModel != nil {
		latestVer, _ := s.versionModel.GetLatestVersionNumber(projectID)
		var triggerID *string
		if userMsg != nil {
			triggerID = &userMsg.ID
		}
		_, _ = s.versionModel.CreateVersion(
			projectID,
			latestVer+1,
			assistantReply,
			nodesJSON,
			edgesJSON,
			triggerID,
		)
	}

	return s.GetProjectByID(projectID)
}

func (s *ProjectService) RollbackToVersion(projectID, versionID string) (*dtos.ProjectResponse, error) {
	projectID = strings.TrimSpace(projectID)
	versionID = strings.TrimSpace(versionID)
	if projectID == "" || versionID == "" {
		return nil, fmt.Errorf("project_id and version_id are required")
	}

	if s.versionModel == nil {
		return nil, fmt.Errorf("version management is not configured")
	}

	targetVer, err := s.versionModel.GetVersionByID(versionID)
	if err != nil {
		return nil, fmt.Errorf("failed to find version: %w", err)
	}

	if targetVer.ProjectID != projectID {
		return nil, fmt.Errorf("version does not belong to the specified project")
	}

	// Update current graph to the target version
	err = s.projectModel.UpdateGraph(projectID, targetVer.Nodes, targetVer.Edges)
	if err != nil {
		return nil, fmt.Errorf("failed to update project graph on rollback: %w", err)
	}

	// Append assistant announcement message
	rollbackMsg := fmt.Sprintf("Diagram berhasil dikembalikan (rollback) ke Versi %d.", targetVer.VersionNumber)
	_, _ = s.messageModel.AppendMessage(projectID, "assistant", rollbackMsg, nil)

	// Create a new version record documenting the rollback
	latestVer, _ := s.versionModel.GetLatestVersionNumber(projectID)
	_, _ = s.versionModel.CreateVersion(
		projectID,
		latestVer+1,
		fmt.Sprintf("Rollback ke Versi %d", targetVer.VersionNumber),
		targetVer.Nodes,
		targetVer.Edges,
		nil,
	)

	return s.GetProjectByID(projectID)
}

func (s *ProjectService) GetProjectVersions(projectID string) ([]dtos.DiagramVersionDTO, error) {
	projectID = strings.TrimSpace(projectID)
	if projectID == "" {
		return nil, fmt.Errorf("project id is required")
	}

	if s.versionModel == nil {
		return nil, fmt.Errorf("version management is not configured")
	}

	versions, err := s.versionModel.GetVersionsByProjectID(projectID)
	if err != nil {
		return nil, err
	}

	result := make([]dtos.DiagramVersionDTO, 0, len(versions))
	for _, v := range versions {
		result = append(result, dtos.DiagramVersionDTO{
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

func (s *ProjectService) ListTemplates() ([]dtos.TemplateDTO, error) {
	if s.templateModel == nil {
		return nil, fmt.Errorf("template service is not configured")
	}

	templates, err := s.templateModel.GetAllTemplates()
	if err != nil {
		return nil, err
	}

	result := make([]dtos.TemplateDTO, 0, len(templates))
	for _, t := range templates {
		result = append(result, dtos.TemplateDTO{
			ID:          t.ID,
			Name:        t.Name,
			Category:    t.Category,
			Description: t.Description,
			DiagramType: t.DiagramType,
			Nodes:       t.Nodes,
			Edges:       t.Edges,
			IsFeatured:  t.IsFeatured,
		})
	}
	return result, nil
}

func (s *ProjectService) GetProjectByID(id string) (*dtos.ProjectResponse, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return nil, fmt.Errorf("project id is required")
	}

	project, messages, err := s.projectModel.GetByID(id)
	if err != nil {
		return nil, err
	}

	dtoMessages := make([]dtos.ChatMessageDTO, 0, len(messages))
	for _, m := range messages {
		dtoMessages = append(dtoMessages, dtos.ChatMessageDTO{
			ID:            m.ID,
			ProjectID:     m.ProjectID,
			Role:          m.Role,
			Content:       m.Content,
			TargetNodeIDs: m.TargetNodeIDs,
			CreatedAt:     m.CreatedAt.Format(time.RFC3339),
		})
	}

	return &dtos.ProjectResponse{
		ID:           project.ID,
		Title:        project.Title,
		DiagramType:  project.DiagramType,
		CurrentNodes: project.CurrentNodes,
		CurrentEdges: project.CurrentEdges,
		Nodes:        project.CurrentNodes,
		Edges:        project.CurrentEdges,
		Messages:     dtoMessages,
		CreatedAt:    project.CreatedAt.Format(time.RFC3339),
		UpdatedAt:    project.UpdatedAt.Format(time.RFC3339),
	}, nil
}

func (s *ProjectService) ListProjects(limit, offset int) (*dtos.PaginatedProjectsResponse, error) {
	if limit <= 0 {
		limit = 15
	}
	if offset < 0 {
		offset = 0
	}

	summaries, totalCount, err := s.projectModel.GetPaginated(limit, offset)
	if err != nil {
		return nil, err
	}

	items := make([]dtos.ProjectSidebarItem, 0, len(summaries))
	for _, p := range summaries {
		items = append(items, dtos.ProjectSidebarItem{
			ID:          p.ID,
			Title:       p.Title,
			DiagramType: p.DiagramType,
			ProjectMode: p.ProjectMode,
			IsPinned:    p.IsPinned,
			UpdatedAt:   p.UpdatedAt.Format(time.RFC3339),
			CreatedAt:   p.CreatedAt.Format(time.RFC3339),
		})
	}

	hasMore := (offset + len(items)) < totalCount

	return &dtos.PaginatedProjectsResponse{
		Items:      items,
		TotalCount: totalCount,
		Limit:      limit,
		Offset:     offset,
		HasMore:    hasMore,
	}, nil
}

func (s *ProjectService) ToggleProjectPin(id string) (*dtos.TogglePinResponse, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return nil, fmt.Errorf("project id is required")
	}

	isPinned, err := s.projectModel.TogglePin(id)
	if err != nil {
		return nil, err
	}

	return &dtos.TogglePinResponse{
		ID:       id,
		IsPinned: isPinned,
	}, nil
}
