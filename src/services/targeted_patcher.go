package services

import (
	"context"
	"fmt"
	"log"
	"regexp"
	"strings"

	"github.com/Bobby-P-dev/go-diagram.git/src/dtos"
)

type TargetedPatcher struct {
	aiService *AIService
}

func NewTargetedPatcher(aiService *AIService) *TargetedPatcher {
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

	// 2. Identify target ID
	targetID := plan.Target.ID
	if targetID == "" {
		if plan.Target.ComponentID != "" {
			targetID = plan.Target.ComponentID
		} else if plan.Target.SectionID != "" {
			targetID = plan.Target.SectionID
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

// findElementSnippet finds an element marked with data-rl-id="targetID" or id="targetID"
// using balanced tag boundary scanning.
func findElementSnippet(html, targetID string) (string, int, int, error) {
	// Look for data-rl-id="targetID" or id="targetID"
	patterns := []string{
		fmt.Sprintf(`data-rl-id=["']%s["']`, regexp.QuoteMeta(targetID)),
		fmt.Sprintf(`id=["']%s["']`, regexp.QuoteMeta(targetID)),
	}

	var matchIdx = -1
	for _, p := range patterns {
		re := regexp.MustCompile(p)
		loc := re.FindStringIndex(html)
		if loc != nil {
			matchIdx = loc[0]
			break
		}
	}

	var startTagOpen = -1
	if matchIdx != -1 {
		startTagOpen = strings.LastIndex(html[:matchIdx+1], "<")
	} else {
		// Fallback for semantic sections in legacy HTML
		if targetID == "sec-header" || targetID == "header" {
			re := regexp.MustCompile(`(?i)<header\b`)
			loc := re.FindStringIndex(html)
			if loc != nil {
				startTagOpen = loc[0]
			}
		} else if targetID == "sec-footer" || targetID == "footer" {
			re := regexp.MustCompile(`(?i)<footer\b`)
			loc := re.FindStringIndex(html)
			if loc != nil {
				startTagOpen = loc[0]
			}
		}
	}

	if startTagOpen == -1 {
		return "", -1, -1, fmt.Errorf("target %s not found in html", targetID)
	}

	// Determine tag name
	tagRest := html[startTagOpen+1:]
	spaceIdx := strings.IndexAny(tagRest, " >\n\r\t")
	if spaceIdx == -1 {
		return "", -1, -1, fmt.Errorf("cannot parse tag name for target %s", targetID)
	}
	tagName := strings.ToLower(tagRest[:spaceIdx])

	// Self-closing tags (img, input, hr, br)
	if tagName == "img" || tagName == "input" || tagName == "hr" || tagName == "br" {
		closeIdx := strings.Index(html[startTagOpen:], ">")
		if closeIdx != -1 {
			end := startTagOpen + closeIdx + 1
			return html[startTagOpen:end], startTagOpen, end, nil
		}
	}

	// Balanced tag scanner
	openPattern := regexp.MustCompile(fmt.Sprintf(`(?i)<%s\b`, regexp.QuoteMeta(tagName)))
	closePattern := regexp.MustCompile(fmt.Sprintf(`(?i)</%s\s*>`, regexp.QuoteMeta(tagName)))

	depth := 0
	curr := startTagOpen
	htmlLen := len(html)

	for curr < htmlLen {
		openLoc := openPattern.FindStringIndex(html[curr:])
		closeLoc := closePattern.FindStringIndex(html[curr:])

		if closeLoc == nil {
			break
		}

		if openLoc != nil && openLoc[0] < closeLoc[0] {
			depth++
			curr += openLoc[1]
		} else {
			depth--
			if depth == 0 {
				endIdx := curr + closeLoc[1]
				return html[startTagOpen:endIdx], startTagOpen, endIdx, nil
			}
			curr += closeLoc[1]
		}
	}

	return "", -1, -1, fmt.Errorf("could not find matching closing tag for <%s>", tagName)
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

// callLLMTargetedPatch uses the LLM to surgically modify only the target snippet.
func (tp *TargetedPatcher) callLLMTargetedPatch(
	ctx context.Context,
	snippet string,
	targetID string,
	plan *dtos.ChangePlanDTO,
	frame *dtos.UIFrameData,
) (string, string, error) {
	accentColor := "#D4AF37"
	if frame.Theme != nil {
		if prim, ok := frame.Theme["primary"].(string); ok && prim != "" {
			accentColor = prim
		}
	}

	systemPrompt := `You are a Surgical AI UI Code Editor for RancangLab.
Your goal is to modify ONLY the provided HTML snippet to fulfill the user's edit instruction.

STRICT EDIT RULES:
1. Maintain data-rl-id and id markers on elements.
2. Use modern Tailwind CSS classes conforming to the aesthetic style.
3. DO NOT output markdown code blocks, backticks, or explanation. Output ONLY the raw HTML replacement.
4. DO NOT alter elements or sections outside this target.
5. In-page links must use #anchors (e.g. #products, #story). Never use relative links like /products or /.
`

	userPrompt := fmt.Sprintf(`Target ID: %s
User Instruction: %s
Scope: %s
Primary Accent: %s

ORIGINAL SNIPPET:
%s

Return ONLY the updated HTML snippet:`, targetID, plan.Request, plan.Scope, accentColor, snippet)

	resp, err := tp.aiService.CallLLMText(systemPrompt, nil, userPrompt)
	if err != nil {
		return "", "", fmt.Errorf("llm targeted patch call failed: %w", err)
	}

	clean := strings.TrimSpace(resp)
	clean = strings.TrimPrefix(clean, "```html")
	clean = strings.TrimPrefix(clean, "```")
	clean = strings.TrimSuffix(clean, "```")
	clean = strings.TrimSpace(clean)

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
