from typing import Optional, List, Dict, Any
from pydantic import BaseModel, Field

class StructuredRequirementItemDTO(BaseModel):
    id: str
    description: str
    confidence: Optional[float] = None

class RequirementSourceDTO(BaseModel):
    type: str = Field(description="explicit | implied | design")
    id: Optional[str] = None

class UIComponentDTO(BaseModel):
    id: str
    type: str
    name: Optional[str] = None
    label: Optional[str] = None
    purpose: Optional[str] = None
    requirement_source: Optional[RequirementSourceDTO] = None
    style: Optional[Dict[str, Any]] = None
    data: Optional[Dict[str, Any]] = None

class UISectionDTO(BaseModel):
    id: str
    type: str = Field(description="navbar | hero | kpi_grid | data_table | form | pricing | etc.")
    purpose: Optional[str] = None
    requirement_source: Optional[str] = None
    requirement_source_obj: Optional[RequirementSourceDTO] = None
    priority: Optional[str] = None
    components: Optional[List[UIComponentDTO]] = None
    data: Optional[Dict[str, Any]] = None

class RequirementPageDTO(BaseModel):
    type: str = "homepage"
    purpose: str = ""
    complexity: str = "moderate"

class RequirementContextDTO(BaseModel):
    domain: Optional[str] = None
    target_user: Optional[str] = None

class RequirementGoalsDTO(BaseModel):
    primary: str
    secondary: Optional[List[str]] = None

class RequirementsListDTO(BaseModel):
    explicit: List[str] = []
    implied: List[str] = []
    optional: List[str] = []
    excluded: Optional[List[str]] = None
    structured_explicit: Optional[List[StructuredRequirementItemDTO]] = None
    structured_implied: Optional[List[StructuredRequirementItemDTO]] = None
    structured_optional: Optional[List[StructuredRequirementItemDTO]] = None
    structured_excluded: Optional[List[StructuredRequirementItemDTO]] = None

class DesignFreedomDTO(BaseModel):
    functional: str = "low"
    visual: str = "high"
    composition: str = "high"

class RequirementSpecificationDTO(BaseModel):
    raw_prompt: str
    page: RequirementPageDTO
    context: RequirementContextDTO
    goals: RequirementGoalsDTO
    requirements: RequirementsListDTO
    constraints: List[str] = []
    visual_preferences: List[str] = []
    design_freedom: Optional[DesignFreedomDTO] = None
    responsive: bool = True

class DesignPageDTO(BaseModel):
    type: str = "homepage"
    purpose: str = ""
    complexity: str = "moderate"

class DesignLayoutDTO(BaseModel):
    type: str = "single_column"
    container: str = "max-w-7xl"
    alignment: str = "center"

class DesignVisualDTO(BaseModel):
    style: str = "modern"
    foundation: str = "ramp"
    palette: str = "dark"
    accent: str = "#6366f1"

class DesignTypographyDTO(BaseModel):
    scale: str = "standard"
    heading_weight: str = "font-bold"
    body_weight: str = "font-normal"

class DesignResponsiveDTO(BaseModel):
    mobile: str = "stack_vertical"
    desktop: str = "grid_columns"

class DesignDecisionItemDTO(BaseModel):
    id: str
    decision: str
    reason: str
    source: str = "design_system"

class DesignSpecificationDTO(BaseModel):
    page: DesignPageDTO
    layout: DesignLayoutDTO
    visual: DesignVisualDTO
    typography: DesignTypographyDTO
    sections: List[UISectionDTO]
    responsive: DesignResponsiveDTO
    design_decisions: Optional[List[DesignDecisionItemDTO]] = None
    states: Optional[List[str]] = None

class AntiSlopAuditDTO(BaseModel):
    zero_ornamental_gradients: bool = True
    zero_fake_blobs: bool = True
    zero_lorem_ipsum: bool = True
    zero_unrequested_features: bool = True
    subtle_borders_only: bool = True
    wcag_contrast_passed: bool = True
    verified_rules: List[str] = []

class PageSpecificationDTO(BaseModel):
    page_name: str
    purpose: str
    primary_user: Optional[str] = None
    primary_goal: str
    layout: str
    info_hierarchy: Optional[List[str]] = None
    required_components: Optional[List[str]] = None
    interactions: Optional[List[str]] = None
    states: Optional[List[str]] = None
    responsive_behavior: Optional[str] = None
    visual_direction: Optional[str] = None

class DesignDecisionsDTO(BaseModel):
    primary_task: str
    most_important_info: str
    simplest_structure: str
    justified_components: Optional[List[str]] = None
    omitted_features: Optional[List[str]] = None
    visual_hierarchy_focus: str
    mobile_adaptation: str
    anti_slop_check: str

class CreativeDirectionItemDTO(BaseModel):
    id: str = Field(description="e.g. direction_a, direction_b, direction_c")
    title: str = Field(description="Descriptive title of the creative direction")
    concept: str = Field(description="Core visual and product concept narrative")
    visual_personality: str = Field(description="Personality tone, e.g. Editorial Premium, Brutalist Bold, Clean Minimal")
    composition_philosophy: str = Field(description="Philosophy governing rhythm, asymmetry, and section transitions")
    hero_strategy: str = Field(description="How the hero section captures attention and frames the value")
    product_presentation: str = Field(description="How products/features are presented and discovered")
    typography_approach: str = Field(description="Font scale, pairing philosophy, heading weights")
    color_strategy: str = Field(description="Color behavior, contrast accents, background shades")
    media_strategy: str = Field(description="Imagery style, ratios, and visual framing")
    spacing_density: str = Field(description="Whitespace rhythm, padding, layout density")
    interaction_character: str = Field(description="Hover micro-interactions and interactive feel")

class CreativeDirectionsListDTO(BaseModel):
    directions: List[CreativeDirectionItemDTO] = Field(description="At least 3 distinct creative directions")

class DesignDirectorChoiceDTO(BaseModel):
    selected_direction_id: str = Field(description="ID of the chosen or synthesized direction")
    selection_rationale: str = Field(description="Architectural rationale for choosing this direction")
    composition_mandate: str = Field(description="Specific mandates for layout, section count, and rhythm")
    media_mandate: str = Field(description="Mandate for photography, artwork, and aspect ratios")
    originality_notes: str = Field(description="How this design avoids generic AI template cliches")

class MediaItemStrategyDTO(BaseModel):
    section_id: str
    media_type: str = Field(description="editorial_photo | studio_product | lifestyle | abstract_minimal | chart_preview")
    aspect_ratio: str = Field(description="16:9 | 4:5 | 1:1 | 3:4 | 21:9")
    placement: str = Field(description="split_right | full_bleed | card_grid | background | floating")
    visual_treatment: str = Field(description="high_contrast | warm_neutral | clean_white | monochrome")
    query_hint: str = Field(description="Search keywords for authentic photography, e.g. 'luxury leather sneakers'")
    sample_image_url: Optional[str] = None

class MediaStrategyDTO(BaseModel):
    art_direction: str
    items: List[MediaItemStrategyDTO] = []

class VisualIssueDTO(BaseModel):
    target: str = Field(description="Section or element ID with defect, e.g. 'sec-hero'")
    type: str = Field(description="composition | whitespace | hierarchy | media | contrast | generic_ai_feel | composition_repetition | rigid_rhythm | card_overuse")
    severity: str = Field(description="high | medium | low")
    problem: str = Field(description="Concise description of the visual defect")
    fix_direction: str = Field(description="Concrete actionable instruction to resolve the defect")

class VisualCritiqueDTO(BaseModel):
    status: str = Field(description="pass | revise")
    composition_score: float = Field(default=8.5, description="Score 0.0 - 10.0")
    visual_hierarchy_score: float = Field(default=8.5)
    whitespace_balance_score: float = Field(default=8.5)
    distinctiveness_score: float = Field(default=8.5)
    overall_visual_score: float = Field(default=8.5)
    has_excessive_empty_space: bool = False
    has_generic_template_feel: bool = False
    critique_summary: str = "Visual composition meets high quality standards."
    issues: List[VisualIssueDTO] = []

class ReferenceAnalysisDTO(BaseModel):
    url: str
    brand_name: Optional[str] = None
    domain_detected: Optional[str] = None
    page_title: Optional[str] = None
    meta_description: Optional[str] = None
    headings: List[str] = []
    categories: List[str] = []
    sample_products: List[Dict[str, Any]] = []
    visual_hints: Dict[str, Any] = {}
    summary: str = ""

class UIFrameSynthesisDTO(BaseModel):
    id: Optional[str] = "ui-frame-1"
    device: str = "web"
    title: str
    width: int = 1024
    height: int = 720
    theme: Dict[str, Any]
    sections: List[UISectionDTO] = []
    raw_html: Optional[str] = None

class VisualPatchResultDTO(BaseModel):
    raw_html: str
    explanation: Optional[str] = "Targeted visual patch applied."

class UIFrameData(BaseModel):
    id: Optional[str] = "ui-frame-1"
    device: str = "web"
    title: str
    width: int = 1024
    height: int = 720
    theme: Dict[str, Any]
    sections: List[UISectionDTO] = []
    code_export: Optional[Dict[str, str]] = None
    raw_html: Optional[str] = None
    screenshot_base64: Optional[str] = None
    reference_analysis: Optional[ReferenceAnalysisDTO] = None
    creative_directions: Optional[List[CreativeDirectionItemDTO]] = None
    selected_creative_direction: Optional[CreativeDirectionItemDTO] = None
    design_director_choice: Optional[DesignDirectorChoiceDTO] = None
    media_strategy: Optional[MediaStrategyDTO] = None
    visual_critique: Optional[VisualCritiqueDTO] = None
    page_spec: Optional[PageSpecificationDTO] = None
    design_decisions: Optional[DesignDecisionsDTO] = None
    anti_slop_audit: Optional[AntiSlopAuditDTO] = None
    requirement_spec: Optional[RequirementSpecificationDTO] = None
    execution_trace: Optional[Dict[str, Any]] = None

class UIDesignDSL(BaseModel):
    frames: List[UIFrameData]
    metadata: Optional[Dict[str, Any]] = None
