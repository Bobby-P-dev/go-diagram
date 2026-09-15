package dtos

import "encoding/json"

// Canonical Requirement Item with ID
type StructuredRequirementItemDTO struct {
	ID          string  `json:"id"`
	Description string  `json:"description"`
	Confidence  float64 `json:"confidence,omitempty"`
}

// Canonical Requirement Source Link (Traceability)
type RequirementSourceDTO struct {
	Type string `json:"type"` // "explicit", "implied", "design"
	ID   string `json:"id,omitempty"`   // e.g. "req-auth-inputs"
}

// Canonical Stable Component
type UIComponentDTO struct {
	ID                string                `json:"id"`                // e.g. "cmp-login-button"
	Type              string                `json:"type"`              // "input", "button", "heading", "link", etc.
	Name              string                `json:"name,omitempty"`
	Label             string                `json:"label,omitempty"`
	Purpose           string                `json:"purpose,omitempty"`
	RequirementSource *RequirementSourceDTO `json:"requirement_source,omitempty"`
	Style             map[string]interface{}`json:"style,omitempty"`
	Data              map[string]interface{}`json:"data,omitempty"`
}

// Canonical Design Decision
type DesignDecisionItemDTO struct {
	ID       string `json:"id"`
	Decision string `json:"decision"`
	Reason   string `json:"reason"`
	Source   string `json:"source"` // "design_system", "visual_interpretation", "user_constraint"
}

// Canonical Canvas Presentation State
type CanvasPositionDTO struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

type CanvasStateDTO struct {
	Position CanvasPositionDTO `json:"position"`
	Device   string            `json:"device"`
	Width    int               `json:"width"`
	Height   int               `json:"height"`
	Title    string            `json:"title"`
}

// Canonical Design State (WHAT & HOW)
type DesignStateDTO struct {
	RequirementSpec *RequirementSpecificationDTO `json:"requirement_spec"`
	DesignSpec      *DesignSpecificationDTO      `json:"design_spec"`
}

// Canonical Implementation State (CODE)
type ImplementationSourceDTO struct {
	Vue  string `json:"vue"`
	HTML string `json:"html"`
}

type ImplementationStateDTO struct {
	Framework   string                  `json:"framework"` // "vue"
	Styling     string                  `json:"styling"`   // "tailwind"
	Source      ImplementationSourceDTO `json:"source"`
	GeneratedAt string                  `json:"generated_at,omitempty"`
	Version     int                     `json:"version,omitempty"`
}

// Canonical Audit State (EVALUATION ONLY)
type AuditEvaluationDTO struct {
	Status  string                 `json:"status"` // "pass", "repaired", "fail"
	Details map[string]interface{} `json:"details,omitempty"`
}

type AuditStateDTO struct {
	Validation          *UIValidationResultDTO `json:"validation,omitempty"`
	RequirementCoverage *AuditEvaluationDTO    `json:"requirement_coverage,omitempty"`
	AntiSlop            *AuditEvaluationDTO    `json:"anti_slop,omitempty"`
	VisualReview        *AuditEvaluationDTO    `json:"visual_review,omitempty"`
}


type CreateUIDesignRequest struct {
	Prompt      string `json:"prompt"`
	Device      string `json:"device"` // "web", "mobile", "desktop", "all", "multi"
	Theme       string `json:"theme"`  // Preset name or description
	ThemeMode   string `json:"theme_mode,omitempty"`   // "dark", "light"
	AccentColor string `json:"accent_color,omitempty"` // Custom hex color (e.g. #10b981)
	CustomTone  string `json:"custom_tone,omitempty"`  // Freeform user style/vibe descriptor
	Foundation  string `json:"foundation,omitempty"` // "ramp", "calcom", "raycast", "railway", "attio", "mintlify"
	Archetype      string `json:"archetype,omitempty"`       // "dashboard", "booking", "pos", "kanban", "landing", "mobile"
	Density        string `json:"density,omitempty"`         // "compact", "balanced", "generous"
	ProductContext string `json:"product_context,omitempty"` // "erp", "saas", "crm", "ecommerce", "fintech", "healthcare", "internal_app", "marketing", "dashboard", "mobile"
	PrimaryUser    string `json:"primary_user,omitempty"`    // e.g. "Purchasing Officer", "Clinic Doctor", "Store Manager"
	PrimaryTask    string `json:"primary_task,omitempty"`    // e.g. "Create and approve purchase requests"
	TemplateID     string `json:"template_id,omitempty"`
}

type PageSpecificationDTO struct {
	PageName           string   `json:"page_name"`
	Purpose            string   `json:"purpose"`
	PrimaryUser        string   `json:"primary_user"`
	PrimaryGoal        string   `json:"primary_goal"`
	Layout             string   `json:"layout"`
	InfoHierarchy      []string `json:"info_hierarchy,omitempty"`
	RequiredComponents []string `json:"required_components,omitempty"`
	Interactions       []string `json:"interactions,omitempty"`
	States             []string `json:"states,omitempty"`
	ResponsiveBehavior string   `json:"responsive_behavior,omitempty"`
	VisualDirection    string   `json:"visual_direction,omitempty"`
}

type DesignDecisionsDTO struct {
	PrimaryTask          string   `json:"primary_task"`
	MostImportantInfo    string   `json:"most_important_info"`
	SimplestStructure    string   `json:"simplest_structure"`
	JustifiedComponents  []string `json:"justified_components,omitempty"`
	OmittedFeatures      []string `json:"omitted_features,omitempty"`
	VisualHierarchyFocus string   `json:"visual_hierarchy_focus"`
	MobileAdaptation     string   `json:"mobile_adaptation"`
	AntiSlopCheck        string   `json:"anti_slop_check"`
}

type AntiSlopAuditDTO struct {
	ZeroOrnamentalGradients bool     `json:"zero_ornamental_gradients"`
	ZeroFakeBlobs           bool     `json:"zero_fake_blobs"`
	ZeroLoremIpsum          bool     `json:"zero_lorem_ipsum"`
	ZeroUnrequestedFeatures bool     `json:"zero_unrequested_features"`
	SubtleBordersOnly       bool     `json:"subtle_borders_only"`
	WCAGContrastPassed      bool     `json:"wcag_contrast_passed"`
	VerifiedRules           []string `json:"verified_rules,omitempty"`
}

type UISectionDTO struct {
	ID                   string                 `json:"id"`
	Type                 string                 `json:"type"`
	Purpose              string                 `json:"purpose,omitempty"`
	RequirementSource    string                 `json:"requirement_source,omitempty"`
	RequirementSourceObj *RequirementSourceDTO  `json:"requirement_source_obj,omitempty"`
	Priority             string                 `json:"priority,omitempty"`
	Components           []UIComponentDTO       `json:"components,omitempty"`
	Data                 map[string]interface{} `json:"data,omitempty"`
}

// ============================================================================
// AI DESIGN COMPILER ARCHITECTURE DTOS
// ============================================================================

type RequirementSpecificationDTO struct {
	RawPrompt         string                         `json:"raw_prompt"`
	Explicit          []StructuredRequirementItemDTO `json:"explicit,omitempty"`
	Implied           []StructuredRequirementItemDTO `json:"implied,omitempty"`
	Optional          []StructuredRequirementItemDTO `json:"optional,omitempty"`
	Excluded          []StructuredRequirementItemDTO `json:"excluded,omitempty"`
	Page              RequirementPageDTO             `json:"page"`
	Context           RequirementContextDTO `json:"context"`
	Goals             RequirementGoalsDTO   `json:"goals"`
	Requirements      RequirementsListDTO   `json:"requirements"`
	Constraints       []string              `json:"constraints"`
	VisualPreferences []string              `json:"visual_preferences"`
	DesignFreedom     *DesignFreedomDTO     `json:"design_freedom,omitempty"`
	Responsive        bool                  `json:"responsive"`
}

type DesignFreedomDTO struct {
	Functional string `json:"functional"` // "low", "medium"
	Visual     string `json:"visual"`     // "low", "medium", "high"
}

type RequirementPageDTO struct {
	Type       string `json:"type"`       // "homepage", "dashboard", "login", "checkout", "detail", "profile", etc.
	Purpose    string `json:"purpose"`
	Complexity string `json:"complexity"` // "simple", "moderate", "complex"
}

type RequirementContextDTO struct {
	Domain     *string `json:"domain"`      // MUST be null if user did not explicitly provide one
	TargetUser *string `json:"target_user"` // MUST be null if user did not explicitly provide one
}

type RequirementGoalsDTO struct {
	Primary   string   `json:"primary"`
	Secondary []string `json:"secondary,omitempty"`
}

type RequirementsListDTO struct {
	Explicit           []string                       `json:"explicit"`
	Implied            []string                       `json:"implied"`
	Optional           []string                       `json:"optional"`
	Excluded           []string                       `json:"excluded,omitempty"`
	StructuredExplicit []StructuredRequirementItemDTO `json:"structured_explicit,omitempty"`
	StructuredImplied  []StructuredRequirementItemDTO `json:"structured_implied,omitempty"`
	StructuredOptional []StructuredRequirementItemDTO `json:"structured_optional,omitempty"`
	StructuredExcluded []StructuredRequirementItemDTO `json:"structured_excluded,omitempty"`
}

type DesignSpecificationDTO struct {
	Page            DesignPageDTO           `json:"page"`
	Layout          DesignLayoutDTO         `json:"layout"`
	Visual          DesignVisualDTO         `json:"visual"`
	Typography      DesignTypographyDTO     `json:"typography"`
	Sections        []UISectionDTO          `json:"sections"`
	Responsive      DesignResponsiveDTO     `json:"responsive"`
	DesignDecisions []DesignDecisionItemDTO `json:"design_decisions,omitempty"`
	States          []string                `json:"states,omitempty"`
}

type DesignPageDTO struct {
	Type       string `json:"type"`
	Purpose    string `json:"purpose"`
	Complexity string `json:"complexity"`
}

type DesignLayoutDTO struct {
	Type      string `json:"type"`
	Container string `json:"container"`
	Alignment string `json:"alignment"`
	Spacing   string `json:"spacing"`
}

type DesignVisualDTO struct {
	Style   string `json:"style"`
	Density string `json:"density"`
	Theme   string `json:"theme"`
}

type DesignTypographyDTO struct {
	Heading  string `json:"heading"`
	Body     string `json:"body"`
	Metadata string `json:"metadata"`
}

type DesignResponsiveDTO struct {
	Desktop string `json:"desktop"`
	Tablet  string `json:"tablet"`
	Mobile  string `json:"mobile"`
}

type ValidationScoreDTO struct {
	RequirementFidelity int `json:"requirement_fidelity"`
	ScopeAccuracy       int `json:"scope_accuracy"`
	Traceability        int `json:"traceability"`
	Simplicity          int `json:"simplicity"`
	Hierarchy           int `json:"hierarchy"`
	VisualConsistency   int `json:"visual_consistency"`
	ResponsiveQuality   int `json:"responsive_quality"`
	HallucinationSafety int `json:"hallucination_safety"`
	// Backward compatibility aliases
	Scope               int `json:"scope,omitempty"`
	Consistency         int `json:"consistency,omitempty"`
}

type ValidationIssueDTO struct {
	Type      string `json:"type"`      // "optional_feature_drift", "domain_drift", "content_drift", "hallucination", etc.
	Component string `json:"component"` // Section ID or field name
	Action    string `json:"action"`    // "remove", "replace", "prune", "neutralize"
	Reason    string `json:"reason"`
}

type UIValidationResultDTO struct {
	Status             string                `json:"status"` // "pass", "repaired", "fail"
	Score              ValidationScoreDTO    `json:"score"`
	Issues             []string              `json:"issues,omitempty"`
	StrippedSections   []string              `json:"stripped_sections,omitempty"`
	StructuredIssues   []ValidationIssueDTO  `json:"structured_issues,omitempty"`
	HallucinationCheck string                `json:"hallucination_check"`
}


type ChangeDetailDTO struct {
	Property string `json:"property"`
	From     string `json:"from,omitempty"`
	To       string `json:"to,omitempty"`
	Action   string `json:"action"`
}

type TargetElementDTO struct {
	Component   string `json:"component,omitempty"`
	ComponentID string `json:"component_id,omitempty"` // Stable ID e.g. "cmp-login-button"
	Section     string `json:"section,omitempty"`
	SectionID   string `json:"section_id,omitempty"`   // Stable ID e.g. "sec-auth-form"
	Property    string `json:"property,omitempty"`
}

type ChangePlanDTO struct {
	Request        string            `json:"request"`
	Classification string            `json:"classification"` // "content" | "style" | "layout" | "component" | "functionality" | "structural" | "global_style" | "page_rebuild"
	Target         TargetElementDTO  `json:"target"`
	Changes        []ChangeDetailDTO `json:"changes"`
	Scope          string            `json:"scope"`    // "property" | "component" | "section" | "page" | "global"
	Strategy       string            `json:"strategy"` // "patch" | "rebuild" | "token_update"
	Preserve       []string          `json:"preserve"` // Elements explicitly preserved
	Regenerate     bool              `json:"regenerate"`
}

type UIDesignDSL struct {
	ChangePlan      *ChangePlanDTO                `json:"change_plan,omitempty"`
	RawPrompt       string                       `json:"raw_prompt"`
	RequirementSpec *RequirementSpecificationDTO `json:"requirement_spec"`
	DesignSpec      *DesignSpecificationDTO      `json:"design_spec"`
	Validation      *UIValidationResultDTO       `json:"validation"`
	Frames          []UIFrameData                `json:"frames"`
}

type UIFrameData struct {
	ID              string                       `json:"id,omitempty"`
	Type            string                       `json:"type,omitempty"`
	Canvas          *CanvasStateDTO              `json:"canvas,omitempty"`
	DesignState     *DesignStateDTO              `json:"design_state,omitempty"`
	Implementation  *ImplementationStateDTO      `json:"implementation,omitempty"`
	Audit           *AuditStateDTO               `json:"audit,omitempty"`

	// Backwards compatibility legacy fields (maintained and synced):
	ChangePlan      *ChangePlanDTO                `json:"change_plan,omitempty"`
	Device          string                       `json:"device,omitempty"`
	Title           string                       `json:"title,omitempty"`
	Width           int                          `json:"width,omitempty"`
	Height          int                          `json:"height,omitempty"`
	Theme           map[string]interface{}       `json:"theme,omitempty"`
	RawHtml         string                       `json:"raw_html,omitempty"`
	Sections        []UISectionDTO               `json:"sections,omitempty"`
	CodeExport      map[string]string            `json:"code_export,omitempty"`
	PageSpec        *PageSpecificationDTO        `json:"page_spec,omitempty"`
	DesignDecisions *DesignDecisionsDTO          `json:"design_decisions,omitempty"`
	AntiSlopAudit   *AntiSlopAuditDTO            `json:"anti_slop_audit,omitempty"`
	RequirementSpec *RequirementSpecificationDTO `json:"requirement_spec,omitempty"`
	Validation      *UIValidationResultDTO       `json:"validation,omitempty"`
}

func (f *UIFrameData) SyncCanonical(posX, posY float64) {
	if f.Canvas == nil {
		w := f.Width
		h := f.Height
		if w == 0 {
			if f.Device == "mobile" {
				w, h = 375, 812
			} else {
				w, h = 1024, 720
			}
		}
		f.Canvas = &CanvasStateDTO{
			Position: CanvasPositionDTO{X: posX, Y: posY},
			Device:   f.Device,
			Width:    w,
			Height:   h,
			Title:    f.Title,
		}
	}
	if f.DesignState != nil && f.DesignState.RequirementSpec != nil {
		rs := f.DesignState.RequirementSpec
		if len(rs.Explicit) == 0 && len(rs.Requirements.StructuredExplicit) > 0 {
			rs.Explicit = rs.Requirements.StructuredExplicit
		}
		if len(rs.Implied) == 0 && len(rs.Requirements.StructuredImplied) > 0 {
			rs.Implied = rs.Requirements.StructuredImplied
		}
		if len(rs.Optional) == 0 && len(rs.Requirements.StructuredOptional) > 0 {
			rs.Optional = rs.Requirements.StructuredOptional
		}
		if len(rs.Excluded) == 0 && len(rs.Requirements.StructuredExcluded) > 0 {
			rs.Excluded = rs.Requirements.StructuredExcluded
		}
	}
	if f.RequirementSpec != nil {
		if len(f.RequirementSpec.Explicit) == 0 && len(f.RequirementSpec.Requirements.StructuredExplicit) > 0 {
			f.RequirementSpec.Explicit = f.RequirementSpec.Requirements.StructuredExplicit
		}
		if len(f.RequirementSpec.Implied) == 0 && len(f.RequirementSpec.Requirements.StructuredImplied) > 0 {
			f.RequirementSpec.Implied = f.RequirementSpec.Requirements.StructuredImplied
		}
	}
	if f.DesignState == nil && (f.RequirementSpec != nil || len(f.Sections) > 0) {
		themeFamily := "indigo"
		if f.Theme != nil {
			if prim, ok := f.Theme["primary"].(string); ok {
				themeFamily = prim
			}
		}
		mode := "dark"
		if f.Theme != nil {
			if m, ok := f.Theme["mode"].(string); ok {
				mode = m
			}
		}
		f.DesignState = &DesignStateDTO{
			RequirementSpec: f.RequirementSpec,
			DesignSpec: &DesignSpecificationDTO{
				Page: DesignPageDTO{
					Type:       f.Title,
					Purpose:    "Render requested screen",
					Complexity: "simple",
				},
				Layout: DesignLayoutDTO{
					Type: "single_column",
				},
				Visual: DesignVisualDTO{
					Style: "modern",
					Theme: mode,
				},
				Sections: f.Sections,
			},
		}
		_ = themeFamily
	}
	if f.Implementation == nil && f.CodeExport != nil {
		f.Implementation = &ImplementationStateDTO{
			Framework: "vue",
			Styling:   "tailwind",
			Source: ImplementationSourceDTO{
				Vue:  f.CodeExport["vue"],
				HTML: f.CodeExport["html"],
			},
			Version: 1,
		}
	}
	if f.Audit == nil && f.Validation != nil {
		f.Audit = &AuditStateDTO{
			Validation: f.Validation,
			RequirementCoverage: &AuditEvaluationDTO{Status: f.Validation.Status},
			AntiSlop:            &AuditEvaluationDTO{Status: "pass"},
			VisualReview:        &AuditEvaluationDTO{Status: "pass"},
		}
	}
}

type UIDesignTemplateDTO struct {
	ID          string          `json:"id"`
	Name        string          `json:"name"`
	Category    string          `json:"category"`
	Device      string          `json:"device"`
	Description string          `json:"description"`
	Theme       json.RawMessage `json:"theme"`
	Sections    json.RawMessage `json:"sections"`
	CodeExport  json.RawMessage `json:"code_export"`
	IsFeatured  bool            `json:"is_featured"`
}
