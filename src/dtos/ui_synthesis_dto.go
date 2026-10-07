package dtos

// UIDesignSynthesisDTO is the single-request wire type returned by the merged
// analyze + plan + implement LLM call. Keeping it separate from the rich
// canonical DTOs preserves modularity: the compiler reconstructs the canonical
// RequirementSpecificationDTO / DesignSpecificationDTO from it and then runs
// the deterministic anti-slop guards before assembly.
type UIDesignSynthesisDTO struct {
	Title         string                 `json:"title"`
	Page          RequirementPageDTO     `json:"page"`
	Context       RequirementContextDTO  `json:"context"`
	Goals         RequirementGoalsDTO    `json:"goals"`
	Requirements  RequirementsListDTO    `json:"requirements"`
	DesignFreedom *DesignFreedomDTO      `json:"design_freedom,omitempty"`
	Layout        DesignLayoutDTO        `json:"layout"`
	Visual        DesignVisualDTO        `json:"visual"`
	Typography    DesignTypographyDTO    `json:"typography"`
	Responsive    DesignResponsiveDTO    `json:"responsive"`
	States        []string               `json:"states,omitempty"`
	Sections      []UISectionDTO         `json:"sections"`
	RawHTML       string                 `json:"raw_html"`
	Theme         map[string]interface{} `json:"theme"`
}