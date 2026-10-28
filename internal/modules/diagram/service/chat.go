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

func (service *DiagramService) SendChatMessage(projectID string, req dto.ChatRequest) (*dto.ProjectResponse, error) {
	return service.SendChatMessageScoped(projectID, req, "")
}

func (service *DiagramService) SendChatMessageScoped(projectID string, req dto.ChatRequest, userID string) (*dto.ProjectResponse, error) {
	projectID = strings.TrimSpace(projectID)
	if projectID == "" {
		return nil, fmt.Errorf("project id is required")
	}

	prompt := strings.TrimSpace(req.Prompt)
	if prompt == "" {
		return nil, fmt.Errorf("prompt is required")
	}

	project, history, err := service.projects.GetByIDScoped(projectID, userID, "")
	if err != nil {
		return nil, fmt.Errorf("project not found or unauthorized: %w", err)
	}

	var targetIDsJSON json.RawMessage
	if len(req.TargetedNodeIDs) > 0 {
		b, err := json.Marshal(req.TargetedNodeIDs)
		if err == nil {
			targetIDsJSON = json.RawMessage(b)
		}
	}

	userMsg, err := service.messages.AppendMessage(projectID, "user", prompt, targetIDsJSON)
	if err != nil {
		return nil, fmt.Errorf("failed to append user message: %w", err)
	}

	currentGraph := fmt.Sprintf(`{"nodes": %s, "edges": %s}`, string(project.CurrentNodes), string(project.CurrentEdges))

	effectiveDiagramType := project.DiagramType
	pLower := strings.ToLower(prompt)
	if strings.Contains(pLower, "usecase") || strings.Contains(pLower, "use case") || strings.Contains(pLower, "use-case") {
		effectiveDiagramType = "usecase"
	}

	startTime := time.Now()
	graph, assistantReply, err := service.ai.GenerateDiagram(
		history,
		currentGraph,
		prompt,
		effectiveDiagramType,
		req.TargetedNodeIDs,
	)
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

	err = service.projects.UpdateGraph(projectID, nodesJSON, edgesJSON)
	if err != nil {
		return nil, fmt.Errorf("failed to update graph: %w", err)
	}

	_, err = service.messages.AppendMessage(projectID, "assistant", assistantReply, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to append assistant message: %w", err)
	}

	// Record version snapshot
	if service.versions != nil {
		latestVer, _ := service.versions.GetLatestVersionNumber(projectID)
		var triggerID *string
		if userMsg != nil {
			triggerID = &userMsg.ID
		}
		_, _ = service.versions.CreateVersion(
			projectID,
			latestVer+1,
			assistantReply,
			nodesJSON,
			edgesJSON,
			triggerID,
		)
	}

	return service.GetProjectByIDScoped(projectID, userID, "")
}
