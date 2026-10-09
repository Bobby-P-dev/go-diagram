package services

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"regexp"
	"strconv"
	"strings"

	"github.com/Bobby-P-dev/go-diagram.git/src/dtos"
	"github.com/Bobby-P-dev/go-diagram.git/src/entities"
	"github.com/Bobby-P-dev/go-diagram.git/src/middlewares"
	"github.com/Bobby-P-dev/go-diagram.git/src/models"
)

type UIDesignServiceInterface interface {
	GetTemplates(ctx context.Context) ([]entities.UITemplate, error)
	CreateProjectFromTemplate(ctx context.Context, templateID string) (*dtos.ProjectResponse, error)
	GenerateUIDesign(ctx context.Context, req dtos.CreateUIDesignRequest) (*dtos.ProjectResponse, error)
	GenerateUIDesignStream(ctx context.Context, req dtos.CreateUIDesignRequest, onEvent func(eventType string, data any)) (*dtos.ProjectResponse, error)
	IterateUIDesignWithChat(ctx context.Context, projectID, prompt string, targetedNodeIDs []string) (*dtos.ProjectResponse, error)
	IterateUIDesignWithTargetedChat(ctx context.Context, projectID string, req *dtos.ChatRequest) (*dtos.ProjectResponse, error)
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
	targetedPatcher *TargetedPatcher
}

// Preserve automatic styling until the design brief can interpret the user's prompt.
func resolveUIDesignThemeMode(req dtos.CreateUIDesignRequest) string {
	if mode := strings.ToLower(strings.TrimSpace(req.ThemeMode)); mode != "" {
		return mode
	}
	theme := strings.ToLower(strings.TrimSpace(req.Theme))
	if strings.Contains(theme, "light") {
		return "light"
	}
	if strings.Contains(theme, "dark") {
		return "dark"
	}
	return "auto"
}

func buildUIDesignPrompt(req dtos.CreateUIDesignRequest) string {
	prompt := strings.TrimSpace(req.Prompt)
	if prompt == "" {
		return ""
	}
	if tone := strings.TrimSpace(req.CustomTone); tone != "" {
		prompt += "\nRequested visual tone: " + tone
	}
	if context := strings.TrimSpace(req.ProductContext); context != "" {
		prompt += "\nProduct context: " + context
	}
	return prompt
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
		targetedPatcher: NewTargetedPatcher(aiService),
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

	userID := ""
	if user := middlewares.GetUserFromContext(ctx); user != nil {
		userID = user.ID
	}

	project, err := s.projectModel.CreateWithUser(
		userID,
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
	return s.GenerateUIDesignStream(ctx, req, nil)
}

func (s *uiDesignService) GenerateUIDesignStream(
	ctx context.Context,
	req dtos.CreateUIDesignRequest,
	onEvent func(eventType string, data any),
) (*dtos.ProjectResponse, error) {
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

	themeMode := resolveUIDesignThemeMode(req)

	accentColor := strings.TrimSpace(req.AccentColor)
	foundation := strings.ToLower(strings.TrimSpace(req.Foundation))
	productContext := strings.ToLower(strings.TrimSpace(req.ProductContext))

	prompt := buildUIDesignPrompt(req)
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

		userID := ""
		if user := middlewares.GetUserFromContext(ctx); user != nil {
			userID = user.ID
		}

		project, err := s.projectModel.CreateWithUser(
			userID,
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

		blankResp := &dtos.ProjectResponse{
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
		}
		if onEvent != nil {
			onEvent("complete", blankResp)
		}
		return blankResp, nil
	}

	if onEvent != nil {
		onEvent("status", map[string]string{
			"stage":   "analyzing",
			"message": "Menganalisis kebutuhan & arsitektur antarmuka...",
		})
	}

	var onChunk LLMStreamCallback
	if onEvent != nil {
		tokenCounter := 0
		charCounter := 0
		lastEmitted := 0
		var streamBuffer strings.Builder
		var detectedSections []string
		seenSections := make(map[string]bool)

		secRegex := regexp.MustCompile(`"type":\s*"([a-zA-Z0-9_\-]+)"`)
		secIdJsonRegex := regexp.MustCompile(`"id":\s*"(sec-[a-zA-Z0-9_\-]+)"`)
		idRegex := regexp.MustCompile(`data-rl-id="([a-zA-Z0-9_\-]+)"`)

		onChunk = func(delta string, tokens int) {
			charCounter += len(delta)
			streamBuffer.WriteString(delta)
			if tokens > 0 {
				tokenCounter = tokens
			} else {
				tokenCounter += (len(delta) + 3) / 4
			}

			bufStr := streamBuffer.String()

			// 1. Detect planned sections from JSON schema ("id": "sec-...") early in the stream
			for _, m := range secIdJsonRegex.FindAllStringSubmatch(bufStr, -1) {
				id := m[1]
				cleanName := strings.TrimPrefix(id, "sec-")
				if !seenSections[id] && !seenSections[cleanName] {
					seenSections[id] = true
					seenSections[cleanName] = true
					detectedSections = append(detectedSections, cleanName)
				}
			}

			// 2. Detect section types from JSON while filtering non-section words
			for _, m := range secRegex.FindAllStringSubmatch(bufStr, -1) {
				t := strings.ToLower(m[1])
				if t == "json_object" || t == "web" || t == "mobile" || t == "desktop" ||
					t == "light" || t == "dark" || t == "text" || t == "string" ||
					t == "number" || t == "email" || t == "password" || t == "date" ||
					t == "select" || t == "textarea" || t == "button" || t == "app" ||
					t == "page" || t == "input" || t == "simple" || t == "moderate" ||
					t == "complex" || t == "high" || t == "medium" || t == "low" {
					continue
				}
				if !seenSections[t] {
					seenSections[t] = true
					detectedSections = append(detectedSections, t)
				}
			}

			// 3. Detect sections from rendered HTML tags
			for _, m := range idRegex.FindAllStringSubmatch(bufStr, -1) {
				id := m[1]
				if strings.HasPrefix(id, "sec-") {
					cleanName := strings.TrimPrefix(id, "sec-")
					if !seenSections[id] && !seenSections[cleanName] {
						seenSections[id] = true
						seenSections[cleanName] = true
						detectedSections = append(detectedSections, cleanName)
					}
				}
			}

			if tokenCounter-lastEmitted >= 60 || strings.Contains(delta, `data-rl-id="sec-`) {
				lastEmitted = tokenCounter
				msg := "Menyusun komponen antarmuka & kode Tailwind..."
				currentSec := ""
				if len(detectedSections) > 0 {
					currentSec = detectedSections[len(detectedSections)-1]
				}
				if m := idRegex.FindStringSubmatch(delta); len(m) > 1 && strings.HasPrefix(m[1], "sec-") {
					currentSec = strings.TrimPrefix(m[1], "sec-")
				}

				if strings.Contains(delta, "nav") || strings.Contains(delta, "header") || strings.Contains(currentSec, "nav") {
					msg = "Merancang struktur navigasi & header..."
				} else if strings.Contains(delta, "hero") || strings.Contains(currentSec, "hero") {
					msg = "Menyusun hero section & headline..."
				} else if strings.Contains(delta, "chat") || strings.Contains(delta, "message") || strings.Contains(delta, "workspace") {
					msg = "Merakit workspace pesan & thread obrolan..."
				} else if strings.Contains(delta, "card") || strings.Contains(delta, "grid") || strings.Contains(delta, "sidebar") {
					msg = "Merakit komponen panel & bilah menu..."
				} else if strings.Contains(delta, "footer") || strings.Contains(currentSec, "footer") {
					msg = "Menyelesaikan footer & struktur penutup..."
				}

				onEvent("progress", map[string]interface{}{
					"stage":          "generating",
					"tokens":         tokenCounter,
					"chars":          charCounter,
					"message":        msg,
					"sections":       detectedSections,
					"active_section": currentSec,
				})
			}
		}
	}

	// Compile UI Design through the 11-Layer AI Design Compiler
	dsl, err := s.compiler.CompileStream(ctx, prompt, device, foundation, themeMode, accentColor, onChunk)
	if err != nil {
		return nil, fmt.Errorf("ai compiler execution failed: %w", err)
	}

	if onEvent != nil {
		onEvent("status", map[string]string{
			"stage":   "compiling",
			"message": "Memvalidasi komponen & menyiapkan kanvas...",
		})
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

	userID := ""
	if user := middlewares.GetUserFromContext(ctx); user != nil {
		userID = user.ID
	}

	project, err := s.projectModel.CreateWithUser(
		userID,
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

	resp := &dtos.ProjectResponse{
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
	}

	if onEvent != nil {
		onEvent("complete", resp)
	}

	return resp, nil
}

func (s *uiDesignService) IterateUIDesignWithChat(ctx context.Context, projectID, prompt string, targetedNodeIDs []string) (*dtos.ProjectResponse, error) {
	return s.IterateUIDesignWithTargetedChat(ctx, projectID, &dtos.ChatRequest{
		Prompt:          prompt,
		TargetedNodeIDs: targetedNodeIDs,
	})
}

func (s *uiDesignService) IterateUIDesignWithTargetedChat(ctx context.Context, projectID string, req *dtos.ChatRequest) (*dtos.ProjectResponse, error) {
	userID := ""
	if user := middlewares.GetUserFromContext(ctx); user != nil {
		userID = user.ID
	}

	project, messages, err := s.projectModel.GetByIDScoped(projectID, userID, "")
	if err != nil {
		return nil, fmt.Errorf("project not found or unauthorized: %w", err)
	}

	prompt := strings.TrimSpace(req.Prompt)
	if prompt == "" {
		prompt = strings.TrimSpace(req.Message)
	}
	if prompt == "" {
		prompt = strings.TrimSpace(req.Instruction)
	}

	targetRef := req.Target
	if targetRef == nil && req.SelectedComponentID != "" {
		targetRef = &dtos.TargetElementRefDTO{
			Type: "component",
			ID:   req.SelectedComponentID,
		}
	}

	// 1. Parse existing nodes and find current primary UI Frame
	var currentNodes []map[string]interface{}
	_ = json.Unmarshal(project.CurrentNodes, &currentNodes)

	var currentFrame *dtos.UIFrameData
	var targetNodeIdx = -1

	// 1.1 Match by explicit TargetedNodeIDs from request
	if len(req.TargetedNodeIDs) > 0 {
		targetID := req.TargetedNodeIDs[0]
		for idx, n := range currentNodes {
			if n["type"] == "ui_frame" && fmt.Sprintf("%v", n["id"]) == targetID {
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
	}

	// 1.2 Match by Target.FrameID if provided
	if targetNodeIdx == -1 && targetRef != nil && targetRef.FrameID != "" {
		for idx, n := range currentNodes {
			if n["type"] == "ui_frame" && fmt.Sprintf("%v", n["id"]) == targetRef.FrameID {
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
	}

	// 1.3 If user asks to switch to web (e.g. "ubah dari mobile ke bentuk web"), prioritize targeting a mobile frame
	lowerPrompt := strings.ToLower(prompt)
	isSwitchingToWeb := strings.Contains(lowerPrompt, "web") && (strings.Contains(lowerPrompt, "mobile") || strings.Contains(lowerPrompt, "hp"))
	if targetNodeIdx == -1 && isSwitchingToWeb {
		for idx, n := range currentNodes {
			if n["type"] == "ui_frame" {
				if dataMap, ok := n["data"].(map[string]interface{}); ok {
					if dev, ok := dataMap["device"].(string); ok && (dev == "mobile" || dev == "smartphone") {
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
		}
	}

	// 1.4 Fallback: Find the first valid ui_frame
	if targetNodeIdx == -1 {
		for idx, n := range currentNodes {
			if n["type"] == "ui_frame" {
				if dataMap, ok := n["data"].(map[string]interface{}); ok {
					dataBytes, _ := json.Marshal(dataMap)
					var f dtos.UIFrameData
					if err := json.Unmarshal(dataBytes, &f); err == nil {
						currentFrame = &f
						targetNodeIdx = idx
						break
					} else {
						log.Printf("[UI_FRAME UNMARSHAL ERROR] Failed to unmarshal node %d into UIFrameData: %v", idx, err)
					}
				}
			}
		}
	}

	var oldHtml string
	if currentFrame != nil {
		oldHtml = currentFrame.RawHtml
		if oldHtml == "" && currentFrame.CodeExport != nil {
			oldHtml = currentFrame.CodeExport["html"]
		}
		if oldHtml == "" && currentFrame.Implementation != nil {
			oldHtml = currentFrame.Implementation.Source.HTML
		}
	}

	// 2. RUN CHANGE ANALYZER WITH STRICT LOCALITY
	changePlan, err := s.changeAnalyzer.AnalyzeTargetedChange(ctx, prompt, targetRef, req.SelectionContext, currentFrame)
	if err != nil {
		changePlan = &dtos.ChangePlanDTO{
			Request:        prompt,
			Classification: "style",
			Scope:          "component",
			Strategy:       "component_patch",
			Preserve:       []string{"layout", "typography", "content"},
			Regenerate:     false,
		}
	}

	// 3.0 APPEND A NEW FRAME (INSERT_FRAME): add a new page/screen to the project
	// while preserving every existing frame node untouched.
	if changePlan.Operation == dtos.OpInsertFrame {
		return s.appendNewUIFrame(ctx, project, messages, currentNodes, changePlan, prompt)
	}

	// 3.0.1 DEVICE / VIEWPORT MODE SWITCH: Convert frame between mobile and web/desktop
	// while preserving every existing frame node untouched!
	if changePlan.Operation == dtos.OpDeviceModeSwitch || changePlan.Strategy == "device_switch" || changePlan.Target.Type == "device" {
		return s.switchFrameDevice(ctx, project, messages, currentNodes, targetNodeIdx, currentFrame, changePlan, prompt)
	}

	// 3.0.2 DELETE FRAME: Remove a screen from the canvas
	if changePlan.Operation == dtos.OpDeleteFrame || changePlan.Strategy == "delete_frame" {
		return s.deleteFrame(ctx, project, messages, currentNodes, targetNodeIdx, currentFrame, prompt)
	}

	var summary string
	var updatedNodes []interface{}

	// 3. DECIDE: SURGICAL TARGETED PATCH VS REBUILD
	isLocalStrategy := changePlan.Strategy == "component_patch" ||
		changePlan.Strategy == "section_patch" ||
		changePlan.Strategy == "patch" ||
		changePlan.Strategy == "token_update" ||
		changePlan.Strategy == "insert_section" ||
		changePlan.Strategy == "delete_section" ||
		changePlan.Operation == dtos.OpComponentPatch ||
		changePlan.Operation == dtos.OpSectionPatch ||
		changePlan.Operation == dtos.OpInsertSection ||
		changePlan.Operation == dtos.OpDeleteSection

	if isLocalStrategy && currentFrame != nil && targetNodeIdx >= 0 {
		// Attempt surgical targeted patch
		patchedFrame, patchExplanation, patchErr := s.targetedPatcher.ApplyTargetedPatch(ctx, currentFrame, changePlan)
		if patchErr != nil || patchedFrame == nil {
			log.Printf("[TARGETED PATCH FAILED]: %v (falling back to legacy patch engine)", patchErr)
			// Fallback to legacy patch engine if targeted patcher encountered an issue
			patchedFrame, patchExplanation, patchErr = s.patchEngine.ApplyPatch(currentFrame, changePlan)
			if patchErr != nil {
				log.Printf("[LEGACY PATCH ENGINE FAILED]: %v", patchErr)
			}
		}

		if patchErr == nil && patchedFrame != nil {
			patchedFrameBytes, _ := json.Marshal(patchedFrame)
			var patchedMap map[string]interface{}
			_ = json.Unmarshal(patchedFrameBytes, &patchedMap)

			// Ensure raw_html, rawHtml, and implementation.source.html are synchronized
			if rHtml, ok := patchedMap["raw_html"].(string); ok && rHtml != "" {
				patchedMap["rawHtml"] = rHtml
				if implMap, ok := patchedMap["implementation"].(map[string]interface{}); ok {
					if srcMap, ok := implMap["source"].(map[string]interface{}); ok {
						srcMap["html"] = rHtml
					}
				}
			}

			// Preserve existing node position and id
			existingNode := currentNodes[targetNodeIdx]
			existingNode["data"] = patchedMap

			for _, n := range currentNodes {
				updatedNodes = append(updatedNodes, n)
			}

			summary = fmt.Sprintf("✨ **[%s]** %s", strings.ToUpper(changePlan.Strategy), patchExplanation)
		}
	}

	// 4. FULL REBUILD ONLY IF EXPLICITLY REQUESTED
	if len(updatedNodes) == 0 {
		if changePlan.Strategy == "rebuild" || changePlan.Regenerate || changePlan.Operation == dtos.OpFullReplace {
			device := "web"
			foundation := ""
			themeMode := ""
			accentColor := ""

			if changePlan.Target.Device != "" {
				device = changePlan.Target.Device
			} else if currentFrame != nil && currentFrame.Device != "" {
				device = currentFrame.Device
			}

			if currentFrame != nil {
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
				if len(currentNodes) > 1 && targetNodeIdx >= 0 && len(dsl.Frames) == 1 {
					// Replace only the targeted frame, preserve all other frames!
					f := dsl.Frames[0]
					f.ChangePlan = changePlan
					frameBytes, _ := json.Marshal(f)
					var frameMap map[string]interface{}
					_ = json.Unmarshal(frameBytes, &frameMap)
					if rHtml, ok := frameMap["raw_html"].(string); ok && rHtml != "" {
						frameMap["rawHtml"] = rHtml
					}
					for i, n := range currentNodes {
						if i == targetNodeIdx {
							updatedNodes = append(updatedNodes, map[string]interface{}{
								"id":       n["id"],
								"type":     "ui_frame",
								"position": n["position"],
								"data":     frameMap,
							})
						} else {
							updatedNodes = append(updatedNodes, n)
						}
					}
				} else {
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
				}
				summary = fmt.Sprintf("✨ **[REBUILD STRUKTURAL]** Desain antarmuka disusun ulang sesuai kebutuhan: %q.", prompt)
			}
		}

		// Safe Locality Guard: If still empty, preserve current layout completely
		if len(updatedNodes) == 0 {
			summary = fmt.Sprintf("⚠️ **[NO_CHANGE]** Tidak ada modifikasi yang diterapkan untuk instruksi: %q. Pastikan elemen target dipilih dengan benar.", prompt)
			for _, n := range currentNodes {
				updatedNodes = append(updatedNodes, n)
			}
		}
	}

	// Validate duplicate IDs across updated nodes
	validator := NewDuplicateValidator()
	for _, n := range updatedNodes {
		if nodeMap, ok := n.(map[string]interface{}); ok {
			if dataMap, ok := nodeMap["data"].(map[string]interface{}); ok {
				if rHtml, ok := dataMap["raw_html"].(string); ok && rHtml != "" {
					if dupErr := validator.ValidateHTMLDuplicateIDs(rHtml); dupErr != nil {
						log.Printf("[VALIDATOR WARNING] %v in targeted chat result", dupErr)
					}
				}
			}
		}
	}

	log.Printf("[STATE RECONCILIATION] op=%s target=%s before_nodes=%d after_nodes=%d", changePlan.Operation, changePlan.Target.ID, len(currentNodes), len(updatedNodes))

	var newHtml string
	for _, n := range updatedNodes {
		if nodeMap, ok := n.(map[string]interface{}); ok {
			if dataMap, ok := nodeMap["data"].(map[string]interface{}); ok {
				if rHtml, ok := dataMap["raw_html"].(string); ok && rHtml != "" {
					newHtml = rHtml
					break
				}
			}
		}
	}

	latestVer, _ := s.versionModel.GetLatestVersionNumber(project.ID)
	if latestVer <= 0 {
		latestVer = 1
	}

	// No-Op Detector: Check if targeted edit produced identical markup
	isNoOp := isLocalStrategy && len(currentNodes) == len(updatedNodes) && strings.TrimSpace(newHtml) != "" && strings.TrimSpace(newHtml) == strings.TrimSpace(oldHtml)
	if isNoOp {
		summary = fmt.Sprintf("⚠️ **[NO_CHANGE]** Tidak ada modifikasi kode yang dihasilkan untuk target `%s`. Instruksi tidak mengubah markup yang ada.", changePlan.Target.ID)
	}

	versionNum := latestVer
	if !isNoOp {
		versionNum = latestVer + 1
	}

	// Synchronize version and implementation into node data
	for _, n := range updatedNodes {
		if nodeMap, ok := n.(map[string]interface{}); ok {
			if dataMap, ok := nodeMap["data"].(map[string]interface{}); ok {
				dataMap["version"] = versionNum
				if rHtml, ok := dataMap["raw_html"].(string); ok && rHtml != "" {
					if implMap, ok := dataMap["implementation"].(map[string]interface{}); ok {
						implMap["version"] = versionNum
						if srcMap, ok := implMap["source"].(map[string]interface{}); ok {
							srcMap["html"] = rHtml
						}
					}
				}
			}
		}
	}

	nodesBytes, _ := json.Marshal(updatedNodes)
	edgesBytes := project.CurrentEdges

	if !isNoOp {
		if err := s.projectModel.UpdateGraph(project.ID, nodesBytes, edgesBytes); err != nil {
			return nil, fmt.Errorf("failed to update graph: %w", err)
		}
		_, _ = s.versionModel.CreateVersion(project.ID, versionNum, summary, nodesBytes, edgesBytes, nil)
	} else {
		log.Printf("[NO-OP DETECTOR] Skipped creating redundant version for project %s: zero diff detected", project.ID)
	}

	// Save messages
	var targetBytes json.RawMessage
	if len(req.TargetedNodeIDs) > 0 {
		targetBytes, _ = json.Marshal(req.TargetedNodeIDs)
	} else if req.Target != nil {
		targetBytes, _ = json.Marshal(req.Target)
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

// appendNewUIFrame adds a brand-new page/frame to an existing UI Design project
// without touching any existing frame nodes. It compiles one fresh frame through
// the single-call pipeline, appends it as a new ui_frame node, persists a new
// version, and returns the updated project state containing ALL frames.
func (s *uiDesignService) appendNewUIFrame(
	ctx context.Context,
	project *entities.Project,
	messages []entities.ChatMessage,
	currentNodes []map[string]interface{},
	changePlan *dtos.ChangePlanDTO,
	userPrompt string,
) (*dtos.ProjectResponse, error) {
	// 1. Determine the next frame index + x offset from existing ui_frame nodes.
	maxIndex := 0
	maxX := 60.0
	foundFrame := false
	device := strings.ToLower(strings.TrimSpace(changePlan.Target.Device))
	if device == "" {
		device = "web"
	}
	pageType := strings.TrimSpace(changePlan.Target.SectionID)

	for _, n := range currentNodes {
		if n["type"] != "ui_frame" {
			continue
		}
		foundFrame = true
		if id, ok := n["id"].(string); ok {
			if idx, err := strconv.Atoi(strings.TrimPrefix(id, "ui-frame-")); err == nil && idx > maxIndex {
				maxIndex = idx
			}
		}
		if pos, ok := n["position"].(map[string]interface{}); ok {
			if px, ok := pos["x"].(float64); ok && px >= maxX {
				maxX = px
			}
		}
		if data, ok := n["data"].(map[string]interface{}); ok {
			if w, ok := data["width"].(float64); ok {
				maxX = maxX + w
			}
		}
	}
	nextIndex := maxIndex + 1
	xOffset := maxX + 80.0
	if !foundFrame {
		xOffset = 60.0
	}

	// 2. Derive shared design tokens, navigation/sidebar structure, product domain, and branding from the first frame.
	themeMode := ""
	accentColor := ""
	firstFrameTitle := ""
	var navSnippet string
	var footerSnippet string
	var isSidebarLayout bool
	var productDomain string
	var targetUser string
	var pagePurpose string
	var brandSnippet string

	if len(currentNodes) > 0 {
		if currentFrameData, ok := currentNodes[0]["data"].(map[string]interface{}); ok {
			if t, ok := currentFrameData["title"].(string); ok {
				firstFrameTitle = t
			}
			if m, ok := currentFrameData["theme"].(map[string]interface{}); ok {
				if mode, ok := m["mode"].(string); ok {
					themeMode = mode
				}
				if prim, ok := m["primary"].(string); ok {
					accentColor = prim
				}
			}
			if reqSpec, ok := currentFrameData["requirement_spec"].(map[string]interface{}); ok {
				if ctxMap, ok := reqSpec["context"].(map[string]interface{}); ok {
					if d, ok := ctxMap["domain"].(string); ok && d != "" {
						productDomain = d
					}
					if u, ok := ctxMap["target_user"].(string); ok && u != "" {
						targetUser = u
					}
				}
			}
			if pageSpec, ok := currentFrameData["page_spec"].(map[string]interface{}); ok {
				if p, ok := pageSpec["purpose"].(string); ok && p != "" {
					pagePurpose = p
				}
			}

			firstHtml, _ := currentFrameData["raw_html"].(string)
			if firstHtml == "" {
				firstHtml, _ = currentFrameData["rawHtml"].(string)
			}
			if firstHtml != "" {
				// Search for any navigation container (sidebar, topbar, header)
				for _, navId := range []string{"sec-sidebar", "sec-top-navigation", "sec-header", "sec-navbar-global", "sec-topbar", "navbar", "header", "aside"} {
					if snip, _, _, err := findElementSnippet(firstHtml, navId); err == nil && snip != "" {
						navSnippet = snip
						if navId == "sec-sidebar" || navId == "aside" || strings.Contains(snip, "<aside") {
							isSidebarLayout = true
						}
						break
					}
				}
				brandSnippet = extractBrandSnippet(firstHtml, navSnippet)
				for _, fId := range []string{"sec-footer", "footer"} {
					if snip, _, _, err := findElementSnippet(firstHtml, fId); err == nil && snip != "" {
						footerSnippet = snip
						break
					}
				}
			}
		}
	}

	// Extract original user prompt / concept from project chat messages
	var initialPromptConcept string
	if len(messages) > 0 {
		for _, m := range messages {
			if m.Role == "user" && strings.TrimSpace(m.Content) != "" {
				initialPromptConcept = strings.TrimSpace(m.Content)
				if len(initialPromptConcept) > 400 {
					initialPromptConcept = initialPromptConcept[:400] + "..."
				}
				break
			}
		}
	}

	// Detect if new screen is a login or authentication page
	lowerPrompt := strings.ToLower(userPrompt)
	isAuthPage := strings.Contains(lowerPrompt, "login") ||
		strings.Contains(lowerPrompt, "masuk") ||
		strings.Contains(lowerPrompt, "auth") ||
		strings.Contains(lowerPrompt, "sign in") ||
		strings.Contains(lowerPrompt, "signin") ||
		strings.Contains(lowerPrompt, "register") ||
		strings.Contains(lowerPrompt, "daftar") ||
		strings.Contains(lowerPrompt, "sign up") ||
		pageType == "login" || pageType == "auth" || pageType == "register"

	// Extract clean brand text if brand snippet exists
	brandText := ""
	if brandSnippet != "" {
		reStrip := regexp.MustCompile(`<[^>]*>`)
		t := strings.TrimSpace(reStrip.ReplaceAllString(brandSnippet, " "))
		brandText = strings.Join(strings.Fields(t), " ")
	}

	// 3. Build the new-page synthesis prompt with strict Screen 1 brand & domain consistency.
	newPrompt := strings.TrimSpace(userPrompt)
	if pageType != "" && pageType != "new_page" && !strings.Contains(strings.ToLower(newPrompt), pageType) {
		if newPrompt != "" {
			newPrompt = "Buat halaman UI: " + pageType + ". " + newPrompt
		} else {
			newPrompt = "Buat halaman UI: " + pageType
		}
	}

	var consistencyPrompt strings.Builder
	consistencyPrompt.WriteString(newPrompt)
	consistencyPrompt.WriteString("\n\nPANDUAN KONSISTENSI MULTI-SCREEN (WAJIB MENGIKUTI SCREEN 1 SECARA HARMONIS):\n")

	if productDomain != "" {
		consistencyPrompt.WriteString(fmt.Sprintf("- Domain Sistem / Produk: %s\n", productDomain))
	}
	if targetUser != "" {
		consistencyPrompt.WriteString(fmt.Sprintf("- Target Pengguna: %s\n", targetUser))
	}
	if pagePurpose != "" {
		consistencyPrompt.WriteString(fmt.Sprintf("- Tujuan Ekosistem Platform: %s\n", pagePurpose))
	}
	if initialPromptConcept != "" {
		consistencyPrompt.WriteString(fmt.Sprintf("- Konsep Platform Dasar: %s\n", initialPromptConcept))
	}

	if brandText != "" {
		consistencyPrompt.WriteString(fmt.Sprintf("- Nama Brand / Organisasi di Screen 1: %s\n", brandText))
	} else if firstFrameTitle != "" {
		consistencyPrompt.WriteString(fmt.Sprintf("- Nama Brand / Workspace di Screen 1: %s\n", firstFrameTitle))
	}

	if brandSnippet != "" {
		consistencyPrompt.WriteString(fmt.Sprintf("- LOGO & IDENTITAS BRAND SCREEN 1 (WAJIB GUNAKAN STRUKTUR, KELAS TAILWIND, ICON SVG, DAN TEKS INI):\n%s\n", brandSnippet))
	}

	if themeMode != "" || accentColor != "" {
		consistencyPrompt.WriteString(fmt.Sprintf("- Tema Visual: Mode %s, Aksen Warna %s. Pertahankan tone warna latar, border, dan font yang identik dengan Screen 1.\n", themeMode, accentColor))
	}

	consistencyPrompt.WriteString(`- ATURAN KETAT ANTI-SLOP (PURE ONLY CONTEXT & ZERO HALLUCINATION):
  1. DILARANG KERAS membuat seksi promosi marketing klise (testimoni fiktif, ulasan rating bintang, FAQ template, tabel pricing, logo bar 'Trusted by') KECUALI diminta secara eksplisit.
  2. Halaman baru harus menjadi bagian fungsional nyata dari domain sistem Screen 1, bukan landing page promosi SaaS.
  3. DILARANG menggunakan gradien ungu-indigo klise AI (from-purple-600 to-indigo-600), background glow orbs, atau 3 kartu seragam copy-paste.
  4. DILARANG menggunakan kata-kata AI klise (supercharge, seamless, unlock, revolutionize). Gunakan bahasa manusia konkret.
`)

	if isAuthPage {
		effectiveBrand := brandText
		if effectiveBrand == "" {
			effectiveBrand = firstFrameTitle
		}
		consistencyPrompt.WriteString(fmt.Sprintf(`- INSTRUKSI KHUSUS HALAMAN LOGIN / AUTHENTICATION:
  1. PERINGATAN KERAS ANTI-HALLUCINATION: DILARANG KERAS membuat navbar marketing SaaS publik (seperti 'Solutions', 'Pricing', 'Infrastructure', 'Cluster Status', 'Documentation', 'Enterprise Helpdesk', 'Auth Gateway v2.4', 'API Status', dsb).
  2. Ini adalah portal autentikasi internal resmi karyawan/organisasi untuk %s (%s).
  3. IDENTITAS BRAND & JUDUL: Wajib menampilkan logo dan nama brand '%s' secara persis (JANGAN menggunakan nama brand atau portal SaaS acak). Judul halaman harus menggunakan brand '%s'.
  4. Header / Navigasi: Minimalis atau terintegrasi langsung di atas form login. Hanya tampilkan logo resmi Screen 1 dan status keamanan/lingkungan (misal badge 'SSO Protected', 'Corporate Gateway', atau 'Internal Network').
  5. Form Login Utama:
     - Card login yang presisi, clean, dan profesional di tengah layar (atau split layout minimalis).
     - Opsi login Corporate SSO ('Sign in with Corporate SSO' / SAML / Okta) sebagai opsi utama karyawan.
     - Input Corporate Email (@perusahaan) dan Password dengan styling Tailwind yang halus dan serasi.
     - Opsi 'Ingat saya di perangkat ini' dan link 'Bantuan Akses IT / Hubungi Admin'.
  6. Catatan Kepatuhan: Sertakan disclaimer keamanan korporat ringkas di bagian bawah card login.
  7. Skema Warna & Estetika: Latar belakang, card, dan warna aksen (%s) harus 100%%%% serasi dengan Screen 1.
  8. ATURAN STATIC MARKUP: DILARANG KERAS menyertakan event handler JavaScript (seperti onsubmit="return false", onclick, onchange) dan DILARANG menggunakan href="javascript:void(0)". Gunakan tag <form> standar dan href="#" atau <button type="button">.
`, effectiveBrand, productDomain, effectiveBrand, effectiveBrand, accentColor))
	} else {
		if isSidebarLayout && navSnippet != "" && len(navSnippet) < 3000 {
			consistencyPrompt.WriteString(fmt.Sprintf("- LAYOUT SIDEBAR: Screen 1 menggunakan Sidebar navigasi. Pertahankan Sidebar yang sama persis dengan Screen 1 (logo, menu, styling) dan aktifkan item menu yang relevan dengan halaman baru ini:\n%s\n", navSnippet))
		} else if navSnippet != "" && len(navSnippet) < 2500 {
			consistencyPrompt.WriteString(fmt.Sprintf("- LAYOUT HEADER: Pertahankan header/navbar yang sama persis dengan Screen 1, dengan status tab navigasi aktif mengarah ke halaman baru ini:\n%s\n", navSnippet))
		}
		if footerSnippet != "" && len(footerSnippet) < 1500 {
			consistencyPrompt.WriteString(fmt.Sprintf("- FOOTER: Gunakan footer yang seragam dengan Screen 1:\n%s\n", footerSnippet))
		}
	}

	dsl, err := s.compiler.Compile(ctx, consistencyPrompt.String(), device, "", themeMode, accentColor)
	if err != nil {
		return nil, fmt.Errorf("insert frame compile failed: %w", err)
	}
	if len(dsl.Frames) == 0 {
		return nil, fmt.Errorf("insert frame produced no frames")
	}

	// 4. Convert the freshly compiled frame into a ui_frame node.
	frame := dsl.Frames[0]
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

	newFrameNode := map[string]interface{}{
		"id":       fmt.Sprintf("ui-frame-%d", nextIndex),
		"type":     "ui_frame",
		"position": map[string]float64{"x": xOffset, "y": 60},
		"data": map[string]interface{}{
			"canvas":           frame.Canvas,
			"design_state":     frame.DesignState,
			"implementation":   frame.Implementation,
			"audit":            frame.Audit,
			"device":           frame.Device,
			"title":            frame.Title,
			"width":            w,
			"height":           h,
			"theme":            frame.Theme,
			"raw_html":         rawHtmlContent,
			"rawHtml":          rawHtmlContent,
			"sections":         frame.Sections,
			"code_export":      frame.CodeExport,
			"page_spec":        frame.PageSpec,
			"design_decisions": frame.DesignDecisions,
			"anti_slop_audit":  frame.AntiSlopAudit,
			"requirement_spec": frame.RequirementSpec,
			"validation":       frame.Validation,
		},
	}
	// Keep canonical implementation.source.html in sync.
	if implState, ok := newFrameNode["data"].(map[string]interface{}); ok {
		if implMap, ok := implState["implementation"].(map[string]interface{}); ok {
			if srcMap, ok := implMap["source"].(map[string]interface{}); ok {
				srcMap["html"] = rawHtmlContent
			}
		}
	}

	// 5. Append while preserving every existing node.
	updatedNodes := make([]interface{}, 0, len(currentNodes)+1)
	for _, n := range currentNodes {
		updatedNodes = append(updatedNodes, n)
	}
	updatedNodes = append(updatedNodes, newFrameNode)

	// 6. Version + persist + messages (mirror IterateUIDesignWithTargetedChat tail).
	latestVer, _ := s.versionModel.GetLatestVersionNumber(project.ID)
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

	if err := s.projectModel.UpdateGraph(project.ID, nodesBytes, edgesBytes); err != nil {
		return nil, fmt.Errorf("failed to update graph: %w", err)
	}
	summary := fmt.Sprintf("✨ **[INSERT_FRAME]** Halaman UI baru '%s' (device: %s) ditambahkan di samping frame yang ada.", frame.Title, device)
	_, _ = s.versionModel.CreateVersion(project.ID, versionNum, summary, nodesBytes, edgesBytes, nil)

	msg1, _ := s.messageModel.AppendMessage(project.ID, "user", userPrompt, nil)
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

// deleteFrame removes an existing frame from the project canvas
func (s *uiDesignService) deleteFrame(
	ctx context.Context,
	project *entities.Project,
	messages []entities.ChatMessage,
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

	latestVer, _ := s.versionModel.GetLatestVersionNumber(project.ID)
	if latestVer <= 0 {
		latestVer = 1
	}
	versionNum := latestVer + 1
	if err := s.projectModel.UpdateGraph(project.ID, nodesBytes, edgesBytes); err != nil {
		return nil, fmt.Errorf("failed to update graph: %w", err)
	}

	summary := fmt.Sprintf("🗑️ **[DELETE_FRAME]** Screen '%s' berhasil dihapus dari kanvas.", deletedTitle)
	_, _ = s.versionModel.CreateVersion(project.ID, versionNum, summary, nodesBytes, edgesBytes, nil)

	msg1, _ := s.messageModel.AppendMessage(project.ID, "user", userPrompt, nil)
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

// switchFrameDevice converts an existing frame between mobile and web/desktop viewports
// while preserving EVERY other frame on the canvas untouched.
func (s *uiDesignService) switchFrameDevice(
	ctx context.Context,
	project *entities.Project,
	messages []entities.ChatMessage,
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

		newHtml, err := s.aiService.CallLLMText(sysPrompt, nil, userPrompt)
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
	latestVer, _ := s.versionModel.GetLatestVersionNumber(project.ID)
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

	if err := s.projectModel.UpdateGraph(project.ID, nodesBytes, edgesBytes); err != nil {
		return nil, fmt.Errorf("failed to update graph: %w", err)
	}

	summary := fmt.Sprintf("✨ **[VIEWPORT_SWITCH]** Frame '%s' berhasil diubah dari %s ke %s dengan konten dan tata letak responsif yang terjaga.",
		targetFrame.Title, strings.ToUpper(sourceDev), strings.ToUpper(targetDev))

	_, _ = s.versionModel.CreateVersion(project.ID, versionNum, summary, nodesBytes, edgesBytes, nil)

	msg1, _ := s.messageModel.AppendMessage(project.ID, "user", prompt, nil)
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

// extractBrandSnippet finds the logo/brand element in the HTML
func extractBrandSnippet(html, navSnippet string) string {
	// 1. Try finding explicit brand/logo component IDs
	for _, id := range []string{"comp-brand-logo", "cmp-logo", "brand-logo", "logo", "comp-logo"} {
		if snip, _, _, err := findElementSnippet(html, id); err == nil && snip != "" {
			return snip
		}
	}

	// 2. Search inside navSnippet for the container enclosing the SVG icon AND brand text
	if navSnippet != "" {
		svgIdx := strings.Index(navSnippet, "<svg")
		if svgIdx != -1 {
			openDiv := strings.LastIndex(navSnippet[:svgIdx], "<div")
			if openDiv != -1 {
				// Check parent div if openDiv is just the icon wrapper
				parentDiv := strings.LastIndex(navSnippet[:openDiv], "<div")
				if parentDiv != -1 {
					if parentSnip, _, _, err := findBalancedTagFromIndex(navSnippet, parentDiv); err == nil && parentSnip != "" && len(parentSnip) <= 1500 {
						// Ensure this parent actually contains brand name text
						textOnly := strings.TrimSpace(regexp.MustCompile(`<[^>]*>`).ReplaceAllString(parentSnip, ""))
						if len(textOnly) > 0 {
							return parentSnip
						}
					}
				}
				// If parentDiv wasn't the brand wrapper, try openDiv directly
				if snip, _, _, err := findBalancedTagFromIndex(navSnippet, openDiv); err == nil && snip != "" && len(snip) <= 1200 {
					textOnly := strings.TrimSpace(regexp.MustCompile(`<[^>]*>`).ReplaceAllString(snip, ""))
					if len(textOnly) > 0 {
						return snip
					}
				}
			}
		}

		// Fallback: first div inside navSnippet up to 2500 chars if it has text
		startDiv := strings.Index(navSnippet, "<div")
		if startDiv != -1 {
			if snip, _, _, err := findBalancedTagFromIndex(navSnippet, startDiv); err == nil && snip != "" && len(snip) <= 2500 {
				return snip
			}
		}
	}

	return ""
}


