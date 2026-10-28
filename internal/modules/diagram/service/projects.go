package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	dto "github.com/Bobby-P-dev/go-diagram.git/internal/modules/diagram/dto"
	"github.com/Bobby-P-dev/go-diagram.git/internal/platform/aiusage"
)

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

func (service *DiagramService) CreateNewProject(req dto.CreateProjectRequest) (*dto.ProjectResponse, error) {
	return service.CreateNewProjectForUser("", req)
}

func (service *DiagramService) CreateNewProjectForUser(userID string, req dto.CreateProjectRequest) (*dto.ProjectResponse, error) {
	// 1. Instant creation from template if TemplateID is provided
	if req.TemplateID != "" && service.templates != nil {
		tpl, err := service.templates.GetTemplateByID(req.TemplateID)
		if err != nil {
			return nil, fmt.Errorf("template not found: %w", err)
		}

		title := tpl.Name
		if strings.TrimSpace(req.Prompt) != "" {
			title = generateTitle(req.Prompt)
		}

		project, err := service.projects.CreateWithUser(userID, title, tpl.DiagramType, "diagram", json.RawMessage("{}"), tpl.Nodes, tpl.Edges)
		if err != nil {
			return nil, fmt.Errorf("failed to create project from template: %w", err)
		}

		userMsg, _ := service.messages.AppendMessage(project.ID, "user", "Membuat diagram dari template: "+tpl.Name, nil)
		assistantReply := fmt.Sprintf("Diagram berhasil dibuat menggunakan template '%s'. Anda dapat meminta AI untuk memodifikasi atau memperluas diagram ini!", tpl.Name)
		_, _ = service.messages.AppendMessage(project.ID, "assistant", assistantReply, nil)

		var triggerID *string
		if userMsg != nil {
			triggerID = &userMsg.ID
		}
		if service.versions != nil {
			_, _ = service.versions.CreateVersion(project.ID, 1, "Template: "+tpl.Name, tpl.Nodes, tpl.Edges, triggerID)
		}

		return service.GetProjectByID(project.ID)
	}

	prompt := strings.TrimSpace(req.Prompt)
	diagramType := strings.TrimSpace(req.DiagramType)
	pLower := strings.ToLower(prompt)
	if diagramType == "" || diagramType == "flowchart" {
		if strings.Contains(pLower, "usecase") || strings.Contains(pLower, "use case") || strings.Contains(pLower, "use-case") {
			diagramType = "usecase"
		} else if diagramType == "" {
			diagramType = "flowchart"
		}
	}

	// Instant creation of blank canvas project if prompt is empty
	if prompt == "" {
		title := "Untitled Project"
		if diagramType == "ui_design" {
			title = "Untitled UI Canvas"
		}
		project, err := service.projects.CreateWithUser(userID, title, diagramType, "diagram", json.RawMessage("{}"), json.RawMessage("[]"), json.RawMessage("[]"))
		if err != nil {
			return nil, fmt.Errorf("failed to create blank project: %w", err)
		}
		if service.versions != nil {
			_, _ = service.versions.CreateVersion(project.ID, 1, "Initial Blank Canvas", json.RawMessage("[]"), json.RawMessage("[]"), nil)
		}
		assistantReply := "✨ Kanvas kosong telah siap. Anda dapat menambahkan diagram alur kerja atau desain melalui AI copilot kapan saja!"
		_, _ = service.messages.AppendMessage(project.ID, "assistant", assistantReply, nil)
		return service.GetProjectByID(project.ID)
	}

	title := generateTitle(prompt)

	project, err := service.projects.CreateWithUser(userID, title, diagramType, "diagram", json.RawMessage("{}"), json.RawMessage("[]"), json.RawMessage("[]"))
	if err != nil {
		return nil, fmt.Errorf("failed to create project: %w", err)
	}

	userMsg, err := service.messages.AppendMessage(project.ID, "user", prompt, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to append initial user message: %w", err)
	}

	startTime := time.Now()
	graph, assistantReply, err := service.ai.GenerateDiagram(nil, "", prompt, diagramType, nil)
	elapsed := time.Since(startTime)

	if err != nil {
		return nil, fmt.Errorf("AI generation failed: %w", err)
	}

	// Non-blocking telemetry logging via Goroutine
	if service.usage != nil {
		go func(pID string, elapsedMs int) {
			_ = service.usage.LogAIUsage(context.Background(), aiusage.Entry{
				ProjectID: &pID,
				Provider:  service.ai.Provider(),
				Model:     service.ai.Model(),
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

	err = service.projects.UpdateGraph(project.ID, nodesJSON, edgesJSON)
	if err != nil {
		return nil, fmt.Errorf("failed to update project graph: %w", err)
	}

	_, err = service.messages.AppendMessage(project.ID, "assistant", assistantReply, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to append assistant message: %w", err)
	}

	// Record version 1 snapshot
	if service.versions != nil {
		var triggerID *string
		if userMsg != nil {
			triggerID = &userMsg.ID
		}
		_, _ = service.versions.CreateVersion(
			project.ID,
			1,
			"Initial diagram created: "+title,
			nodesJSON,
			edgesJSON,
			triggerID,
		)
	}

	return service.GetProjectByID(project.ID)
}

func (service *DiagramService) GetProjectByID(id string) (*dto.ProjectResponse, error) {
	return service.GetProjectByIDScoped(id, "", "")
}

func (service *DiagramService) GetProjectByIDScoped(id, userID, userRole string) (*dto.ProjectResponse, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return nil, fmt.Errorf("project id is required")
	}

	project, messages, err := service.projects.GetByIDScoped(id, userID, userRole)
	if err != nil {
		return nil, err
	}

	dtoMessages := make([]dto.ChatMessageDTO, 0, len(messages))
	for _, m := range messages {
		dtoMessages = append(dtoMessages, dto.ChatMessageDTO{
			ID:            m.ID,
			ProjectID:     m.ProjectID,
			Role:          m.Role,
			Content:       m.Content,
			TargetNodeIDs: m.TargetNodeIDs,
			CreatedAt:     m.CreatedAt.Format(time.RFC3339),
		})
	}

	return &dto.ProjectResponse{
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

func (service *DiagramService) UpdateProjectGraphScoped(id, userID string, nodes, edges json.RawMessage) error {
	id = strings.TrimSpace(id)
	if id == "" {
		return fmt.Errorf("project id is required")
	}

	if _, _, err := service.projects.GetByIDScoped(id, userID, ""); err != nil {
		return fmt.Errorf("project not found or unauthorized: %w", err)
	}

	return service.projects.UpdateGraph(id, nodes, edges)
}

func (service *DiagramService) ListProjects(limit, offset int) (*dto.PaginatedProjectsResponse, error) {
	return service.ListProjectsForUser("", limit, offset)
}

func (service *DiagramService) ListProjectsForUser(userID string, limit, offset int) (*dto.PaginatedProjectsResponse, error) {
	if limit <= 0 {
		limit = 15
	}
	if offset < 0 {
		offset = 0
	}

	summaries, totalCount, err := service.projects.GetPaginatedScoped(userID, limit, offset)
	if err != nil {
		return nil, err
	}

	items := make([]dto.ProjectSidebarItem, 0, len(summaries))
	for _, p := range summaries {
		items = append(items, dto.ProjectSidebarItem{
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

	return &dto.PaginatedProjectsResponse{
		Items:      items,
		TotalCount: totalCount,
		Limit:      limit,
		Offset:     offset,
		HasMore:    hasMore,
	}, nil
}

func (service *DiagramService) ToggleProjectPin(id string) (*dto.TogglePinResponse, error) {
	return service.ToggleProjectPinScoped(id, "")
}

func (service *DiagramService) ToggleProjectPinScoped(id, userID string) (*dto.TogglePinResponse, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return nil, fmt.Errorf("project id is required")
	}

	if userID != "" {
		if _, _, err := service.projects.GetByIDScoped(id, userID, ""); err != nil {
			return nil, fmt.Errorf("project not found or unauthorized: %w", err)
		}
	}

	isPinned, err := service.projects.TogglePin(id)
	if err != nil {
		return nil, err
	}

	return &dto.TogglePinResponse{
		ID:       id,
		IsPinned: isPinned,
	}, nil
}
