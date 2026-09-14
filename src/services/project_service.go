package services

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/Bobby-P-dev/go-diagram.git/src/dtos"
	"github.com/Bobby-P-dev/go-diagram.git/src/models"
)

type ProjectService struct {
	aiService    *AIService
	projectModel models.ProjectModelInterface
	messageModel models.MessageModelInterface
}

func NewProjectService(
	ai *AIService,
	projectModel models.ProjectModelInterface,
	messageModel models.MessageModelInterface,
) *ProjectService {
	return &ProjectService{
		aiService:    ai,
		projectModel: projectModel,
		messageModel: messageModel,
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
	prompt := strings.TrimSpace(req.Prompt)
	if prompt == "" {
		return nil, fmt.Errorf("prompt is required")
	}

	diagramType := strings.TrimSpace(req.DiagramType)
	if diagramType == "" {
		diagramType = "flowchart"
	}

	title := generateTitle(prompt)

	project, err := s.projectModel.Create(title, diagramType, json.RawMessage("[]"), json.RawMessage("[]"))
	if err != nil {
		return nil, fmt.Errorf("failed to create project: %w", err)
	}

	_, err = s.messageModel.AppendMessage(project.ID, "user", prompt, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to append initial user message: %w", err)
	}

	graph, assistantReply, err := s.aiService.GenerateDiagram(nil, "", prompt, diagramType, nil)
	if err != nil {
		return nil, fmt.Errorf("AI generation failed: %w", err)
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

	_, err = s.messageModel.AppendMessage(projectID, "user", prompt, targetIDsJSON)
	if err != nil {
		return nil, fmt.Errorf("failed to append user message: %w", err)
	}

	currentGraph := fmt.Sprintf(`{"nodes": %s, "edges": %s}`, string(project.CurrentNodes), string(project.CurrentEdges))

	graph, assistantReply, err := s.aiService.GenerateDiagram(
		history,
		currentGraph,
		prompt,
		project.DiagramType,
		req.TargetedNodeIDs,
	)
	if err != nil {
		return nil, fmt.Errorf("AI generation failed: %w", err)
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

	return s.GetProjectByID(projectID)
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

func (s *ProjectService) ListProjects() ([]dtos.ProjectSidebarItem, error) {
	projects, err := s.projectModel.GetAll()
	if err != nil {
		return nil, err
	}

	items := make([]dtos.ProjectSidebarItem, 0, len(projects))
	for _, p := range projects {
		items = append(items, dtos.ProjectSidebarItem{
			ID:          p.ID,
			Title:       p.Title,
			DiagramType: p.DiagramType,
			UpdatedAt:   p.UpdatedAt.Format(time.RFC3339),
			CreatedAt:   p.CreatedAt.Format(time.RFC3339),
		})
	}

	return items, nil
}
