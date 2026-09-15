package services

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/Bobby-P-dev/go-diagram.git/src/dtos"
	"github.com/Bobby-P-dev/go-diagram.git/src/entities"
	"github.com/Bobby-P-dev/go-diagram.git/src/models"
)

type UIDesignServiceInterface interface {
	GetTemplates(ctx context.Context) ([]entities.UITemplate, error)
	CreateProjectFromTemplate(ctx context.Context, templateID string) (*dtos.ProjectResponse, error)
	GenerateUIDesign(ctx context.Context, req dtos.CreateUIDesignRequest) (*dtos.ProjectResponse, error)
	IterateUIDesignWithChat(ctx context.Context, projectID, prompt string, targetedNodeIDs []string) (*dtos.ProjectResponse, error)
}

type uiDesignService struct {
	projectModel    models.ProjectModelInterface
	messageModel    models.MessageModelInterface
	versionModel    models.VersionModelInterface
	uiTemplateModel models.UITemplateModelInterface
	aiService       *AIService
	compiler        *UIDesignCompiler
	changeAnalyzer  *ChangeAnalyzer
	patchEngine     *PatchEngine
}

func NewUIDesignService(
	projectModel models.ProjectModelInterface,
	messageModel models.MessageModelInterface,
	versionModel models.VersionModelInterface,
	uiTemplateModel models.UITemplateModelInterface,
	aiService *AIService,
) UIDesignServiceInterface {
	return &uiDesignService{
		projectModel:    projectModel,
		messageModel:    messageModel,
		versionModel:    versionModel,
		uiTemplateModel: uiTemplateModel,
		aiService:       aiService,
		compiler:        NewUIDesignCompiler(aiService),
		changeAnalyzer:  NewChangeAnalyzer(aiService),
		patchEngine:     NewPatchEngine(),
	}
}

func (s *uiDesignService) GetTemplates(ctx context.Context) ([]entities.UITemplate, error) {
	return s.uiTemplateModel.GetAllUITemplates()
}

func (s *uiDesignService) CreateProjectFromTemplate(ctx context.Context, templateID string) (*dtos.ProjectResponse, error) {
	tpl, err := s.uiTemplateModel.GetUITemplateByID(templateID)
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

	project, err := s.projectModel.CreateWithMode(
		tpl.Name,
		"ui_design",
		"ui_design",
		metaBytes,
		nodesBytes,
		edgesBytes,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create ui design project: %w", err)
	}

	// Inisialisasi Version 1
	_, _ = s.versionModel.CreateVersion(project.ID, 1, "Initial UI Design from template: "+tpl.Name, nodesBytes, edgesBytes, nil)

	// Chat history
	msg1, _ := s.messageModel.AppendMessage(project.ID, "user", "Membuat UI Design dari template: "+tpl.Name, nil)
	msg2, _ := s.messageModel.AppendMessage(project.ID, "assistant", "UI Design berhasil dibuat dari template '"+tpl.Name+"'. Anda dapat melihat pratinjau komponen, menyalin kode Tailwind/Vue, atau meminta AI untuk memodifikasi section!", nil)

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

func (s *uiDesignService) GenerateUIDesign(ctx context.Context, req dtos.CreateUIDesignRequest) (*dtos.ProjectResponse, error) {
	if req.TemplateID != "" {
		return s.CreateProjectFromTemplate(ctx, req.TemplateID)
	}

	device := strings.ToLower(strings.TrimSpace(req.Device))
	if device == "" {
		device = "web"
	}

	theme := strings.TrimSpace(req.Theme)
	if theme == "" {
		theme = "Modern Custom"
	}

	themeMode := strings.ToLower(strings.TrimSpace(req.ThemeMode))
	if themeMode == "" {
		if strings.Contains(strings.ToLower(theme), "light") {
			themeMode = "light"
		} else {
			themeMode = "dark"
		}
	}

	accentColor := strings.TrimSpace(req.AccentColor)
	if accentColor == "" {
		accentColor = "#6366f1"
	}

	foundation := strings.ToLower(strings.TrimSpace(req.Foundation))
	if foundation == "" {
		foundation = "ramp"
	}

	productContext := strings.ToLower(strings.TrimSpace(req.ProductContext))
	if productContext == "" {
		productContext = "saas"
	}

	prompt := strings.TrimSpace(req.Prompt)
	if customTone := strings.TrimSpace(req.CustomTone); customTone != "" && prompt != "" {
		prompt = prompt + " (Tone: " + customTone + ")"
	}
	if prompt == "" {
		title := "Untitled UI Design"
		nodesBytes := json.RawMessage("[]")
		edgesBytes := json.RawMessage("[]")
		metaBytes, _ := json.Marshal(map[string]interface{}{
			"target_device":   device,
			"theme":           theme,
			"theme_mode":      themeMode,
			"accent_color":    accentColor,
			"foundation":      foundation,
			"product_context": productContext,
		})

		project, err := s.projectModel.CreateWithMode(
			title,
			"ui_design",
			"ui_design",
			metaBytes,
			nodesBytes,
			edgesBytes,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to save blank ui project: %w", err)
		}

		_, _ = s.versionModel.CreateVersion(project.ID, 1, "Initial Blank UI Canvas", nodesBytes, edgesBytes, nil)
		assistantReply := "✨ **Kanvas UI Design baru telah siap.**\n\nKetik ide atau deskripsi antarmuka yang ingin Anda bangun di AI Copilot untuk mulai mendesain!"
		msg, _ := s.messageModel.AppendMessage(project.ID, "assistant", assistantReply, nil)

		var msgs []dtos.ChatMessageDTO
		if msg != nil {
			msgs = append(msgs, dtos.ChatMessageDTO{
				ID:        msg.ID,
				ProjectID: msg.ProjectID,
				Role:      msg.Role,
				Content:   msg.Content,
				CreatedAt: msg.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
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
			Messages:     msgs,
			CreatedAt:    project.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
			UpdatedAt:    project.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
		}, nil
	}	// Compile UI Design through the 11-Layer AI Design Compiler
	dsl, err := s.compiler.Compile(ctx, prompt, device, foundation, themeMode, accentColor)
	if err != nil {
		return nil, fmt.Errorf("ai compiler execution failed: %w", err)
	}

	// Convert compiled frames into Vue Flow nodes
	var flowNodes []interface{}
	xOffset := 60.0
	for idx, frame := range dsl.Frames {
		w := frame.Width
		if w <= 0 {
			if frame.Device == "mobile" {
				w = 375
			} else {
				w = 1024
			}
		}
		h := frame.Height
		if h <= 0 {
			if frame.Device == "mobile" {
				h = 812
			} else {
				h = 720
			}
		}

		rawHtmlContent := ""
		if frame.CodeExport != nil {
			rawHtmlContent = frame.CodeExport["html"]
		}

		frame.SyncCanonical(xOffset, 60)
		if dsl.DesignSpec != nil && frame.DesignState != nil {
			frame.DesignState.DesignSpec = dsl.DesignSpec
		}

		flowNode := map[string]interface{}{
			"id":       fmt.Sprintf("ui-frame-%d", idx+1),
			"type":     "ui_frame",
			"position": map[string]float64{"x": xOffset, "y": 60},
			"data": map[string]interface{}{
				// CANONICAL HIERARCHY
				"canvas":         frame.Canvas,
				"design_state":   frame.DesignState,
				"implementation": frame.Implementation,
				"audit":          frame.Audit,

				// Backward compatibility fields
				"device":           frame.Device,
				"title":            frame.Title,
				"width":            w,
				"height":           h,
				"theme":            frame.Theme,
				"raw_html":         rawHtmlContent,
				"sections":         frame.Sections,
				"code_export":      frame.CodeExport,
				"page_spec":        frame.PageSpec,
				"design_decisions": frame.DesignDecisions,
				"anti_slop_audit":  frame.AntiSlopAudit,
				"requirement_spec": frame.RequirementSpec,
				"validation":       frame.Validation,
			},
		}
		flowNodes = append(flowNodes, flowNode)
		xOffset += float64(w) + 80.0
	}

	nodesBytes, _ := json.Marshal(flowNodes)
	edgesBytes := json.RawMessage("[]")

	domainStr := "general"
	if dsl.RequirementSpec != nil && dsl.RequirementSpec.Context.Domain != nil {
		domainStr = *dsl.RequirementSpec.Context.Domain
	}
	primaryUserStr := ""
	if dsl.RequirementSpec != nil && dsl.RequirementSpec.Context.TargetUser != nil {
		primaryUserStr = *dsl.RequirementSpec.Context.TargetUser
	}
	primaryTaskStr := ""
	if dsl.RequirementSpec != nil {
		primaryTaskStr = dsl.RequirementSpec.Goals.Primary
	}

	metaBytes, _ := json.Marshal(map[string]interface{}{
		"target_device":   device,
		"theme":           theme,
		"theme_mode":      themeMode,
		"accent_color":    accentColor,
		"foundation":      foundation,
		"product_context": domainStr,
		"primary_user":    primaryUserStr,
		"primary_task":    primaryTaskStr,
	})

	projectTitle := dsl.Frames[0].Title
	if projectTitle == "" {
		projectTitle = "UI Design: " + prompt
	}

	project, err := s.projectModel.CreateWithMode(
		projectTitle,
		"ui_design",
		"ui_design",
		metaBytes,
		nodesBytes,
		edgesBytes,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to save ui project: %w", err)
	}

	// Version 1
	_, _ = s.versionModel.CreateVersion(project.ID, 1, "AI Design Compiler - Initial Version", nodesBytes, edgesBytes, nil)

	// Build rich assistant reply highlighting compiler results
	var assistantReplyBuilder strings.Builder
	assistantReplyBuilder.WriteString(fmt.Sprintf("✨ **Desain UI untuk '%s' berhasil dikompilasi (AI Design Compiler)**\n\n", projectTitle))

	if dsl.RequirementSpec != nil {
		assistantReplyBuilder.WriteString("📋 **Pilar 1: Requirement Specification (WHAT)**\n")
		assistantReplyBuilder.WriteString(fmt.Sprintf("- **Tipe & Kompleksitas**: %s (`%s`)\n", dsl.RequirementSpec.Page.Type, dsl.RequirementSpec.Page.Complexity))
		assistantReplyBuilder.WriteString(fmt.Sprintf("- **Tujuan Utama**: %s\n", dsl.RequirementSpec.Goals.Primary))
		if len(dsl.RequirementSpec.Requirements.Explicit) > 0 {
			assistantReplyBuilder.WriteString(fmt.Sprintf("- **Kebutuhan Eksplisit**: %s\n", strings.Join(dsl.RequirementSpec.Requirements.Explicit, ", ")))
		}
		assistantReplyBuilder.WriteString("\n")
	}

	if dsl.DesignSpec != nil {
		assistantReplyBuilder.WriteString("📐 **Pilar 2: Design Specification & Traceability (HOW)**\n")
		assistantReplyBuilder.WriteString(fmt.Sprintf("- **Tata Letak**: %s (%s)\n", dsl.DesignSpec.Layout.Type, dsl.DesignSpec.Layout.Container))
		assistantReplyBuilder.WriteString(fmt.Sprintf("- **Gaya Visual**: %s (Foundation: %s)\n", dsl.DesignSpec.Visual.Style, foundation))
		assistantReplyBuilder.WriteString(fmt.Sprintf("- **Sections Terpasang (%d)**: ", len(dsl.DesignSpec.Sections)))
		secNames := make([]string, 0, len(dsl.DesignSpec.Sections))
		for _, sec := range dsl.DesignSpec.Sections {
			secNames = append(secNames, fmt.Sprintf("%s [%s]", sec.Type, sec.Priority))
		}
		assistantReplyBuilder.WriteString(strings.Join(secNames, " → ") + "\n\n")
	}

	if dsl.Validation != nil {
		assistantReplyBuilder.WriteString("🛡️ **Pilar 3: UI Validator & Anti-Hallucination Audit (VERIFIED)**\n")
		assistantReplyBuilder.WriteString(fmt.Sprintf("- **Status Audit**: `%s`\n", dsl.Validation.Status))
		assistantReplyBuilder.WriteString(fmt.Sprintf("- **Anti-Hallucination**: %s\n", dsl.Validation.HallucinationCheck))
		assistantReplyBuilder.WriteString(fmt.Sprintf("- **Skor Kualitas**: Fidelity: %d/10 | Scope: %d/10 | Hierarchy: %d/10 | Simplicity: %d/10\n",
			dsl.Validation.Score.RequirementFidelity,
			dsl.Validation.Score.Scope,
			dsl.Validation.Score.Hierarchy,
			dsl.Validation.Score.Simplicity,
		))
		if len(dsl.Validation.StrippedSections) > 0 {
			assistantReplyBuilder.WriteString(fmt.Sprintf("- **Fitur Dipangkas (Anti-Bloat)**: %s\n", strings.Join(dsl.Validation.StrippedSections, ", ")))
		}
		assistantReplyBuilder.WriteString("\n")
	}

	assistantReplyBuilder.WriteString("Semua komponen telah dirender secara modular melalui registry komponen terverifikasi. Anda dapat meninjau tab Visual, Spesifikasi Lengkap, atau menyalin kode Vue/Tailwind!")

	assistantReply := assistantReplyBuilder.String()
	msg1, _ := s.messageModel.AppendMessage(project.ID, "user", req.Prompt, nil)
	msg2, _ := s.messageModel.AppendMessage(project.ID, "assistant", assistantReply, nil)

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

func (s *uiDesignService) IterateUIDesignWithChat(ctx context.Context, projectID, prompt string, targetedNodeIDs []string) (*dtos.ProjectResponse, error) {
	project, messages, err := s.projectModel.GetByID(projectID)
	if err != nil {
		return nil, fmt.Errorf("project not found: %w", err)
	}

	// 1. Parse existing nodes and find current primary UI Frame
	var currentNodes []map[string]interface{}
	_ = json.Unmarshal(project.CurrentNodes, &currentNodes)

	var currentFrame *dtos.UIFrameData
	var targetNodeIdx = -1

	for idx, n := range currentNodes {
		if n["type"] == "ui_frame" {
			if dataMap, ok := n["data"].(map[string]interface{}); ok {
				dataBytes, _ := json.Marshal(dataMap)
				var f dtos.UIFrameData
				if err := json.Unmarshal(dataBytes, &f); err == nil {
					currentFrame = &f
					targetNodeIdx = idx
					break
				}
			}
		}
	}

	// 2. RUN CHANGE ANALYZER
	changePlan, err := s.changeAnalyzer.AnalyzeChange(ctx, prompt, currentFrame)
	if err != nil {
		changePlan = &dtos.ChangePlanDTO{
			Request:        prompt,
			Classification: "style",
			Scope:          "component",
			Strategy:       "patch",
			Preserve:       []string{"layout", "typography", "content"},
			Regenerate:     false,
		}
	}

	var summary string
	var updatedNodes []interface{}

	// 3. DECIDE: PATCH VS REBUILD
	if changePlan.Strategy == "patch" && currentFrame != nil {
		// EXECUTE TARGETED PATCH
		patchedFrame, patchExplanation, patchErr := s.patchEngine.ApplyPatch(currentFrame, changePlan)
		if patchErr == nil {
			patchedFrameBytes, _ := json.Marshal(patchedFrame)
			var patchedMap map[string]interface{}
			_ = json.Unmarshal(patchedFrameBytes, &patchedMap)

			// Ensure raw_html and rawHtml are synchronized
			if rHtml, ok := patchedMap["raw_html"].(string); ok && rHtml != "" {
				patchedMap["rawHtml"] = rHtml
			}

			// Preserve existing node position and id
			existingNode := currentNodes[targetNodeIdx]
			existingNode["data"] = patchedMap

			for _, n := range currentNodes {
				updatedNodes = append(updatedNodes, n)
			}

			summary = fmt.Sprintf("✨ **[UPDATE] %s**\n\n%s", strings.ToUpper(changePlan.Classification), patchExplanation)
		}
	}

	// If rebuild or patch didn't produce nodes, run full compiler or LLM iteration
	if len(updatedNodes) == 0 {
		device := "web"
		foundation := "ramp"
		themeMode := "dark"
		accentColor := "#6366f1"

		if currentFrame != nil {
			if currentFrame.Device != "" {
				device = currentFrame.Device
			}
			if mode, ok := currentFrame.Theme["mode"].(string); ok && mode != "" {
				themeMode = mode
			}
			if prim, ok := currentFrame.Theme["primary"].(string); ok && prim != "" {
				accentColor = prim
			}
		}

		dsl, compileErr := s.compiler.Compile(ctx, prompt, device, foundation, themeMode, accentColor)
		if compileErr == nil && len(dsl.Frames) > 0 {
			dsl.ChangePlan = changePlan
			xOffset := 60.0
			for idx, f := range dsl.Frames {
				f.ChangePlan = changePlan
				frameBytes, _ := json.Marshal(f)
				var frameMap map[string]interface{}
				_ = json.Unmarshal(frameBytes, &frameMap)

				if rHtml, ok := frameMap["raw_html"].(string); ok && rHtml != "" {
					frameMap["rawHtml"] = rHtml
				}

				flowNode := map[string]interface{}{
					"id":       fmt.Sprintf("ui-frame-%d", idx+1),
					"type":     "ui_frame",
					"position": map[string]float64{"x": xOffset, "y": 60},
					"data":     frameMap,
				}
				updatedNodes = append(updatedNodes, flowNode)
				xOffset += float64(f.Width) + 80.0
			}
			summary = fmt.Sprintf("✨ **[REBUILD STRUKTURAL]** Desain antarmuka disusun ulang sesuai kebutuhan: %q.", prompt)
		} else {
			// Fallback: minimal update
			summary = "Desain antarmuka diperbarui sesuai permintaan."
			for _, n := range currentNodes {
				updatedNodes = append(updatedNodes, n)
			}
		}
	}

	nodesBytes, _ := json.Marshal(updatedNodes)
	edgesBytes := project.CurrentEdges

	if err := s.projectModel.UpdateGraph(project.ID, nodesBytes, edgesBytes); err != nil {
		return nil, fmt.Errorf("failed to update graph: %w", err)
	}

	// Record version
	versionNum := len(messages)/2 + 2
	_, _ = s.versionModel.CreateVersion(project.ID, versionNum, summary, nodesBytes, edgesBytes, nil)

	// Save messages
	var targetBytes json.RawMessage
	if len(targetedNodeIDs) > 0 {
		targetBytes, _ = json.Marshal(targetedNodeIDs)
	}
	msg1, _ := s.messageModel.AppendMessage(project.ID, "user", prompt, targetBytes)
	msg2, _ := s.messageModel.AppendMessage(project.ID, "assistant", summary, nil)

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
		ID:           project.ID,
		Title:        project.Title,
		DiagramType:  project.DiagramType,
		CurrentNodes: nodesBytes,
		CurrentEdges: edgesBytes,
		Nodes:        nodesBytes,
		Edges:        edgesBytes,
		Messages:     messageDTOs,
		CreatedAt:    project.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		UpdatedAt:    project.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}, nil
}
