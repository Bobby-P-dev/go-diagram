package services

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/Bobby-P-dev/go-diagram.git/src/dtos"
)

type ChangeAnalyzer struct {
	aiService *AIService
}

func NewChangeAnalyzer(aiService *AIService) *ChangeAnalyzer {
	return &ChangeAnalyzer{aiService: aiService}
}

const changeAnalyzerPrompt = `You are a strict, objective AI Change Analyzer for an AI Design Compiler and Iteration Engine.

YOUR GOAL:
Understand WHAT the user wants to change, identify the SMALLEST affected scope, and create a structured Change Plan that PRESERVES everything not affected.

CHANGE CLASSIFICATIONS:
1. CONTENT: Text copy, title, subtitle, CTA label, placeholder. (e.g. "ubah judul", "ganti teks tombol")
2. STYLE: Local visual styling, size, padding, margin, border, rounded corner, button height. (e.g. "buat tombol login lebih kecil", "ubah warna tombol login")
3. LAYOUT: Spatial arrangement of sections or components. (e.g. "pindahkan form ke kanan", "tata letak horizontal")
4. COMPONENT: Adding or modifying a specific component inside an existing section. (e.g. "tambahkan input nomor telepon", "tambahkan link forgot password")
5. FUNCTIONALITY: Adding a functional capability. (e.g. "tambahkan login dengan Google")
6. STRUCTURAL: Fundamental redesign of page purpose. (e.g. "ubah login page menjadi dashboard admin")
7. GLOBAL_STYLE: Theme-wide token change. (e.g. "ubah seluruh desain menjadi dark mode", "ubah warna primary menjadi hitam")
8. PAGE_REBUILD: Explicit user request to start over. (e.g. "buat ulang dari awal")

STRATEGY DECISION RULES (CRITICAL):
- IF the change affects a single property, component, style, or text:
  * Strategy: "patch"
  * Regenerate: false
  * Scope: "component" or "property" or "section"
- IF the change is a theme/token update:
  * Strategy: "patch"
  * Regenerate: false
  * Scope: "global"
- IF the change alters the entire page structure or rebuilds it:
  * Strategy: "rebuild"
  * Regenerate: true
  * Scope: "page"

NEVER REGENERATE THE ENTIRE PAGE WHEN A LOCAL PATCH IS SUFFICIENT (CHANGE LOCALITY PRINCIPLE).

OUTPUT FORMAT (STRICT JSON ONLY):
{
  "request": "raw request string",
  "classification": "content" | "style" | "layout" | "component" | "functionality" | "structural" | "global_style" | "page_rebuild",
  "target": {
    "component": "name of target component or null",
    "section": "name or id of target section or null",
    "property": "name of target property or null"
  },
  "changes": [
    {
      "property": "property name (e.g. size, color, title, layout)",
      "from": "current value or empty",
      "to": "new desired value",
      "action": "update" | "add" | "remove"
    }
  ],
  "scope": "property" | "component" | "section" | "page" | "global",
  "strategy": "patch" | "rebuild" | "token_update",
  "preserve": [
    "list of unaffected elements guaranteed to remain untouched, e.g. layout, typography, inputs, background, card"
  ],
  "regenerate": false
}`

func (a *ChangeAnalyzer) AnalyzeChange(
	ctx context.Context,
	rawRequest string,
	currentFrame *dtos.UIFrameData,
) (*dtos.ChangePlanDTO, error) {
	return a.AnalyzeTargetedChange(ctx, rawRequest, nil, nil, currentFrame)
}

func (a *ChangeAnalyzer) AnalyzeTargetedChange(
	ctx context.Context,
	rawRequest string,
	targetRef *dtos.TargetElementRefDTO,
	selectionCtx *dtos.SelectionContextDTO,
	currentFrame *dtos.UIFrameData,
) (*dtos.ChangePlanDTO, error) {
	trimmed := strings.TrimSpace(rawRequest)
	lower := strings.ToLower(trimmed)

	// STRICT LOCALITY RULE 0: Explicit Section Insert / Delete / Reorder
	if strings.Contains(lower, "tambah") || strings.Contains(lower, "add section") || strings.Contains(lower, "insert section") {
		targetSec := ""
		if strings.Contains(lower, "setelah") {
			parts := strings.Split(lower, "setelah")
			if len(parts) > 1 {
				targetSec = strings.TrimSpace(parts[1])
			}
		} else if strings.Contains(lower, "after") {
			parts := strings.Split(lower, "after")
			if len(parts) > 1 {
				targetSec = strings.TrimSpace(parts[1])
			}
		}
		return &dtos.ChangePlanDTO{
			Request:        trimmed,
			Classification: "layout",
			Operation:      dtos.OpInsertSection,
			Target: dtos.TargetElementDTO{
				Type:      "section",
				SectionID: targetSec,
			},
			RequestedChanges:      []string{trimmed},
			Scope:                 "section",
			Strategy:              "insert_section",
			Preserve:              []string{"all existing sections"},
			PreserveOutsideTarget: true,
			Regenerate:            false,
		}, nil
	}

	if strings.Contains(lower, "hapus section") || strings.Contains(lower, "hapus seksi") || strings.Contains(lower, "delete section") || strings.Contains(lower, "remove section") {
		delTarget := ""
		if targetRef != nil && targetRef.ID != "" {
			delTarget = targetRef.ID
		} else {
			for _, kw := range []string{"testimonial", "review", "spotlight", "product", "hero", "footer"} {
				if strings.Contains(lower, kw) {
					delTarget = "sec-" + kw
					break
				}
			}
		}
		return &dtos.ChangePlanDTO{
			Request:        trimmed,
			Classification: "layout",
			Operation:      dtos.OpDeleteSection,
			Target: dtos.TargetElementDTO{
				Type:      "section",
				ID:        delTarget,
				SectionID: delTarget,
			},
			RequestedChanges:      []string{trimmed},
			Scope:                 "section",
			Strategy:              "delete_section",
			Preserve:              []string{"all other sections"},
			PreserveOutsideTarget: true,
			Regenerate:            false,
		}, nil
	}

	// STRICT LOCALITY RULE 1: User explicitly selected a component
	if targetRef != nil && (targetRef.Type == "component" || strings.HasPrefix(targetRef.ID, "cmp-")) {
		secID := targetRef.SectionID
		if secID == "" {
			secID = "sec-header"
		}
		classif := "style"
		if strings.Contains(lower, "teks") || strings.Contains(lower, "text") || strings.Contains(lower, "label") || strings.Contains(lower, "judul") || strings.Contains(lower, "kata") {
			classif = "content"
		} else if strings.Contains(lower, "kanan") || strings.Contains(lower, "kiri") || strings.Contains(lower, "tengah") || strings.Contains(lower, "posisi") {
			classif = "layout"
		}

		return &dtos.ChangePlanDTO{
			Request:        trimmed,
			Classification: classif,
			Operation:      dtos.OpComponentPatch,
			Target: dtos.TargetElementDTO{
				Type:        "component",
				ID:          targetRef.ID,
				ComponentID: targetRef.ID,
				SectionID:   secID,
			},
			RequestedChanges: []string{trimmed},
			Scope:            "component",
			Strategy:         "component_patch",
			Preserve: []string{
				"header layout",
				"logo",
				"navigation",
				"hero",
				"product sections",
				"page theme",
			},
			PreserveOutsideTarget: true,
			Regenerate:            false,
		}, nil
	}

	// STRICT LOCALITY RULE 2: User explicitly selected a section
	if targetRef != nil && (targetRef.Type == "section" || strings.HasPrefix(targetRef.ID, "sec-")) {
		return &dtos.ChangePlanDTO{
			Request:        trimmed,
			Classification: "layout",
			Operation:      dtos.OpSectionPatch,
			Target: dtos.TargetElementDTO{
				Type:      "section",
				ID:        targetRef.ID,
				SectionID: targetRef.ID,
			},
			RequestedChanges: []string{trimmed},
			Scope:            "section",
			Strategy:         "section_patch",
			Preserve: []string{
				"all sections outside target",
				"hero",
				"product sections",
				"footer",
				"page theme",
				"typography outside target",
			},
			PreserveOutsideTarget: true,
			Regenerate:            false,
		}, nil
	}

	// STRICT LOCALITY RULE 3: Implicit section targeting from prompt keywords (when target is unspecified)
	if strings.Contains(lower, "header") || strings.Contains(lower, "navbar") || strings.Contains(lower, "nav") {
		return &dtos.ChangePlanDTO{
			Request:        trimmed,
			Classification: "layout",
			Target: dtos.TargetElementDTO{
				Type:      "section",
				ID:        "sec-header",
				SectionID: "sec-header",
			},
			RequestedChanges: []string{trimmed},
			Scope:            "section",
			Strategy:         "section_patch",
			Preserve: []string{
				"all sections outside header",
				"hero",
				"products",
				"story",
				"testimonials",
				"footer",
				"page theme",
			},
			PreserveOutsideTarget: true,
			Regenerate:            false,
		}, nil
	}

	if strings.Contains(lower, "hero") {
		return &dtos.ChangePlanDTO{
			Request:        trimmed,
			Classification: "layout",
			Target: dtos.TargetElementDTO{
				Type:      "section",
				ID:        "sec-hero",
				SectionID: "sec-hero",
			},
			RequestedChanges: []string{trimmed},
			Scope:            "section",
			Strategy:         "section_patch",
			Preserve: []string{
				"header",
				"products",
				"story",
				"testimonials",
				"footer",
				"page theme",
			},
			PreserveOutsideTarget: true,
			Regenerate:            false,
		}, nil
	}

	if strings.Contains(lower, "produk") || strings.Contains(lower, "product") || strings.Contains(lower, "menu") || strings.Contains(lower, "katalog") {
		return &dtos.ChangePlanDTO{
			Request:        trimmed,
			Classification: "layout",
			Target: dtos.TargetElementDTO{
				Type:      "section",
				ID:        "sec-products",
				SectionID: "sec-products",
			},
			RequestedChanges: []string{trimmed},
			Scope:            "section",
			Strategy:         "section_patch",
			Preserve: []string{
				"header",
				"hero",
				"story",
				"testimonials",
				"footer",
				"page theme",
			},
			PreserveOutsideTarget: true,
			Regenerate:            false,
		}, nil
	}

	if strings.Contains(lower, "footer") {
		return &dtos.ChangePlanDTO{
			Request:        trimmed,
			Classification: "layout",
			Target: dtos.TargetElementDTO{
				Type:      "section",
				ID:        "sec-footer",
				SectionID: "sec-footer",
			},
			RequestedChanges: []string{trimmed},
			Scope:            "section",
			Strategy:         "section_patch",
			Preserve: []string{
				"all sections above footer",
				"header",
				"hero",
				"products",
				"page theme",
			},
			PreserveOutsideTarget: true,
			Regenerate:            false,
		}, nil
	}

	// 1. DETERMINISTIC FAST-PATH GUARDRAILS (High confidence matching for precision & speed)
	// Full Creative Freedom / Freeform Redesign (Zero locked elements):
	if strings.Contains(lower, "bebas") || strings.Contains(lower, "ekspresi") || strings.Contains(lower, "ubah desain") || strings.Contains(lower, "ganti desain") || strings.Contains(lower, "redesign") || strings.Contains(lower, "rombak") || strings.Contains(lower, "ganti konsep") || strings.Contains(lower, "tampilan baru") {
		return &dtos.ChangePlanDTO{
			Request:        trimmed,
			Classification: "structural",
			Target: dtos.TargetElementDTO{
				Property: "page_design",
			},
			Changes: []dtos.ChangeDetailDTO{
				{
					Property: "design",
					From:     "existing",
					To:       "freely_customized",
					Action:   "rebuild",
				},
			},
			Scope:      "page",
			Strategy:   "rebuild",
			Preserve:   []string{},
			Regenerate: true,
		}, nil
	}

	// Small change: Button size
	if strings.Contains(lower, "tombol") && (strings.Contains(lower, "kecil") || strings.Contains(lower, "besar") || strings.Contains(lower, "size")) ||
		strings.Contains(lower, "button") && (strings.Contains(lower, "smaller") || strings.Contains(lower, "bigger") || strings.Contains(lower, "size")) {
		return &dtos.ChangePlanDTO{
			Request:        trimmed,
			Classification: "style",
			Target: dtos.TargetElementDTO{
				Component:   "button",
				ComponentID: "cmp-login-button",
				Section:     "form",
				SectionID:   "sec-auth-form",
				Property:    "size",
			},
			Changes: []dtos.ChangeDetailDTO{
				{
					Property: "size",
					From:     "standard",
					To:       "small",
					Action:   "update",
				},
			},
			Scope:      "component",
			Strategy:   "patch",
			Preserve:   []string{"layout", "typography", "inputs", "background", "card", "colors", "tokens"},
			Regenerate: false,
		}, nil
	}

	// Small change: Primary color to black
	if (strings.Contains(lower, "warna") || strings.Contains(lower, "color")) && (strings.Contains(lower, "hitam") || strings.Contains(lower, "black")) {
		return &dtos.ChangePlanDTO{
			Request:        trimmed,
			Classification: "style",
			Target: dtos.TargetElementDTO{
				Component: "theme",
				Property:  "primary_color",
			},
			Changes: []dtos.ChangeDetailDTO{
				{
					Property: "primary_color",
					From:     "#6366f1",
					To:       "#000000",
					Action:   "update",
				},
			},
			Scope:      "global",
			Strategy:   "patch",
			Preserve:   []string{"layout", "typography", "form", "inputs", "sections", "content"},
			Regenerate: false,
		}, nil
	}

	// Medium change: Layout relocation (form ke kanan / ke kiri)
	if strings.Contains(lower, "form") && (strings.Contains(lower, "kanan") || strings.Contains(lower, "kiri") || strings.Contains(lower, "right") || strings.Contains(lower, "left")) {
		targetAlign := "right"
		if strings.Contains(lower, "kiri") || strings.Contains(lower, "left") {
			targetAlign = "left"
		}
		return &dtos.ChangePlanDTO{
			Request:        trimmed,
			Classification: "layout",
			Target: dtos.TargetElementDTO{
				Section:  "form",
				Property: "alignment",
			},
			Changes: []dtos.ChangeDetailDTO{
				{
					Property: "alignment",
					From:     "center",
					To:       targetAlign,
					Action:   "update",
				},
			},
			Scope:      "section",
			Strategy:   "patch",
			Preserve:   []string{"content", "typography", "button", "inputs", "colors", "tokens"},
			Regenerate: false,
		}, nil
	}

	// Global design system change: Dark mode
	if strings.Contains(lower, "dark mode") || strings.Contains(lower, "mode gelap") || strings.Contains(lower, "menjadi dark") {
		return &dtos.ChangePlanDTO{
			Request:        trimmed,
			Classification: "global_style",
			Target: dtos.TargetElementDTO{
				Property: "theme_mode",
			},
			Changes: []dtos.ChangeDetailDTO{
				{
					Property: "theme_mode",
					From:     "light",
					To:       "dark",
					Action:   "update",
				},
			},
			Scope:      "global",
			Strategy:   "patch",
			Preserve:   []string{"layout", "typography", "form", "inputs", "sections", "content", "functionality"},
			Regenerate: false,
		}, nil
	}

	// Feature addition: Google login explicitly requested
	if strings.Contains(lower, "google") && (strings.Contains(lower, "login") || strings.Contains(lower, "masuk") || strings.Contains(lower, "auth")) {
		return &dtos.ChangePlanDTO{
			Request:        trimmed,
			Classification: "functionality",
			Target: dtos.TargetElementDTO{
				Component: "social_login",
				Section:   "form",
			},
			Changes: []dtos.ChangeDetailDTO{
				{
					Property: "social_buttons",
					From:     "[]",
					To:       "['Google']",
					Action:   "add",
				},
			},
			Scope:      "component",
			Strategy:   "patch",
			Preserve:   []string{"layout", "typography", "card", "inputs", "colors", "background"},
			Regenerate: false,
		}, nil
	}

	// Major structural redesign: Login to Dashboard
	if (strings.Contains(lower, "dashboard") || strings.Contains(lower, "admin")) && !strings.Contains(lower, "warna") && !strings.Contains(lower, "tombol") {
		return &dtos.ChangePlanDTO{
			Request:        trimmed,
			Classification: "structural",
			Target: dtos.TargetElementDTO{
				Property: "page_type",
			},
			Changes: []dtos.ChangeDetailDTO{
				{
					Property: "page_type",
					From:     "login",
					To:       "dashboard",
					Action:   "rebuild",
				},
			},
			Scope:      "page",
			Strategy:   "rebuild",
			Preserve:   []string{"theme_mode", "primary_color"},
			Regenerate: true,
		}, nil
	}

	// Full Page Rebuild
	if strings.Contains(lower, "buat ulang") || strings.Contains(lower, "regenerate all") || strings.Contains(lower, "dari awal") {
		return &dtos.ChangePlanDTO{
			Request:        trimmed,
			Classification: "page_rebuild",
			Target: dtos.TargetElementDTO{
				Property: "full_page",
			},
			Changes: []dtos.ChangeDetailDTO{
				{
					Property: "full_page",
					Action:   "rebuild",
				},
			},
			Scope:      "page",
			Strategy:   "rebuild",
			Preserve:   []string{},
			Regenerate: true,
		}, nil
	}

	// 2. LLM FALLBACK FOR ARBITRARY CHANGES
	if a.aiService == nil {
		return &dtos.ChangePlanDTO{
			Request:        trimmed,
			Classification: "style",
			Scope:          "component",
			Strategy:       "patch",
			Preserve:       []string{"layout", "typography", "content"},
			Regenerate:     false,
		}, nil
	}

	sectionsSummary := "none"
	if currentFrame != nil && len(currentFrame.Sections) > 0 {
		var secNames []string
		for _, s := range currentFrame.Sections {
			secNames = append(secNames, fmt.Sprintf("%s (%s)", s.ID, s.Type))
		}
		sectionsSummary = strings.Join(secNames, ", ")
	}

	userMsg := fmt.Sprintf(`User Change Request: "%s"
Current Sections: %s
Current Device: %s

Analyze this change and return a strict JSON Change Plan adhering to Change Locality.`,
		trimmed, sectionsSummary, func() string {
			if currentFrame != nil && currentFrame.Device != "" {
				return currentFrame.Device
			}
			return "web"
		}())

	respText, err := a.aiService.CallLLM(changeAnalyzerPrompt, nil, userMsg)
	if err != nil {
		// Safe fallback: default to patch
		return &dtos.ChangePlanDTO{
			Request:        trimmed,
			Classification: "style",
			Scope:          "component",
			Strategy:       "patch",
			Preserve:       []string{"layout", "typography", "content"},
			Regenerate:     false,
		}, nil
	}

	cleanedJSON := a.aiService.SanitizeJSON(respText)
	var plan dtos.ChangePlanDTO
	if err := json.Unmarshal([]byte(cleanedJSON), &plan); err != nil {
		return &dtos.ChangePlanDTO{
			Request:        trimmed,
			Classification: "style",
			Scope:          "component",
			Strategy:       "patch",
			Preserve:       []string{"layout", "typography", "content"},
			Regenerate:     false,
		}, nil
	}

	plan.Request = trimmed
	ensureOperationType(&plan)
	return &plan, nil
}

func ensureOperationType(plan *dtos.ChangePlanDTO) {
	if plan == nil {
		return
	}
	if plan.Operation != "" {
		return
	}
	lowerReq := strings.ToLower(plan.Request)
	if strings.Contains(lowerReq, "tambah") || strings.Contains(lowerReq, "add section") || strings.Contains(lowerReq, "insert") {
		plan.Operation = dtos.OpInsertSection
		plan.Strategy = "insert_section"
		return
	}
	if strings.Contains(lowerReq, "hapus section") || strings.Contains(lowerReq, "delete section") || strings.Contains(lowerReq, "buang section") {
		plan.Operation = dtos.OpDeleteSection
		plan.Strategy = "delete_section"
		return
	}
	if strings.Contains(lowerReq, "pindahkan") || strings.Contains(lowerReq, "reorder") || strings.Contains(lowerReq, "tukar urutan") {
		plan.Operation = dtos.OpReorderSections
		plan.Strategy = "reorder_sections"
		return
	}
	if plan.Regenerate || plan.Strategy == "rebuild" || plan.Classification == "page_rebuild" || plan.Scope == "page" {
		plan.Operation = dtos.OpFullReplace
		return
	}
	if plan.Strategy == "component_patch" || plan.Scope == "component" || plan.Target.Type == "component" {
		plan.Operation = dtos.OpComponentPatch
		return
	}
	if plan.Strategy == "section_replace" {
		plan.Operation = dtos.OpSectionReplace
		return
	}
	plan.Operation = dtos.OpSectionPatch
}
