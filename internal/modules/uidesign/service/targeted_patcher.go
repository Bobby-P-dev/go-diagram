package service

import (
	"context"
	"fmt"
	"log"
	"regexp"
	"strings"
	"time"

	dtos "github.com/Bobby-P-dev/go-diagram.git/internal/modules/uidesign/dto"
)

type TargetedPatcher struct {
	aiService LLMClient
}

func NewTargetedPatcher(aiService LLMClient) *TargetedPatcher {
	return &TargetedPatcher{aiService: aiService}
}

// ApplyTargetedPatch applies a surgical code and spec patch to currentFrame based on changePlan.
// It guarantees that unrelated components and sections remain strictly unchanged.
func (tp *TargetedPatcher) ApplyTargetedPatch(
	ctx context.Context,
	currentFrame *dtos.UIFrameData,
	plan *dtos.ChangePlanDTO,
) (*dtos.UIFrameData, string, error) {
	if currentFrame == nil {
		return nil, "", fmt.Errorf("cannot patch nil frame")
	}
	if plan == nil {
		return currentFrame, "No changes requested.", nil
	}

	// 1. If strategy is full rebuild, do not patch locally
	if plan.Strategy == "rebuild" || plan.Regenerate {
		return nil, "", fmt.Errorf("rebuild strategy requires full compiler pipeline")
	}

	rawHtml := currentFrame.RawHtml
	if rawHtml == "" && currentFrame.CodeExport != nil {
		rawHtml = currentFrame.CodeExport["html"]
	}
	if rawHtml == "" && currentFrame.Implementation != nil {
		rawHtml = currentFrame.Implementation.Source.HTML
	}

	// 2. Identify target ID (with semantic section fallback so chat requests without canvas selection still patch locally)
	targetID := plan.Target.ID
	if targetID == "" {
		if plan.Target.ComponentID != "" {
			targetID = plan.Target.ComponentID
		} else if plan.Target.SectionID != "" {
			targetID = plan.Target.SectionID
		}
	}
	if targetID != "" && rawHtml != "" {
		if _, _, _, findErr := findElementSnippet(rawHtml, targetID); findErr != nil {
			if resolved := resolveSemanticSectionID(plan.Request, targetID, currentFrame, rawHtml); resolved != "" {
				targetID = resolved
			}
		}
	} else if targetID == "" && rawHtml != "" {
		if resolved := resolveSemanticSectionID(plan.Request, "", currentFrame, rawHtml); resolved != "" {
			targetID = resolved
		}
	}

	// 3. Global Token / Theme Updates (dark mode, primary color)
	if plan.Scope == "global" || plan.Strategy == "token_update" {
		patchedFrame := *currentFrame
		if patchedFrame.Theme == nil {
			patchedFrame.Theme = make(map[string]interface{})
		}
		for _, c := range plan.Changes {
			if c.Property == "primary_color" && c.To != "" {
				patchedFrame.Theme["primary"] = c.To
			}
			if c.Property == "theme_mode" && c.To != "" {
				patchedFrame.Theme["mode"] = c.To
			}
		}
		return &patchedFrame, fmt.Sprintf("Tema global diperbarui (%s).", plan.Request), nil
	}

	// 3.1 Handle Section Deletion (OpDeleteSection)
	if plan.Operation == dtos.OpDeleteSection {
		return tp.applyDeleteSection(currentFrame, plan, targetID, rawHtml)
	}

	// 3.2 Handle Section Insertion (OpInsertSection)
	if plan.Operation == dtos.OpInsertSection || plan.Strategy == "insert_section" {
		return tp.applyInsertSection(ctx, currentFrame, plan, targetID, rawHtml)
	}

	// 4. Section or Component Patching
	if targetID != "" && rawHtml != "" {
		snippet, startIdx, endIdx, findErr := findElementSnippet(rawHtml, targetID)
		if findErr == nil && snippet != "" {
			var patchedSnippet string
			var patchExplanation string

			// Fast-path / deterministic heuristics for common local styling
			patchedSnippet, patchExplanation = applyDeterministicSnippetPatch(snippet, plan)

			// If no deterministic patch or LLM is available for complex prompts, call LLM
			if patchedSnippet == snippet && tp.aiService != nil {
				llmSnippet, explanation, err := tp.callLLMTargetedPatch(ctx, snippet, targetID, plan, currentFrame)
				if err != nil {
					log.Printf("[TARGETED PATCHER] callLLMTargetedPatch failed for %s: %v", targetID, err)
				} else if llmSnippet != "" {
					log.Printf("[TARGETED PATCHER] callLLMTargetedPatch succeeded for %s (len=%d)", targetID, len(llmSnippet))
					patchedSnippet = llmSnippet
					patchExplanation = explanation
				}
			}

			if patchedSnippet != "" && patchedSnippet != snippet {
				// Perform surgical string replacement ONLY at the exact index
				newHtml := rawHtml[:startIdx] + patchedSnippet + rawHtml[endIdx:]

				// Preservation Guard: verify other sections are still intact
				if !verifyPreservation(rawHtml, newHtml, targetID) {
					return nil, "", fmt.Errorf("preservation guard failed: unrelated sections were modified")
				}

				// Duplicate Guard: ensure the patch did not introduce duplicate data-rl-id
				validator := NewDuplicateValidator()
				if err := validator.ValidateHTMLDuplicateIDs(newHtml); err != nil {
					return nil, "", fmt.Errorf("duplicate validator failed after patch: %w", err)
				}

				patchedFrame := *currentFrame
				patchedFrame.RawHtml = newHtml
				if patchedFrame.CodeExport == nil {
					patchedFrame.CodeExport = make(map[string]string)
				}
				patchedFrame.CodeExport["html"] = newHtml
				patchedFrame.ChangePlan = plan

				// Crucial: Synchronize Implementation.Source.HTML so frontend renders latest code
				if patchedFrame.Implementation != nil {
					implCopy := *patchedFrame.Implementation
					implCopy.Source.HTML = newHtml
					patchedFrame.Implementation = &implCopy
				} else {
					patchedFrame.Implementation = &dtos.ImplementationStateDTO{
						Framework: "vue",
						Styling:   "tailwind",
						Source: dtos.ImplementationSourceDTO{
							HTML: newHtml,
						},
					}
				}

				// Update corresponding section in design spec if applicable
				updateDesignSpecSection(&patchedFrame, targetID, plan)

				if patchExplanation == "" {
					patchExplanation = fmt.Sprintf("Perubahan diterapkan secara presisi pada elemen `%s`.", targetID)
				}
				return &patchedFrame, patchExplanation, nil
			}
		}
	}

	// Fallback: If no HTML snippet matched or no code modified, fail explicitly (zero fake success)
	if targetID != "" {
		return nil, "", fmt.Errorf("target '%s' tidak ditemukan dalam markup HTML atau tidak ada perubahan kode yang dihasilkan", targetID)
	}
	return nil, "", fmt.Errorf("tidak ada target elemen yang dipilih untuk targeted patch")
}

func findElementSnippet(source, targetID string) (string, int, int, error) {
	return findHTMLTarget(source, targetID, true)
}

// applyDeterministicSnippetPatch applies fast rule-based edits for high speed and test repeatability.
func applyDeterministicSnippetPatch(snippet string, plan *dtos.ChangePlanDTO) (string, string) {
	reqLower := strings.ToLower(plan.Request)
	result := snippet
	var explanation []string

	// Header floating & rounded styling
	if strings.Contains(reqLower, "header") || strings.Contains(reqLower, "navbar") {
		if strings.Contains(reqLower, "tidak full") || strings.Contains(reqLower, "floating") || strings.Contains(reqLower, "max width") || strings.Contains(reqLower, "rounded") {
			// Transform w-full border-b into floating rounded container
			result = strings.ReplaceAll(result, "w-full border-b", "max-w-7xl mx-auto my-4 rounded-3xl border")
			result = strings.ReplaceAll(result, "sticky top-0", "sticky top-4")
			explanation = append(explanation, "Header nav diubah menjadi floating container dengan sudut membulat (rounded-3xl) dan margin simetris.")
		}
	}

	// Button rounded styling
	if strings.Contains(reqLower, "rounded") && (strings.Contains(reqLower, "button") || strings.Contains(reqLower, "tombol")) {
		if !strings.Contains(result, "rounded-full") {
			result = strings.ReplaceAll(result, "rounded-lg", "rounded-full")
			result = strings.ReplaceAll(result, "rounded-xl", "rounded-full")
			result = strings.ReplaceAll(result, "rounded-2xl", "rounded-full")
			result = strings.ReplaceAll(result, "rounded-md", "rounded-full")
			explanation = append(explanation, "Sudut tombol disesuaikan menjadi rounded-full.")
		}
	}

	// Button size adjustments
	if (strings.Contains(reqLower, "kecil") || strings.Contains(reqLower, "minimalis") || strings.Contains(reqLower, "compact")) &&
		(strings.Contains(reqLower, "button") || strings.Contains(reqLower, "tombol")) {
		result = strings.ReplaceAll(result, "px-5 py-2.5", "px-4 py-1.5 text-[11px]")
		result = strings.ReplaceAll(result, "px-6 py-3", "px-4 py-1.5 text-[11px]")
		result = strings.ReplaceAll(result, "px-7 py-4", "px-5 py-2 text-xs")
		explanation = append(explanation, "Ukuran dan visual weight tombol disesuaikan menjadi lebih ringkas dan minimalis.")
	}

	if len(explanation) > 0 {
		return result, strings.Join(explanation, " ")
	}
	return snippet, ""
}

// resolveSemanticSectionID matches a user's natural-language request or partial section hint
// against the actual sections present in currentFrame.Sections or rawHtml.
func resolveSemanticSectionID(request, hint string, frame *dtos.UIFrameData, rawHtml string) string {
	combined := strings.ToLower(request + " " + hint)
	if frame != nil {
		for _, sec := range frame.Sections {
			secIDLow := strings.ToLower(sec.ID)
			secTypeLow := strings.ToLower(sec.Type)
			cleanID := strings.TrimPrefix(secIDLow, "sec-")
			if cleanID != "" && strings.Contains(combined, cleanID) {
				if _, _, _, err := findElementSnippet(rawHtml, sec.ID); err == nil {
					return sec.ID
				}
			}
			if secTypeLow != "" && strings.Contains(combined, secTypeLow) {
				if _, _, _, err := findElementSnippet(rawHtml, sec.ID); err == nil {
					return sec.ID
				}
			}
		}
	}
	// Scan actual data-rl-id="sec-..." in rawHtml for keyword overlap
	secRe := regexp.MustCompile(`data-rl-id=["'](sec-[^"']+)["']`)
	for _, m := range secRe.FindAllStringSubmatch(rawHtml, -1) {
		if len(m) > 1 {
			cand := m[1]
			tokens := strings.Split(strings.TrimPrefix(strings.ToLower(cand), "sec-"), "-")
			for _, tok := range tokens {
				if len(tok) >= 3 && strings.Contains(combined, tok) {
					return cand
				}
			}
		}
	}
	return ""
}

// extractFrameStyleAndThemeContext extracts theme tokens and scoped <style> rules from the frame
// so local patches and inserted sections preserve 100% of the page's bespoke visual DNA.
func extractFrameStyleAndThemeContext(frame *dtos.UIFrameData) string {
	if frame == nil {
		return ""
	}
	var b strings.Builder
	if len(frame.Theme) > 0 {
		for _, k := range []string{"mode", "background", "surface", "foreground", "primary", "accent", "border", "fontFamily", "headingFont", "monoFont"} {
			if v, ok := frame.Theme[k].(string); ok && v != "" {
				b.WriteString(fmt.Sprintf("- theme.%s: %s\n", k, v))
			}
		}
	}
	rawHtml := frame.RawHtml
	if rawHtml == "" && frame.CodeExport != nil {
		rawHtml = frame.CodeExport["html"]
	}
	if styleMatch := regexp.MustCompile(`(?is)<style[^>]*>(.*?)</style>`).FindStringSubmatch(rawHtml); len(styleMatch) > 1 {
		cssSnippet := strings.TrimSpace(styleMatch[1])
		if len(cssSnippet) > 1200 {
			cssSnippet = cssSnippet[:1200] + "\n/* ... */"
		}
		if cssSnippet != "" {
			b.WriteString("EXISTING PAGE SCOPED <style> VARIABLES & UTILITIES:\n")
			b.WriteString(cssSnippet + "\n")
		}
	}
	return b.String()
}

// callLLMTargetedPatch uses the LLM to surgically modify only the target snippet.
func (tp *TargetedPatcher) callLLMTargetedPatch(
	ctx context.Context,
	snippet string,
	targetID string,
	plan *dtos.ChangePlanDTO,
	frame *dtos.UIFrameData,
) (string, string, error) {
	accentColor := "#6366f1"
	if frame != nil && frame.Theme != nil {
		if prim, ok := frame.Theme["primary"].(string); ok && prim != "" {
			accentColor = prim
		}
	}
	themeAndStyleDNA := extractFrameStyleAndThemeContext(frame)

	systemPrompt := `You are a Surgical AI UI Code Editor for RancangLab.
Your goal is to modify ONLY the provided HTML snippet to fulfill the user's edit instruction while keeping 100% visual harmony with the page's existing design system, palette, typography, and custom CSS classes.

STRICT EDIT RULES:
1. Maintain the outer element's data-rl-id, data-rl-kind, and id attributes intact.
2. Harmonize colors, surfaces, borders, and font classes (font-serif, font-mono, custom CSS variables) with the provided PAGE DESIGN SYSTEM DNA.
3. DO NOT output markdown code blocks, backticks, or explanation. Output ONLY the raw HTML replacement.
4. DO NOT alter elements or sections outside this target.
5. In-page links must use #anchors (e.g. #products, #story). Never use relative links like /products or /.
6. ANTI-SLOP: Do NOT add generic AI purple/blue gradients or blurry glow orbs. Use concrete, human copy in the same language as the snippet.
`

	userPrompt := fmt.Sprintf(`Target ID: %s
User Instruction: %s
Scope: %s
Primary Accent: %s
PAGE DESIGN SYSTEM DNA:
%s
ORIGINAL SNIPPET:
%s

Return ONLY the updated HTML snippet:`, targetID, plan.Request, plan.Scope, accentColor, themeAndStyleDNA, snippet)

	resp, err := tp.aiService.CallLLMText(systemPrompt, userPrompt)
	if err != nil {
		return "", "", fmt.Errorf("llm targeted patch call failed: %w", err)
	}

	clean := strings.TrimSpace(resp)
	clean = sanitizeStaticHTML(clean)

	explanation := fmt.Sprintf("Elemen `%s` berhasil diperbarui sesuai instruksi: %q.", targetID, plan.Request)
	return clean, explanation, nil
}

// verifyPreservation ensures that sections outside the target remain intact.
func verifyPreservation(oldHtml, newHtml, targetID string) bool {
	// Extract all section data-rl-ids
	secRe := regexp.MustCompile(`data-rl-id=["'](sec-[^"']+)["']`)
	oldMatches := secRe.FindAllStringSubmatch(oldHtml, -1)
	newMatches := secRe.FindAllStringSubmatch(newHtml, -1)

	// Build sets
	oldSecs := make(map[string]bool)
	for _, m := range oldMatches {
		if len(m) > 1 {
			oldSecs[m[1]] = true
		}
	}
	newSecs := make(map[string]bool)
	for _, m := range newMatches {
		if len(m) > 1 {
			newSecs[m[1]] = true
		}
	}

	// Verify no unintended deletion of other sections
	for s := range oldSecs {
		if s != targetID && !newSecs[s] {
			return false
		}
	}
	return true
}

// updateDesignSpecSection updates metadata in DesignSpec for the patched section
func updateDesignSpecSection(frame *dtos.UIFrameData, targetID string, plan *dtos.ChangePlanDTO) {
	if frame.Sections != nil {
		for i := range frame.Sections {
			if frame.Sections[i].ID == targetID || frame.Sections[i].Type == targetID {
				if frame.Sections[i].Data == nil {
					frame.Sections[i].Data = make(map[string]interface{})
				}
				frame.Sections[i].Data["last_patch"] = plan.Request
				frame.Sections[i].Data["last_strategy"] = plan.Strategy
			}
		}
	}

	if frame.DesignState != nil && frame.DesignState.DesignSpec != nil && frame.DesignState.DesignSpec.Sections != nil {
		for i := range frame.DesignState.DesignSpec.Sections {
			if frame.DesignState.DesignSpec.Sections[i].ID == targetID || frame.DesignState.DesignSpec.Sections[i].Type == targetID {
				if frame.DesignState.DesignSpec.Sections[i].Data == nil {
					frame.DesignState.DesignSpec.Sections[i].Data = make(map[string]interface{})
				}
				frame.DesignState.DesignSpec.Sections[i].Data["last_patch"] = plan.Request
				frame.DesignState.DesignSpec.Sections[i].Data["last_strategy"] = plan.Strategy
			}
		}
	}
}

func (tp *TargetedPatcher) applyDeleteSection(
	currentFrame *dtos.UIFrameData,
	plan *dtos.ChangePlanDTO,
	targetID string,
	rawHtml string,
) (*dtos.UIFrameData, string, error) {
	if rawHtml == "" {
		return nil, "", fmt.Errorf("raw html is empty")
	}
	if targetID == "" {
		for _, kw := range []string{"testimonial", "review", "pricing", "hero", "footer", "faq", "features", "stats", "cta"} {
			if strings.Contains(strings.ToLower(plan.Request), kw) {
				targetID = "sec-" + kw
				break
			}
		}
	}
	if targetID == "" {
		return nil, "", fmt.Errorf("target section ID is required for section deletion")
	}

	snippet, startIdx, endIdx, err := findElementSnippet(rawHtml, targetID)
	if err != nil || snippet == "" {
		return nil, "", fmt.Errorf("section '%s' not found for deletion: %w", targetID, err)
	}

	newHtml := strings.TrimSpace(rawHtml[:startIdx]) + "\n" + strings.TrimSpace(rawHtml[endIdx:])
	patchedFrame := *currentFrame
	patchedFrame.RawHtml = newHtml
	if patchedFrame.CodeExport == nil {
		patchedFrame.CodeExport = make(map[string]string)
	}
	patchedFrame.CodeExport["html"] = newHtml
	if patchedFrame.Implementation != nil {
		implCopy := *patchedFrame.Implementation
		implCopy.Source.HTML = newHtml
		patchedFrame.Implementation = &implCopy
	}
	patchedFrame.ChangePlan = plan

	return &patchedFrame, fmt.Sprintf("Seksi '%s' berhasil dihapus dari halaman.", targetID), nil
}

func (tp *TargetedPatcher) applyInsertSection(
	ctx context.Context,
	currentFrame *dtos.UIFrameData,
	plan *dtos.ChangePlanDTO,
	targetID string,
	rawHtml string,
) (*dtos.UIFrameData, string, error) {
	if rawHtml == "" {
		return nil, "", fmt.Errorf("raw html is empty")
	}

	newSectionHtml, genErr := tp.generateSectionMarkup(ctx, plan, currentFrame)
	if genErr != nil || strings.TrimSpace(newSectionHtml) == "" {
		return nil, "", fmt.Errorf("failed to generate section markup: %w", genErr)
	}

	// B5 GUARD: The freshly generated section must pass the same sanitization as
	// the main compiler pipeline before it is allowed into the page. This strips
	// any scripts/event handlers the LLM slipped in and rejects a section that is
	// not a real, targetable <section> (zero fake success).
	newSectionHtml = sanitizeStaticHTML(newSectionHtml)
	cleanSection := strings.TrimSpace(newSectionHtml)
	if !regexp.MustCompile(`(?i)<section\b[^>]*data-rl-id=["'][^"']+["']`).MatchString(cleanSection) {
		trunc := cleanSection
		if len(trunc) > 120 {
			trunc = trunc[:120] + "..."
		}
		return nil, "", fmt.Errorf("generated section markup is not a targetable <section>: %s", trunc)
	}

	var insertIdx = -1
	if targetID != "" {
		_, _, endIdx, err := findElementSnippet(rawHtml, targetID)
		if err == nil && endIdx > 0 {
			insertIdx = endIdx
		}
	}

	// If no valid targetID, insert right before footer
	if insertIdx < 0 {
		_, footerStart, _, footerErr := findElementSnippet(rawHtml, "sec-footer")
		if footerErr != nil {
			_, footerStart, _, footerErr = findElementSnippet(rawHtml, "footer")
		}
		if footerErr == nil && footerStart > 0 {
			insertIdx = footerStart
		}
	}

	// If still no insertion point, insert right before the last closing container tag
	if insertIdx < 0 {
		lastClose := strings.LastIndex(rawHtml, "</")
		if lastClose > 0 {
			insertIdx = lastClose
		} else {
			insertIdx = len(rawHtml)
		}
	}

	newHtml := rawHtml[:insertIdx] + "\n\n" + strings.TrimSpace(newSectionHtml) + "\n\n" + rawHtml[insertIdx:]

	validator := NewDuplicateValidator()
	if err := validator.ValidateHTMLDuplicateIDs(newHtml); err != nil {
		log.Printf("[TARGETED PATCHER] duplicate ID warning during section insertion: %v", err)
	}

	patchedFrame := *currentFrame
	patchedFrame.RawHtml = newHtml
	if patchedFrame.CodeExport == nil {
		patchedFrame.CodeExport = make(map[string]string)
	}
	patchedFrame.CodeExport["html"] = newHtml
	if patchedFrame.Implementation != nil {
		implCopy := *patchedFrame.Implementation
		implCopy.Source.HTML = newHtml
		patchedFrame.Implementation = &implCopy
	}
	patchedFrame.ChangePlan = plan

	// Extract new section ID and register into canonical sections list
	secIdRe := regexp.MustCompile(`data-rl-id=["'](sec-[^"']+)["']`)
	newSecID := ""
	if m := secIdRe.FindStringSubmatch(newSectionHtml); len(m) > 1 {
		newSecID = m[1]
	} else {
		newSecID = fmt.Sprintf("sec-%d", time.Now().UnixNano()%100000)
	}
	cleanType := strings.TrimPrefix(newSecID, "sec-")

	newSectionDTO := dtos.UISectionDTO{
		ID:                newSecID,
		Type:              cleanType,
		Purpose:           plan.Request,
		Priority:          "medium",
		RequirementSource: "user_chat",
	}
	patchedFrame.Sections = append(patchedFrame.Sections, newSectionDTO)
	if patchedFrame.DesignState != nil && patchedFrame.DesignState.DesignSpec != nil {
		patchedFrame.DesignState.DesignSpec.Sections = append(patchedFrame.DesignState.DesignSpec.Sections, newSectionDTO)
	}

	return &patchedFrame, fmt.Sprintf("Seksi baru `%s` berhasil ditambahkan (%s).", newSecID, plan.Request), nil
}

func (tp *TargetedPatcher) generateSectionMarkup(
	ctx context.Context,
	plan *dtos.ChangePlanDTO,
	frame *dtos.UIFrameData,
) (string, error) {
	accentColor := "#6366f1"
	themeMode := "dark"
	if frame.Theme != nil {
		if prim, ok := frame.Theme["primary"].(string); ok && prim != "" {
			accentColor = prim
		}
		if m, ok := frame.Theme["mode"].(string); ok && m != "" {
			themeMode = m
		}
	}

	if tp.aiService == nil {
		secID := fmt.Sprintf("sec-%d", time.Now().UnixNano()%100000)
		return fmt.Sprintf(`<section data-rl-id="%s" class="w-full py-12 px-6"><div class="max-w-6xl mx-auto"><h2 class="text-2xl font-bold mb-4">%s</h2></div></section>`, secID, plan.Request), nil
	}

	titleContext := frame.Title
	if titleContext == "" {
		titleContext = "Visual Workspace UI"
	}

	device := "web"
	if frame.Device != "" {
		device = frame.Device
	}

	themeAndStyleDNA := extractFrameStyleAndThemeContext(frame)

	sysPrompt := fmt.Sprintf(`You are an expert Tailwind CSS UI section craftsman for RancangLab.
Generate a SINGLE complete, bespoke, high-craft HTML section matching the user request and harmonizing 100%% with the existing page's visual DNA.
Context:
- Project / Page: %s
- Theme mode: %s
- Accent color: %s
- Target device: %s
PAGE DESIGN SYSTEM DNA:
%s

CRAFT & QUALITY RULES:
1. Wrap everything in a single root <section data-rl-id="sec-[type]-[unique]" data-rl-kind="section" class="..."> tag.
2. Every interactive element, heading, button, or card must have data-rl-id="cmp-[type]-[unique]" data-rl-kind="component".
3. Use Tailwind CSS utility classes and any custom classes/variables from PAGE DESIGN SYSTEM DNA so surfaces, typography (font-serif, font-mono), border treatment, and contrast match the rest of the page seamlessly.
4. Use asymmetric Bento grids, rich inline SVG visualizations, or expressive editorial layouts rather than flat 3-equal-card boilerplate.
5. Provide realistic, human microcopy and data. Do not use Lorem Ipsum.
6. Output ONLY the raw HTML section, no markdown fences, no explanation.`, titleContext, themeMode, accentColor, device, themeAndStyleDNA)

	userMsg := fmt.Sprintf(`Section Request: %q
Page Device: %s`, plan.Request, device)

	resp, err := tp.aiService.CallLLMText(sysPrompt, userMsg)
	if err != nil {
		return "", err
	}
	clean := strings.TrimSpace(resp)
	clean = strings.TrimPrefix(clean, "```html")
	clean = strings.TrimPrefix(clean, "```")
	clean = strings.TrimSuffix(clean, "```")
	return strings.TrimSpace(clean), nil
}
