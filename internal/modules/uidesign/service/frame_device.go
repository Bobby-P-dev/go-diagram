package service

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/Bobby-P-dev/go-diagram.git/internal/modules/uidesign"
	dtos "github.com/Bobby-P-dev/go-diagram.git/internal/modules/uidesign/dto"
	"strings"
)

// switchFrameDevice converts an existing frame between mobile and web/desktop viewports
// while preserving EVERY other frame on the canvas untouched.
func (s *uiDesignService) switchFrameDevice(
	ctx context.Context,
	project *uidesign.Project,
	messages []uidesign.ChatMessage,
	currentNodes []map[string]interface{},
	targetNodeIdx int,
	currentFrame *dtos.UIFrameData,
	plan *dtos.ChangePlanDTO,
	prompt string,
) (*dtos.ProjectResponse, error) {
	if currentFrame == nil || targetNodeIdx < 0 || targetNodeIdx >= len(currentNodes) {
		return nil, fmt.Errorf("target frame not found for device conversion")
	}

	targetDev := strings.ToLower(strings.TrimSpace(plan.Target.Device))
	if targetDev == "" {
		targetDev = "web"
	}

	sourceDev := strings.ToLower(strings.TrimSpace(currentFrame.Device))
	if sourceDev == "" {
		sourceDev = "mobile"
	}

	// 1. Update dimensions and device in targetFrame
	targetFrame := *currentFrame
	targetFrame.Device = targetDev

	if targetDev == "mobile" {
		targetFrame.Width = 375
		targetFrame.Height = 812
	} else if targetDev == "desktop" {
		targetFrame.Width = 1440
		targetFrame.Height = 900
	} else {
		// web default
		targetFrame.Width = 1024
		targetFrame.Height = 768
	}

	// 2. Adapt HTML for new viewport while strictly preserving content and brand
	if s.aiService != nil && targetFrame.RawHtml != "" {
		sysPrompt := `You are an expert Tailwind CSS frontend engineer specializing in responsive layout adaptation.
The user wants to adapt the viewport/device of an existing screen markup between mobile and desktop web.

CRITICAL RULES:
1. STRICT PRESERVATION: Keep EVERY single word, title, text, form input, button, table, metric, card, icon, brand name, and section from the original markup exactly intact. Do not drop, omit, or invent features.
   If the original screen is a modal, form, settings screen, or dashboard, keep it as THAT EXACT modal/form/settings/dashboard (e.g. centered desktop card/container or wide desktop form). NEVER transform it into a landing page, hero banner, or generic marketing site.
2. ADAPT THE VIEWPORT LAYOUT:
   - When converting from mobile to web desktop:
     * Remove narrow mobile container constraints like "max-w-sm", "max-w-md", "w-[375px]" from outer page containers.
     * Use spacious modern desktop layout containers: "w-full max-w-4xl mx-auto px-6" or centered clean desktop card container.
     * Expand mobile single-column vertical stacks to responsive desktop multi-column layouts (e.g. "grid grid-cols-1 md:grid-cols-2 gap-6" or spacious desktop form layout) where appropriate.
   - When converting from web desktop to mobile:
     * Constrain layout to single-column mobile view (375px width friendly, "max-w-md mx-auto px-4").
3. ANTI-SLOP & CLEANLINESS:
   - Maintain the existing color palette, theme, and styling.
   - Output ONLY the raw static HTML body fragment.
   - NO markdown code blocks, no explanations, no javascript <script> tags, no event handlers.
`
		userPrompt := fmt.Sprintf("Source Device: %s\nTarget Device: %s\nScreen Title: %s\nUser Request: %s\n\nORIGINAL HTML MARKUP:\n%s\n\nReturn ONLY the adapted HTML markup:",
			sourceDev, targetDev, targetFrame.Title, prompt, targetFrame.RawHtml)

		newHtml, err := s.aiService.CallLLMText(sysPrompt, userPrompt)
		if err == nil && len(strings.TrimSpace(newHtml)) > 100 {
			cleanHtml := sanitizeStaticHTML(strings.TrimSpace(newHtml))
			targetFrame.RawHtml = cleanHtml
			if targetFrame.CodeExport == nil {
				targetFrame.CodeExport = make(map[string]string)
			}
			targetFrame.CodeExport["html"] = cleanHtml
		} else {
			// Fallback: replace mobile container classes with web container classes
			if targetDev == "web" || targetDev == "desktop" {
				fallbackHtml := targetFrame.RawHtml
				fallbackHtml = strings.ReplaceAll(fallbackHtml, "max-w-sm mx-auto", "w-full max-w-6xl mx-auto")
				fallbackHtml = strings.ReplaceAll(fallbackHtml, "max-w-md mx-auto", "w-full max-w-6xl mx-auto")
				fallbackHtml = strings.ReplaceAll(fallbackHtml, "max-w-sm", "w-full max-w-6xl")
				targetFrame.RawHtml = fallbackHtml
				if targetFrame.CodeExport != nil {
					targetFrame.CodeExport["html"] = fallbackHtml
				}
			}
		}
	}

	targetFrame.ChangePlan = plan

	// 3. Rebuild updatedNodes while preserving ALL other frames!
	updatedNodes := make([]interface{}, len(currentNodes))
	for i, n := range currentNodes {
		if i == targetNodeIdx {
			frameBytes, _ := json.Marshal(targetFrame)
			var frameMap map[string]interface{}
			_ = json.Unmarshal(frameBytes, &frameMap)
			if rHtml, ok := frameMap["raw_html"].(string); ok && rHtml != "" {
				frameMap["rawHtml"] = rHtml
			}
			updatedNodes[i] = map[string]interface{}{
				"id":       n["id"],
				"type":     "ui_frame",
				"position": n["position"],
				"data":     frameMap,
			}
		} else {
			updatedNodes[i] = n
		}
	}

	// 4. Version + persist
	latestVer, _ := s.projects.GetLatestVersionNumber(ctx, project.ID)
	if latestVer <= 0 {
		latestVer = 1
	}
	versionNum := latestVer + 1

	for _, n := range updatedNodes {
		if nodeMap, ok := n.(map[string]interface{}); ok {
			if dataMap, ok := nodeMap["data"].(map[string]interface{}); ok {
				dataMap["version"] = versionNum
			}
		}
	}

	nodesBytes, _ := json.Marshal(updatedNodes)
	edgesBytes := project.CurrentEdges

	if err := s.projects.UpdateGraph(ctx, project.ID, nodesBytes, edgesBytes); err != nil {
		return nil, fmt.Errorf("failed to update graph: %w", err)
	}

	summary := fmt.Sprintf("✨ **[VIEWPORT_SWITCH]** Frame '%s' berhasil diubah dari %s ke %s dengan konten dan tata letak responsif yang terjaga.",
		targetFrame.Title, strings.ToUpper(sourceDev), strings.ToUpper(targetDev))

	_ = s.projects.CreateVersion(ctx, uidesign.CreateVersionInput{ProjectID: project.ID, VersionNumber: versionNum, ChangeSummary: summary, Nodes: nodesBytes, Edges: edgesBytes, TriggerMessageID: nil})

	msg1, _ := s.projects.AppendMessage(ctx, project.ID, "user", prompt, nil)
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
