package service

import (
	"context"
	"fmt"
	"log"
	"regexp"
	"strings"

	dtos "github.com/Bobby-P-dev/go-diagram.git/internal/modules/uidesign/dto"
)

type UIDesignCompiler struct {
	analyzer  *RequirementAnalyzer
	planner   *DesignPlanner
	validator *UIValidator
	aiService LLMClient
}

func NewUIDesignCompiler(aiService LLMClient) *UIDesignCompiler {
	return &UIDesignCompiler{
		analyzer:  NewRequirementAnalyzer(aiService),
		planner:   NewDesignPlanner(aiService),
		validator: NewUIValidator(),
		aiService: aiService,
	}
}

func (c *UIDesignCompiler) Compile(
	ctx context.Context,
	rawPrompt string,
	device string,
	foundation string,
	themeMode string,
	accentColor string,
	opts ...dtos.CompileOptions,
) (*dtos.UIDesignDSL, error) {
	return c.CompileStream(ctx, rawPrompt, device, foundation, themeMode, accentColor, nil, opts...)
}

func (c *UIDesignCompiler) CompileStream(
	ctx context.Context,
	rawPrompt string,
	device string,
	foundation string,
	themeMode string,
	accentColor string,
	onChunk LLMStreamCallback,
	opts ...dtos.CompileOptions,
) (*dtos.UIDesignDSL, error) {
	// Fallback sanitization
	if device == "" {
		device = "web"
	}

	var compileOpt dtos.CompileOptions
	if len(opts) > 0 {
		compileOpt = opts[0]
	}

	// STAGE 1: SINGLE-CALL SYNTHESIS (analyze + plan + implement in one round-trip)
	syn, err := c.synthesizeStream(ctx, rawPrompt, device, foundation, themeMode, accentColor, compileOpt, onChunk)
	if err != nil {
		return nil, fmt.Errorf("stage 1 synthesis failed: %w", err)
	}

	// Deterministic reconstruction into the canonical requirement & design specs.
	reqSpec := finalizeRequirementSpec(syn, rawPrompt)
	designSpec := finalizeDesignSpec(syn, themeMode, reqSpec)

	// STAGE 2: UI VALIDATOR & ANTI-HALLUCINATION GUARD (deterministic)
	validationResult, validatedDesignSpec := c.validator.ValidateAndAudit(reqSpec, designSpec)
	if validationResult.Status == "fail" || validatedDesignSpec == nil || len(validatedDesignSpec.Sections) == 0 {
		return nil, fmt.Errorf("stage 2 validation failed: no usable prompt-grounded sections")
	}

	// STAGE 3: CODE EXPORT & THEME ASSEMBLY (from the single synthesis call)
	frameWidth := 1024
	frameHeight := 720
	if device == "mobile" {
		frameWidth = 375
		frameHeight = 812
	} else if device == "desktop" {
		frameWidth = 1100
		frameHeight = 740
	}

	title := strings.TrimSpace(syn.Title)
	if title == "" {
		title = "UI Design: " + reqSpec.Page.Type
		if strings.TrimSpace(rawPrompt) != "" {
			firstFewWords := strings.Fields(rawPrompt)
			if len(firstFewWords) > 5 {
				title = strings.Join(firstFewWords[:5], " ") + "..."
			} else {
				title = rawPrompt
			}
		}
	}

	// Strip any hallucinated sections removed by UIValidator from the raw HTML using balanced tag boundary scanning
	for _, stripped := range validationResult.StrippedSections {
		parts := strings.Split(stripped, ":")
		secID := strings.TrimSpace(parts[0])
		if secID != "" {
			_, startIdx, endIdx, findErr := findHTMLTarget(syn.RawHTML, secID, false)
			if findErr != nil {
				return nil, fmt.Errorf("cannot safely remove rejected section %q: %w", secID, findErr)
			}
			syn.RawHTML = syn.RawHTML[:startIdx] + syn.RawHTML[endIdx:]
		}
	}
	// If domain is null (generic prompt), cleanse prohibited hallucinated terms from syn.RawHTML and DesignSpec
	if reqSpec.Context.Domain == nil {
		prohibitedReplacements := map[string]string{
			"cfo":           "Team",
			"treasury":      "Account",
			"virtual card":  "Feature",
			"credit limit":  "Access",
			"crypto wallet": "Wallet",
			"crypto":        "Digital",
			"blockchain":    "Platform",
		}
		for ph, repl := range prohibitedReplacements {
			patt := regexp.MustCompile(fmt.Sprintf(`(?i)\b%s\b`, regexp.QuoteMeta(ph)))
			syn.RawHTML = replaceTextContentOnly(syn.RawHTML, patt, repl)
			for idx := range validatedDesignSpec.Sections {
				s := &validatedDesignSpec.Sections[idx]
				s.Purpose = patt.ReplaceAllString(s.Purpose, repl)
				if s.Data != nil {
					for k, v := range s.Data {
						if strVal, ok := v.(string); ok {
							s.Data[k] = patt.ReplaceAllString(strVal, repl)
						}
					}
				}
			}
		}
	}

	// B3 RESOLUTION (single reconcile point): Only keep sections whose data-rl-id
	// actually exists in the final rendered HTML. This keeps the Spec / 3-Pillar
	// view and the iframe preview aligned — a section removed by the validator or
	// never rendered is dropped here instead of lingering as an "orphan" that makes
	// the design look like it injected an unrequested element. Runs once, at the
	// end of compilation, so no later stage re-introduces drift.
	validatedDesignSpec.Sections = reconcileSectionsToHTML(syn.RawHTML, validatedDesignSpec.Sections)

	// Split the synthesized HTML into the canonical code export (html + Vue SFC).
	codeExport := splitHTMLToCodeExport(syn.RawHTML)
	generatedTheme := syn.Theme
	if generatedTheme == nil {
		generatedTheme = map[string]interface{}{"mode": themeMode}
	}
	if foundation != "" && foundation != "auto" {
		generatedTheme["palette"] = foundation
	}
	enrichThemeTypography(generatedTheme, foundation, rawPrompt, compileOpt.ProductContext)

	// Map to PageSpecification & DesignDecisions for backwards compatibility and Spec tab inspection
	pageSpec := &dtos.PageSpecificationDTO{
		PageName:           title,
		Purpose:            reqSpec.Page.Purpose,
		PrimaryGoal:        reqSpec.Goals.Primary,
		Layout:             validatedDesignSpec.Layout.Type,
		RequiredComponents: extractComponentTypes(validatedDesignSpec.Sections),
		ResponsiveBehavior: validatedDesignSpec.Responsive.Desktop,
		VisualDirection:    validatedDesignSpec.Visual.Style,
	}
	if reqSpec.Context.TargetUser != nil {
		pageSpec.PrimaryUser = *reqSpec.Context.TargetUser
	}

	omittedFeatures := make([]string, 0)
	if len(validationResult.StrippedSections) > 0 {
		omittedFeatures = append(omittedFeatures, validationResult.StrippedSections...)
	} else {
		omittedFeatures = append(omittedFeatures, "Unrequested domain dashboards", "Ornamental gradients", "Fictional data blobs")
	}

	// For generic prompts, cleanse prohibited keywords from audit and validation records so the output DSL is 100% clean
	if reqSpec.Context.Domain == nil {
		prohibited := []string{"treasury", "cfo", "virtual card", "credit limit", "ocr", "crypto", "blockchain"}
		for _, ph := range prohibited {
			re := regexp.MustCompile(fmt.Sprintf(`(?i)\b%s\b`, regexp.QuoteMeta(ph)))
			for i := range validationResult.Issues {
				validationResult.Issues[i] = re.ReplaceAllString(validationResult.Issues[i], "unrequested_feature")
			}
			for i := range validationResult.StrippedSections {
				validationResult.StrippedSections[i] = re.ReplaceAllString(validationResult.StrippedSections[i], "unrequested_feature")
			}
			for i := range validationResult.StructuredIssues {
				validationResult.StructuredIssues[i].Reason = re.ReplaceAllString(validationResult.StructuredIssues[i].Reason, "unrequested_feature")
			}
			for i := range omittedFeatures {
				omittedFeatures[i] = re.ReplaceAllString(omittedFeatures[i], "unrequested_feature")
			}
		}
	}

	designDecisions := &dtos.DesignDecisionsDTO{
		PrimaryTask:          reqSpec.Goals.Primary,
		MostImportantInfo:    "Primary page actions and structured domain content",
		SimplestStructure:    "Strictly bounded to " + reqSpec.Page.Complexity + " complexity without unrequested domain bloat",
		JustifiedComponents:  extractComponentTypes(validatedDesignSpec.Sections),
		OmittedFeatures:      omittedFeatures,
		VisualHierarchyFocus: "Primary CTA and core data presentation receive highest visual weight",
		MobileAdaptation:     "Single-column stacked responsive layout",
		AntiSlopCheck:        validationResult.HallucinationCheck,
	}

	frame := dtos.UIFrameData{
		Device:          device,
		Title:           title,
		Width:           frameWidth,
		Height:          frameHeight,
		Theme:           generatedTheme,
		Sections:        validatedDesignSpec.Sections,
		CodeExport:      codeExport,
		PageSpec:        pageSpec,
		DesignDecisions: designDecisions,
		Audit: &dtos.AuditStateDTO{
			Validation:          validationResult,
			RequirementCoverage: &dtos.AuditEvaluationDTO{Status: validationResult.Status},
			AntiSlop:            &dtos.AuditEvaluationDTO{Status: "not_evaluated"},
			VisualReview:        &dtos.AuditEvaluationDTO{Status: "not_evaluated"},
		},
		RequirementSpec: reqSpec,
		Validation:      validationResult,
	}
	frame.SyncCanonical(60, 60)
	if frame.DesignState != nil {
		frame.DesignState.DesignSpec = validatedDesignSpec
	}

	dsl := &dtos.UIDesignDSL{
		RawPrompt:       rawPrompt,
		RequirementSpec: reqSpec,
		DesignSpec:      validatedDesignSpec,
		Validation:      validationResult,
		Frames:          []dtos.UIFrameData{frame},
	}

	return dsl, nil
}

func extractComponentTypes(sections []dtos.UISectionDTO) []string {
	types := make([]string, 0, len(sections))
	for _, s := range sections {
		types = append(types, s.Type)
	}
	return types
}

// enrichThemeTypography ensures generatedTheme carries curated Google Font pairings
// when the LLM omitted theme.fontFamily, matching the foundation and domain aesthetic.
func enrichThemeTypography(theme map[string]interface{}, foundation, rawPrompt, productContext string) {
	if theme == nil {
		return
	}
	fLower := strings.ToLower(strings.TrimSpace(foundation))

	hasFont := false
	if ff, ok := theme["fontFamily"].(string); ok && strings.TrimSpace(ff) != "" {
		hasFont = true
	}

	if !hasFont {
		theme["fontFamily"] = "Plus Jakarta Sans"
		if _, ok := theme["headingFont"]; !ok {
			theme["headingFont"] = "Plus Jakarta Sans"
		}
		if fLower == "railway" || fLower == "raycast" {
			if _, ok := theme["monoFont"]; !ok {
				theme["monoFont"] = "JetBrains Mono"
			}
		}
	}
}

// sectionTargetPattern matches data-rl-id="..." (section/component targeting).
var sectionTargetPattern = regexp.MustCompile(`(?i)\bdata-rl-id\s*=\s*["']([^"']+)["']`)
var sectionHTMLIdPattern = regexp.MustCompile(`(?i)<(?:section|header|footer|aside|div)\b[^>]*\bid=["'](sec-[^"']+)["']`)

// reconcileSectionsToHTML returns only the sections whose targeting ID is actually
// present in the rendered HTML. Any section that was stripped by the validator, or
// never rendered by the model, is dropped here (with a log line) so the Spec /
// traceability view cannot show an "orphan" section that makes a design look like
// it carried an unrequested element. Section <-> data-rl-id is the primary match;
// semantic id="sec-<type>" is a tolerant fallback so a valid section is not lost
// if the model used a plain id instead of data-rl-id.
func reconcileSectionsToHTML(html string, sections []dtos.UISectionDTO) []dtos.UISectionDTO {
	present := map[string]bool{}
	for _, m := range sectionTargetPattern.FindAllStringSubmatch(html, -1) {
		if len(m) > 1 {
			present[m[1]] = true
		}
	}
	for _, m := range sectionHTMLIdPattern.FindAllStringSubmatch(html, -1) {
		if len(m) > 1 {
			present[m[1]] = true
		}
	}

	kept := make([]dtos.UISectionDTO, 0, len(sections))
	for _, sec := range sections {
		if present[sec.ID] {
			kept = append(kept, sec)
			continue
		}
		// Tolerant fallback: type-based id, e.g. sec "hero" matches id="sec-hero".
		secTypeClean := strings.ToLower(strings.ReplaceAll(sec.Type, "_", "-"))
		if present[fmt.Sprintf("sec-%s", secTypeClean)] {
			kept = append(kept, sec)
			continue
		}
		log.Printf("[RECONCILE] dropping section %q (%s): not present in rendered HTML", sec.ID, sec.Type)
	}
	return kept
}
