package services

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
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

	// STRICT LOCALITY RULE -1: Add a NEW page/frame to the project (distinct from
	// adding a section inside the current page). Triggers on explicit phrasing OR
	// when the frontend chat scope is explicitly set to "+ Screen Baru" (target=page / scope=new_screen).
	isExplicitNewFrameScope := (targetRef != nil && (targetRef.Type == "page" || targetRef.Type == "screen" || targetRef.Type == "frame")) ||
		(selectionCtx != nil && (selectionCtx.Scope == "new_screen" || selectionCtx.Scope == "screen_new")) ||
		strings.HasPrefix(lower, "screen baru:") ||
		strings.HasPrefix(lower, "halaman baru:") ||
		strings.HasPrefix(lower, "tambah screen") ||
		strings.HasPrefix(lower, "tambahkan screen") ||
		strings.HasPrefix(lower, "buat screen") ||
		strings.HasPrefix(lower, "buatkan screen")

	pageType, device, isNewFrame := detectInsertFrameIntent(lower)
	if isNewFrame || isExplicitNewFrameScope {
		if pageType == "" {
			pageType = detectInsertFramePageType(lower)
			if pageType == "" {
				pageType = "new_page"
			}
		}
		if device == "" {
			if currentFrame != nil && currentFrame.Device != "" {
				device = currentFrame.Device
			} else {
				device = "web"
			}
		}
		return &dtos.ChangePlanDTO{
			Request:        trimmed,
			Classification: "structural",
			Operation:      dtos.OpInsertFrame,
			Target: dtos.TargetElementDTO{
				Type:      "page",
				Property:  "new_page",
				SectionID: pageType,
				Device:    device,
			},
			RequestedChanges:      []string{trimmed},
			Scope:                 "project",
			Strategy:              "insert_frame",
			Preserve:              []string{"all existing frames"},
			PreserveOutsideTarget: true,
			Regenerate:            true,
		}, nil
	}

	// STRICT LOCALITY RULE 0: Explicit Section Insert / Delete / Reorder
	sectionFeatureKeywords := []string{
		"timeline", "jadwal", "roadmap", "riwayat",
		"faq", "tanya jawab", "pertanyaan",
		"testimoni", "testimonial", "review", "ulasan",
		"pricing", "harga", "paket", "biaya",
		"fitur", "features", "keunggulan",
		"tentang", "about", "story", "cerita",
		"kontak", "contact", "hubungi",
		"galeri", "gallery", "portfolio", "portofolio",
		"tim", "team", "struktur",
		"partner", "klien", "client", "sponsor", "rekanan",
		"cta", "call to action",
		"newsletter", "langganan",
		"statistik", "stats", "angka", "metrik",
		"alur", "langkah", "steps", "cara kerja", "how it works",
	}

	hasAddVerb := strings.Contains(lower, "tambah") ||
		strings.Contains(lower, "buatkan juga") ||
		strings.Contains(lower, "tambahkan juga") ||
		strings.Contains(lower, "buatkan") ||
		strings.Contains(lower, "buat") ||
		strings.Contains(lower, "bikin") ||
		strings.Contains(lower, "bikinkan") ||
		strings.Contains(lower, "insert") ||
		strings.Contains(lower, "add") ||
		strings.Contains(lower, "sisipkan") ||
		strings.Contains(lower, "sertakan")

	hasSectionNoun := strings.Contains(lower, "section") ||
		strings.Contains(lower, "seksi") ||
		strings.Contains(lower, "bagian")

	hasFeatureNoun := false
	for _, kw := range sectionFeatureKeywords {
		if strings.Contains(lower, kw) {
			hasFeatureNoun = true
			break
		}
	}

	isDeleteVerb := strings.Contains(lower, "hapus") ||
		strings.Contains(lower, "delete") ||
		strings.Contains(lower, "remove") ||
		strings.Contains(lower, "buang") ||
		strings.Contains(lower, "hilangkan")

	// Guard against item-level addition inside an already selected section
	isItemLevelAdd := false
	if targetRef != nil && targetRef.ID != "" && strings.HasPrefix(targetRef.ID, "sec-") {
		targetPrefix := strings.TrimPrefix(targetRef.ID, "sec-")
		if strings.Contains(lower, targetPrefix) && (strings.Contains(lower, "item") || strings.Contains(lower, "kartu") || strings.Contains(lower, "card") || strings.Contains(lower, "tombol") || strings.Contains(lower, "button")) {
			isItemLevelAdd = true
		}
	}

	isSectionInsert := false
	if !isDeleteVerb && !isItemLevelAdd {
		if reExplicitSectionInsert.MatchString(lower) ||
			(hasAddVerb && (hasSectionNoun || hasFeatureNoun || strings.Contains(lower, "menu "))) ||
			((strings.Contains(lower, "tambah") || strings.Contains(lower, "insert")) && (strings.Contains(lower, "setelah") || strings.Contains(lower, "after"))) {
			isSectionInsert = true
		}
	}

	if isSectionInsert {
		targetSec := ""
		if targetRef != nil && targetRef.ID != "" && strings.HasPrefix(targetRef.ID, "sec-") {
			targetSec = targetRef.ID
		}
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
				ID:        targetSec,
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

	isSectionDelete := strings.Contains(lower, "hapus section") || strings.Contains(lower, "hapus seksi") ||
		strings.Contains(lower, "delete section") || strings.Contains(lower, "remove section") ||
		strings.Contains(lower, "buang section") || strings.Contains(lower, "hilangkan section")
	if isSectionDelete {
		delTarget := ""
		if targetRef != nil && targetRef.ID != "" {
			delTarget = targetRef.ID
		} else {
			for _, kw := range []string{"testimonial", "review", "spotlight", "product", "hero", "footer", "faq", "features", "pricing"} {
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

	if strings.Contains(lower, "produk") || strings.Contains(lower, "product") || strings.Contains(lower, "katalog") || strings.Contains(lower, "daftar menu") || strings.Contains(lower, "menu makanan") || strings.Contains(lower, "menu resto") {
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

	if strings.Contains(lower, "timeline") || strings.Contains(lower, "jadwal") || strings.Contains(lower, "roadmap") {
		return &dtos.ChangePlanDTO{
			Request:        trimmed,
			Classification: "layout",
			Target: dtos.TargetElementDTO{
				Type:      "section",
				ID:        "sec-timeline",
				SectionID: "sec-timeline",
			},
			RequestedChanges: []string{trimmed},
			Scope:            "section",
			Strategy:         "section_patch",
			Preserve: []string{
				"header",
				"hero",
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
	// If explicit insert frame
	if plan.Strategy == "insert_frame" || plan.Scope == "project" {
		plan.Operation = dtos.OpInsertFrame
		return
	}
	// Section addition ONLY when explicitly targeting a section
	if (strings.Contains(lowerReq, "section") || strings.Contains(lowerReq, "seksi")) &&
		(strings.Contains(lowerReq, "tambah") || strings.Contains(lowerReq, "buat") || strings.Contains(lowerReq, "bikin") || strings.Contains(lowerReq, "add") || strings.Contains(lowerReq, "insert") || strings.Contains(lowerReq, "create")) {
		plan.Operation = dtos.OpInsertSection
		plan.Strategy = "insert_section"
		return
	}
	if strings.Contains(lowerReq, "hapus section") || strings.Contains(lowerReq, "delete section") || strings.Contains(lowerReq, "buang section") || strings.Contains(lowerReq, "hapus seksi") {
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

var (
	// Verbs indicating creation/addition of a new entity
	insertFrameActionVerbs = `(?:tambah(?:kan)?|menambahkan|buat(?:kan)?|membuat|bikin(?:kan)?|create|add|generate|insert|new)`

	// Nouns representing a canvas frame / screen / page
	insertFramePageNouns = `(?:halaman(?:nya)?|screen(?:nya|s)?|page(?:nya|s)?|frame(?:nya|s)?|layar(?:nya)?)`

	// Direct creation: e.g. "tambahkan halaman", "buatkan screen", "add page", "tambah login page", "buatkan register screen"
	reDirectFrameInsert = regexp.MustCompile(fmt.Sprintf(
		`(?i)\b%s\s+(?:(?:sebuah|suatu|1|satu|desain|design|tampilan)\s+)?(?:[a-z0-9_-]+\s+){0,2}%s\b`,
		insertFrameActionVerbs, insertFramePageNouns,
	))

	// Noun followed by "baru", "kedua", "ketiga", "lain", "selanjutnya", "berikutnya", "new", "another", "second", "next"
	// e.g. "halaman baru", "screen baru", "page baru", "halaman kedua", "new page"
	reNewPagePost = regexp.MustCompile(fmt.Sprintf(
		`(?i)\b%s\s+(?:baru|kedua|ke-2|ke\s+2|ketiga|ke-3|lain|selanjutnya|berikutnya|new|another|second|next)\b`,
		insertFramePageNouns,
	))

	// Noun followed by "untuk" / "buat" / "for" e.g. "halaman baru untuk profil", "halaman untuk login"
	rePageFor = regexp.MustCompile(fmt.Sprintf(
		`(?i)\b(?:%s\s+)?%s\s+(?:baru\s+)?(?:untuk|buat|for)\s+[a-z0-9_-]+`,
		insertFrameActionVerbs, insertFramePageNouns,
	))

	// Adding another/additional frame: e.g. "tambah 1 halaman lagi", "tambah page lagi"
	rePageAgain = regexp.MustCompile(fmt.Sprintf(
		`(?i)\b%s\s+(?:(?:satu|1)\s+)?%s\s+lagi\b`,
		insertFrameActionVerbs, insertFramePageNouns,
	))

	// Section exclusions: if the prompt is explicitly targeting a section
	reSectionAction = regexp.MustCompile(`(?i)\b(?:tambah(?:kan)?|buat(?:kan)?|bikin(?:kan)?|add|create|insert|hapus|delete|remove)\s+(?:seksi|section)\b`)

	// Explicit section insert intent
	reExplicitSectionInsert = regexp.MustCompile(`(?i)\b(?:tambah(?:kan)?|buat(?:kan)?|bikin(?:kan)?|add|create|insert|sisipkan)\s+(?:seksi|section)\b`)

	// Prepositional guard: e.g. "ke halaman", "di halaman", "pada screen", "to page", "in frame"
	// where something else is being added into the page
	reComponentIntoPage = regexp.MustCompile(fmt.Sprintf(
		`(?i)\b(?:tombol|button|input|field|form|card|kartu|gambar|image|foto|teks|text|icon|ikon|navbar|header|footer|tabel|table|link|modal|popup)\b.*?\b(?:di|ke|pada|dalam|to|into|in|on)\s+%s\b`,
		insertFramePageNouns,
	))

	// Structural rebuild verbs that modify existing page rather than adding a new one
	reRebuildPage = regexp.MustCompile(fmt.Sprintf(
		`(?i)\b(?:ubah|ganti|convert|rebuild|jadikan)\b.*?\b%s\b.*?\b(?:menjadi|jadi|into|to)\b`,
		insertFramePageNouns,
	))
)

// newFrameMarkers are explicit phrasings that clearly ask for a NEW page/frame (kept as secondary backup).
var newFrameMarkers = []string{
	"tambah halaman", "tambah screen", "tambah frame", "tambah page",
	"tambahkan halaman", "tambahkan screen", "tambahkan frame", "tambahkan page",
	"buat halaman", "buat screen", "buat frame", "buat page",
	"buatkan halaman", "buatkan screen", "buatkan frame", "buatkan page",
	"bikin halaman", "bikin screen", "bikin frame", "bikin page",
	"create page", "create screen", "add page", "add screen", "add frame",
	"new page", "new screen", "new frame",
	"tambah design", "buat design", "tambah desain", "buat desain",
	"tambah projek", "buat projek",
}

// insertFramePageTypeMarkers are recognized page-type keywords used to build the
// new frame's design hint (title + synthesis prompt).
var insertFramePageTypeMarkers = map[string]string{
	"pricing":       "pricing",
	"harga":         "pricing",
	"register":      "register",
	"registrasi":    "register",
	"signup":        "register",
	"daftar":        "register",
	"dashboard":     "dashboard",
	"profil":        "profile",
	"profile":       "profile",
	"login":         "login",
	"masuk":         "login",
	"auth":          "login",
	"landing":       "landing",
	"home":          "landing",
	"beranda":       "landing",
	"checkout":      "checkout",
	"bayar":         "checkout",
	"pembayaran":    "checkout",
	"cart":          "cart",
	"keranjang":     "cart",
	"table":         "data_table",
	"tabel":         "data_table",
	"invoice":       "invoice",
	"billing":       "billing",
	"tagihan":       "billing",
	"settings":      "settings",
	"pengaturan":    "settings",
	"setelan":       "settings",
	"onboarding":    "onboarding",
	"account":       "account",
	"akun":          "account",
	"detail":        "detail",
	"produk":        "product",
	"product":       "product",
	"analytics":     "analytics",
	"analitik":      "analytics",
	"marketplace":   "marketplace",
	"kontak":        "contact",
	"contact":       "contact",
	"faq":           "faq",
	"tentang":       "about",
	"about":         "about",
	"order":         "order",
	"pesanan":       "order",
	"transaksi":     "transaction",
	"transaction":   "transaction",
	"katalog":       "catalog",
	"catalog":       "catalog",
	"timeline":      "timeline",
	"jadwal":        "timeline",
	"roadmap":       "timeline",
}

// detectInsertFrameIntent returns true when the request explicitly asks for a NEW
// page/frame. It also derives a page-type hint and a device (mobile/desktop/web)
// from the prompt. Falling back conservatively avoids mis-classifying patches.
func detectInsertFrameIntent(lower string) (pageType, device string, ok bool) {
	// Guard 1: If prompt explicitly targets a section, it's not a frame insertion
	if reSectionAction.MatchString(lower) {
		return "", "", false
	}
	// Guard 2: If a component is being placed into the existing page
	if reComponentIntoPage.MatchString(lower) {
		return "", "", false
	}
	// Guard 3: If modifying/rebuilding existing page into another
	if reRebuildPage.MatchString(lower) {
		return "", "", false
	}

	explicit := false
	if reDirectFrameInsert.MatchString(lower) ||
		reNewPagePost.MatchString(lower) ||
		rePageFor.MatchString(lower) ||
		rePageAgain.MatchString(lower) {
		explicit = true
	} else {
		for _, m := range newFrameMarkers {
			if strings.Contains(lower, m) {
				explicit = true
				break
			}
		}
	}

	if !explicit {
		return "", "", false
	}

	pageType = detectInsertFramePageType(lower)
	if pageType == "" {
		pageType = "new_page"
	}
	device = "web"
	if strings.Contains(lower, "mobile") || strings.Contains(lower, "smartphone") || strings.Contains(lower, "ponsel") || strings.Contains(lower, "hp ") || strings.HasSuffix(lower, "hp") {
		device = "mobile"
	} else if strings.Contains(lower, "desktop") {
		device = "desktop"
	}
	return pageType, device, true
}

func detectInsertFramePageType(lower string) string {
	for kw, norm := range insertFramePageTypeMarkers {
		if strings.Contains(lower, kw) {
			return norm
		}
	}
	return ""
}
