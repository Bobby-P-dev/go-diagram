package service

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/Bobby-P-dev/go-diagram.git/internal/modules/uidesign"
	dtos "github.com/Bobby-P-dev/go-diagram.git/internal/modules/uidesign/dto"
	"regexp"
	"strings"
)

func (s *uiDesignService) GenerateUIDesign(ctx context.Context, req dtos.CreateUIDesignRequest) (*dtos.ProjectResponse, error) {
	return s.GenerateUIDesignStream(ctx, req, nil)
}

func (s *uiDesignService) GenerateUIDesignStream(
	ctx context.Context,
	req dtos.CreateUIDesignRequest,
	onEvent func(eventType string, data any),
) (*dtos.ProjectResponse, error) {
	if err := req.ValidateGenerationMode(); err != nil {
		return nil, err
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

		userID := s.userID(ctx)

		project, err := s.projects.CreateProject(ctx, uidesign.CreateProjectInput{UserID: userID, Title: title, DiagramType: "ui_design", ProjectMode: "ui_design", Metadata: metaBytes, Nodes: nodesBytes, Edges: edgesBytes})
		if err != nil {
			return nil, fmt.Errorf("failed to save blank ui project: %w", err)
		}

		_ = s.projects.CreateVersion(ctx, uidesign.CreateVersionInput{ProjectID: project.ID, VersionNumber: 1, ChangeSummary: "Initial Blank UI Canvas", Nodes: nodesBytes, Edges: edgesBytes, TriggerMessageID: nil})
		assistantReply := "✨ **Kanvas UI Design baru telah siap.**\n\nKetik ide atau deskripsi antarmuka yang ingin Anda bangun di AI Copilot untuk mulai mendesain!"
		msg, _ := s.projects.AppendMessage(ctx, project.ID, "assistant", assistantReply, nil)

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

			if tokenCounter-lastEmitted >= 60 || strings.Contains(delta, `data-rl-id="sec-`) || strings.Contains(delta, "</section>") {
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

				// Extract progressive HTML snapshot once Stage 2 raw HTML streaming starts
				previewHTML := ""
				if divIdx := strings.Index(bufStr, "<div"); divIdx != -1 {
					rawCandidate := bufStr[divIdx:]
					// Only stream progressive HTML when it is unescaped raw HTML (Stage 2)
					if !strings.Contains(rawCandidate[:min(len(rawCandidate), 120)], `\"`) && len(rawCandidate) > 220 {
						previewHTML = rawCandidate
						if lastSecClose := strings.LastIndex(previewHTML, "</section>"); lastSecClose != -1 {
							previewHTML = previewHTML[:lastSecClose+len("</section>")] + "\n</div>"
						} else if lastHeaderClose := strings.LastIndex(previewHTML, "</header>"); lastHeaderClose != -1 {
							previewHTML = previewHTML[:lastHeaderClose+len("</header>")] + "\n</div>"
						}
					}
				}

				progressPayload := map[string]interface{}{
					"stage":          "generating",
					"tokens":         tokenCounter,
					"chars":          charCounter,
					"message":        msg,
					"sections":       detectedSections,
					"active_section": currentSec,
				}
				if previewHTML != "" {
					progressPayload["preview_html"] = previewHTML
				}
				onEvent("progress", progressPayload)
			}
		}
	}

	// Infer design intent and compile UI Design through the AI Design Compiler
	compileOpts := inferDesignIntent(prompt, &req)
	if foundation == "" && req.Foundation != "" {
		foundation = strings.ToLower(strings.TrimSpace(req.Foundation))
	}
	dsl, err := s.compiler.CompileStream(ctx, prompt, device, foundation, themeMode, accentColor, onChunk, compileOpts)
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

	userID := s.userID(ctx)

	project, err := s.projects.CreateProject(ctx, uidesign.CreateProjectInput{UserID: userID, Title: projectTitle, DiagramType: "ui_design", ProjectMode: "ui_design", Metadata: metaBytes, Nodes: nodesBytes, Edges: edgesBytes})
	if err != nil {
		return nil, fmt.Errorf("failed to save ui project: %w", err)
	}

	// Version 1
	_ = s.projects.CreateVersion(ctx, uidesign.CreateVersionInput{ProjectID: project.ID, VersionNumber: 1, ChangeSummary: "AI Design Compiler - Initial Version", Nodes: nodesBytes, Edges: edgesBytes, TriggerMessageID: nil})

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
	msg1, _ := s.projects.AppendMessage(ctx, project.ID, "user", req.Prompt, nil)
	msg2, _ := s.projects.AppendMessage(ctx, project.ID, "assistant", assistantReply, nil)

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
