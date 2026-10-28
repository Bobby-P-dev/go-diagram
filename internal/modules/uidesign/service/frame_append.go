package service

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/Bobby-P-dev/go-diagram.git/internal/modules/uidesign"
	dtos "github.com/Bobby-P-dev/go-diagram.git/internal/modules/uidesign/dto"
	"regexp"
	"strconv"
	"strings"
)

// appendNewUIFrame adds a brand-new page/frame to an existing UI Design project
// without touching any existing frame nodes. It compiles one fresh frame through
// the single-call pipeline, appends it as a new ui_frame node, persists a new
// version, and returns the updated project state containing ALL frames.
func (s *uiDesignService) appendNewUIFrame(
	ctx context.Context,
	project *uidesign.Project,
	messages []uidesign.ChatMessage,
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
	var screen1DesignDNA strings.Builder

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
				for _, k := range []string{"background", "surface", "foreground", "primary", "accent", "border", "fontFamily", "headingFont", "monoFont"} {
					if v, ok := m[k].(string); ok && v != "" {
						screen1DesignDNA.WriteString(fmt.Sprintf("  * theme.%s: %s\n", k, v))
					}
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
				if styleMatch := regexp.MustCompile(`(?is)<style[^>]*>(.*?)</style>`).FindStringSubmatch(firstHtml); len(styleMatch) > 1 {
					cssSnippet := strings.TrimSpace(styleMatch[1])
					if len(cssSnippet) > 1200 {
						cssSnippet = cssSnippet[:1200] + "\n/* ... */"
					}
					if cssSnippet != "" {
						screen1DesignDNA.WriteString("  * SCREEN 1 SCOPED <style> TOKENS & ANIMATIONS:\n" + cssSnippet + "\n")
					}
				}
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
		consistencyPrompt.WriteString(fmt.Sprintf("- Tema Visual: Mode %s, Aksen Warna %s. Pertahankan tone warna latar, border, dan pasangan font yang 100%% identik dengan Screen 1.\n", themeMode, accentColor))
	}
	if screen1DesignDNA.Len() > 0 {
		consistencyPrompt.WriteString("- DESIGN SYSTEM DNA DARI SCREEN 1 (WAJIB DIPERTAHANKAN):\n")
		consistencyPrompt.WriteString(screen1DesignDNA.String())
	}

	consistencyPrompt.WriteString(`- ATURAN KETAT ANTI-SLOP (PURE ONLY CONTEXT & ZERO HALLUCINATION):
  1. DILARANG KERAS membuat seksi promosi marketing klise (testimoni fiktif, ulasan rating bintang, FAQ template, tabel pricing, logo bar 'Trusted by') KECUALI diminta secara eksplisit.
  2. Halaman baru harus menjadi bagian fungsional nyata dari domain sistem Screen 1.
  3. DILARANG menggunakan gradien ungu-indigo klise AI (from-purple-600 to-indigo-600), background glow orbs, atau 3 kartu seragam copy-paste.
  4. DILARANG menggunakan kata-kata AI klise (supercharge, seamless, unlock, revolutionize). Gunakan bahasa manusia konkret.
`)

	if isAuthPage {
		effectiveBrand := brandText
		if effectiveBrand == "" {
			effectiveBrand = firstFrameTitle
		}
		consistencyPrompt.WriteString(fmt.Sprintf(`- INSTRUKSI KHUSUS HALAMAN LOGIN / AUTHENTICATION (ADAPTIF SESUAI KONTEKS DOMAIN SCREEN 1):
  1. IDENTITAS BRAND & JUDUL: Wajib menampilkan logo dan nama brand '%s' secara persis dengan estetika, palet warna (%s), dan tipografi Screen 1.
  2. KONTEKS AUDIENS (%s — %s):
     - Jika Screen 1 adalah produk konsumen, lifestyle, kuliner, e-commerce, atau kreatif: rancang pengalaman login/member yang berkarakter sesuai mood brand (misal split editorial showcase + form login/member yang elegan), BUKAN portal IT korporat kaku.
     - Jika Screen 1 adalah sistem enterprise, ERP, atau infrastruktur B2B: rancang portal autentikasi organisasi yang presisi dengan dukungan SSO/kredensial kerja.
  3. Jangan menambahkan navbar marketing penuh atau section promosi yang tidak relevan dengan proses autentikasi.
  4. ATURAN STATIC MARKUP: DILARANG KERAS menyertakan event handler JavaScript (seperti onsubmit="return false", onclick, onchange) dan DILARANG menggunakan href="javascript:void(0)". Gunakan tag <form> standar dan href="#" atau <button type="button">.
`, effectiveBrand, accentColor, productDomain, targetUser))
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
	summary := fmt.Sprintf("✨ **[INSERT_FRAME]** Halaman UI baru '%s' (device: %s) ditambahkan di samping frame yang ada.", frame.Title, device)
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
