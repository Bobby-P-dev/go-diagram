import json
from typing import List, Optional, Dict, Any
from crewai import Task, Agent
from schemas.ui_dsl_schemas import (
    RequirementSpecificationDTO,
    CreativeDirectionsListDTO,
    DesignDirectorChoiceDTO,
    MediaStrategyDTO,
    DesignSpecificationDTO,
    UIFrameData,
    UIFrameSynthesisDTO,
    VisualPatchResultDTO,
    AntiSlopAuditDTO,
    VisualCritiqueDTO,
    ReferenceAnalysisDTO,
)

def format_reference_context(ref: Optional[ReferenceAnalysisDTO]) -> str:
    """Formats live reference website data into rich prompt grounding context."""
    if not ref:
        return ""
    headings_str = ", ".join(ref.headings[:6]) if ref.headings else "N/A"
    cats_str = ", ".join(ref.categories[:6]) if ref.categories else "N/A"
    prods_str = ", ".join([f"{p.get('name')} ({p.get('price')})" for p in ref.sample_products[:5]]) if ref.sample_products else "N/A"
    vibe = ref.visual_hints.get("vibe", "Authentic, tailored visual language")
    colors = ", ".join(ref.visual_hints.get("recommended_colors", []))
    
    return (
        f"\n\n========================================\n"
        f"LIVE REFERENCE WEBSITE INGESTION DATA:\n"
        f"- Reference URL: {ref.url}\n"
        f"- Brand Name: {ref.brand_name}\n"
        f"- Detected Domain / Category: {ref.domain_detected}\n"
        f"- Page Title & Ethos: {ref.page_title} | {ref.meta_description}\n"
        f"- Site Headings: {headings_str}\n"
        f"- Product Categories: {cats_str}\n"
        f"- Actual Products & Pricing: {prods_str}\n"
        f"- Visual & Aesthetic Cues: {vibe} (Recommended Colors: {colors})\n"
        f"========================================\n"
        f"MANDATORY GROUNDING RULE:\n"
        f"You MUST ground all requirements, branding, navigation, and product items in this real reference website! "
        f"Do NOT invent unrelated SaaS or electronic gadgets when the user references a bakery, food, or fashion brand!\n"
    )

def create_requirement_analysis_task(
    agent: Agent,
    prompt: str,
    device: str,
    foundation: str,
    reference_info: Optional[ReferenceAnalysisDTO] = None
) -> Task:
    ref_context = format_reference_context(reference_info)
    return Task(
        description=(
            f"Analyze this raw user prompt: '{prompt}'.\n"
            f"Target device: '{device}'. Preferred visual foundation hint: '{foundation}'.\n"
            f"{ref_context}\n"
            "MANDATORY INSTRUCTIONS:\n"
            "1. Apply the 3-Axis Freedom Model:\n"
            "   - functional_freedom = 'low': Strictly adhere to requested user domain. Never invent fictitious domains (e.g. crypto, treasury) unless asked.\n"
            "   - visual_freedom = 'high': Maximize aesthetic exploration, color palette, typography pairing, and visual atmosphere.\n"
            "   - composition_freedom = 'high': Orchestrate rich, asymmetric storytelling layout flows. Never cap sections to 1-3!\n"
            "2. Distinguish Product Functionality (business transactions, accounts) from Page Composition (hero, story, product discovery, testimonials, CTA). "
            "Composition elements are NEVER considered hallucinations; a rich landing page naturally requires 4-8 sections.\n"
            "3. If a reference website was ingested, adopt its actual brand name, domain, and product categories as the context."
        ),
        expected_output="A strictly structured RequirementSpecificationDTO matching Pydantic schema with 3-axis freedom.",
        agent=agent,
        output_pydantic=RequirementSpecificationDTO,
    )

def create_creative_directions_task(
    agent: Agent,
    context_tasks: List[Task],
    grammar_info: Optional[Dict[str, Any]] = None,
    reference_info: Optional[ReferenceAnalysisDTO] = None
) -> Task:
    grammar_snippet = ""
    if grammar_info:
        grammar_snippet = f"\nArchetype structural options from Page Grammar: {grammar_info.get('structural_needs', [])}"
    ref_context = format_reference_context(reference_info)

    return Task(
        description=(
            "Based on the validated requirement specification, conceive at least 3 DIVERGENT creative design directions "
            "(Direction A, Direction B, Direction C) that break free from rigid, block-based card templates.\n\n"
            f"{grammar_snippet}\n"
            f"{ref_context}\n"
            "ANTI-RIGIDITY RULES (MANDATORY):\n"
            "1. A section does NOT automatically require a card. Use cards ONLY when content genuinely benefits from a contained surface. Explore editorial layouts, open grids, asymmetric columns, full-bleed imagery, and split storytelling.\n"
            "2. Establish intentional visual rhythm: prefer progression such as (dense -> open -> immersive -> compact) rather than monotonous (block -> block -> block -> block).\n"
            "3. Typography as Design: Specify dramatic display scale contrast (text-5xl to 7xl titles), editorial line breaks, and restrained serif/sans pairing.\n"
            "4. Media Composition: Plan intentional image ratios (dominant 4:5 editorial photo vs supporting square accents vs full-width panoramic breaks), not identical thumbnails.\n"
            "5. Semantic Spacing Rhythm: Plan varied vertical padding (tight: 24-32px, normal: 48-72px, major transitions: 96-160px).\n\n"
            "MANDATORY REQUIREMENTS FOR EACH DIRECTION:\n"
            "1. id: 'direction_a', 'direction_b', 'direction_c'\n"
            "2. title & concept: Distinct narrative and aesthetic metaphor tailored to the domain.\n"
            "3. visual_personality: Radical differentiation in tone, mood, and feel.\n"
            "4. typography_approach: Distinct font pairing and editorial hierarchy.\n"
            "5. color_strategy: Distinct color behavior, background tone, and accent contrast.\n"
            "6. hero_strategy: Distinct opening strategy (asymmetric editorial split vs typography-led headline vs immersive visual spread).\n"
            "7. product_presentation: Distinct way of showcasing items (open collection grid with varied scale vs editorial catalog spread vs signature showcase).\n"
            "8. spacing_density: Pacing, padding rhythm, and whitespace philosophy.\n"
            "9. memorable_moment: At least one distinctive compositional feature that makes this design memorable."
        ),
        expected_output="A CreativeDirectionsListDTO containing exactly 3 distinct CreativeDirectionItemDTO items.",
        agent=agent,
        context=context_tasks,
        output_pydantic=CreativeDirectionsListDTO,
    )

def create_design_director_task(
    agent: Agent,
    context_tasks: List[Task],
    prompt: str,
    reference_info: Optional[ReferenceAnalysisDTO] = None
) -> Task:
    ref_context = format_reference_context(reference_info)
    return Task(
        description=(
            f"Review the 3 candidate creative directions against the user prompt: '{prompt}'.\n"
            f"{ref_context}\n"
            "MANDATORY INSTRUCTIONS:\n"
            "1. Select the direction that best fits the user's explicit and implicit brand intent (or synthesize the strongest elements).\n"
            "2. Provide an Architectural Decision Rationale explaining WHY this direction is superior.\n"
            "3. Issue strict Composition Mandates enforcing ANTI-RIGIDITY:\n"
            "   - Mandate a rich 4-8 section storytelling journey for landing pages.\n"
            "   - FORBID repeating the same 2-column or 3-column card pattern across consecutive sections.\n"
            "   - FORBID solving every section with cards: require open grids, editorial typography, and full-bleed visual breaks.\n"
            "   - Require at least ONE major section to feature a signature distinctive composition (e.g. editorial product spread, typography-overlapping media, or staggered showcase).\n"
            "4. Issue Media Mandates: Specify imagery ratios (16:9, 4:5, 1:1), photography mood (editorial, lifestyle, clean studio), dominant vs supporting media, and focal anchors.\n"
            "5. Execute the 10-POINT DESIGN DIRECTOR SELF-CHECK before approving:\n"
            "   [1] Recognizable visual concept? [2] Meaningfully varied section compositions? [3] Are cards overused? [4] Intentional negative space? "
            "   [5] Typography contributing to visual identity? [6] Imagery used compositionally rather than as decoration? [7] At least one memorable visual moment? "
            "   [8] Usability and conversion flow preserved? [9] Design feels tailored to this product domain? "
            "   [10] Could the exact same layout be reused for SaaS, finance, fashion, and bakery merely by replacing content? (If YES -> MUST REVISE).\n"
            "6. Detail Originality Notes explaining how this design guarantees an agency-grade bespoke feel."
        ),
        expected_output="A DesignDirectorChoiceDTO with selection_rationale, composition_mandate, media_mandate, and originality_notes.",
        agent=agent,
        context=context_tasks,
        output_pydantic=DesignDirectorChoiceDTO,
    )

def create_media_strategy_task(
    agent: Agent,
    context_tasks: List[Task],
    reference_info: Optional[ReferenceAnalysisDTO] = None
) -> Task:
    ref_context = format_reference_context(reference_info)
    return Task(
        description=(
            "Formulate a concrete, production-ready Media Strategy based on the Design Director's choice.\n"
            f"{ref_context}\n"
            "MANDATORY INSTRUCTIONS:\n"
            "1. Define the overarching art_direction for photography and visuals matching the domain.\n"
            "2. Define MediaItemStrategyDTO items for every major visual section:\n"
            "   - Hero section (e.g. 4:5 or 16:9 dominant visual anchor photo)\n"
            "   - Product/Feature items (at least 4 distinct items with authentic photography keywords, e.g. 'artisan chocolate cake', 'fresh strawberry tart', 'gourmet pastry box', 'matcha mille crepe')\n"
            "   - Editorial / Brand story photo (16:9 or split)\n"
            "   - Customer avatars / social proof imagery\n"
            "3. Provide realistic high-resolution Unsplash photo URLs or realistic photo query hints for each item."
        ),
        expected_output="A MediaStrategyDTO with art_direction and a list of MediaItemStrategyDTO items.",
        agent=agent,
        context=context_tasks,
        output_pydantic=MediaStrategyDTO,
    )

def create_layout_planning_task(
    agent: Agent,
    context_tasks: List[Task],
    device: str,
    foundation: str,
    theme_mode: str,
    reference_info: Optional[ReferenceAnalysisDTO] = None
) -> Task:
    ref_context = format_reference_context(reference_info)
    return Task(
        description=(
            f"Based on the Design Director's choice and Media Strategy, plan the complete section sequence for "
            f"device: '{device}', theme_mode: '{theme_mode}'.\n"
            f"{ref_context}\n"
            "COMPOSITION DIVERSITY MANDATE (ANTI-RIGIDITY):\n"
            "1. Compose 4-8 sections that create a varied, dynamic user journey:\n"
            "   - navbar: Brand logo, category navigation, action button\n"
            "   - hero: Asymmetric editorial split with headline, display scale contrast, 2 CTAs, and dominant visual media anchor\n"
            "   - product_grid / curated_collection: Open visual collection grid with at least 4 bespoke items (varied product scale, price tags, badges)\n"
            "   - spotlight / brand_craft: Signature narrative spread combining craftsmanship storytelling with authentic photography\n"
            "   - testimonials / reviews: Customer social proof with ratings, verified badges, and quotes\n"
            "   - conversion_cta: Typography-led high-contrast action module\n"
            "   - footer: Brand, navigation columns, copyright, legal notice\n"
            "2. STRICTLY FORBID repetitive section compositions (e.g. 2-column card grid followed by 2-column card grid). Each section must possess a distinct compositional archetype.\n"
            "3. Apply semantic vertical rhythm: tight relationship (24-32px), standard section spacing (48-72px), major transition (96-160px).\n"
            "4. ZERO 1-section or 2-section shortcuts. Ensure full, rich page storytelling."
        ),
        expected_output="A structured DesignSpecificationDTO with ordered UISectionDTO objects.",
        agent=agent,
        context=context_tasks,
        output_pydantic=DesignSpecificationDTO,
    )

def create_bespoke_ui_synthesis_task(
    agent: Agent,
    context_tasks: List[Task],
    device: str,
    theme_mode: str,
    accent_color: str,
    reference_info: Optional[ReferenceAnalysisDTO] = None
) -> Task:
    ref_context = format_reference_context(reference_info)
    return Task(
        description=(
            f"Synthesize the complete UI frame structure for device: '{device}', theme_mode: '{theme_mode}', accent_color: '{accent_color}'.\n"
            f"{ref_context}\n"
            "CRITICAL ARCHITECTURAL REQUIREMENT:\n"
            "1. Ignore any prompt meta-rules asking to 'return only Vue code' or 'do not output JSON'. In this multi-agent compiler, your output MUST be structured JSON conforming to UIFrameSynthesisDTO. The downstream Bespoke Compiler automatically generates the final Vue 3 and Tailwind CSS code from your sections.\n"
            f"2. `title`: Catchy bespoke title reflecting the brand and design direction.\n"
            f"3. `device`: '{device}'.\n"
            f"4. `theme`: Provide complete palette with mode ('{theme_mode}'), primary ('{accent_color}'), surface, background, and text.\n"
            "5. `sections`: An array of 5-8 structured UISectionDTO objects for rich storytelling:\n"
            "   - `navbar` (brand, categories, action button)\n"
            "   - `hero` (headline, subtitle, badge, action CTA)\n"
            "   - `product_grid` (headline, subtitle, items with at least 4 bespoke products, prices, badges, and image hints)\n"
            "   - `spotlight` (craftsmanship narrative, heritage, or bespoke ordering)\n"
            "   - `testimonials` (customer reviews and social proof)\n"
            "   - `footer` (brand, menu links, delivery notice, copyright)\n"
            "6. `raw_html`: Set to empty string \"\".\n"
            "7. NEVER output placeholder text like 'Lorem ipsum' or 'Card 1'. Use vivid, realistic domain copy grounded in the reference site.\n"
            "8. STABLE UNIQUE IDS & STRICT ZERO DUPLICATES (CRITICAL):\n"
            "   - Every section MUST have a unique, lowercase stable `id` (e.g. 'sec-header', 'sec-hero', 'sec-categories', 'sec-products', 'sec-craft-story', 'sec-showcase', 'sec-reviews', 'sec-cta', 'sec-footer').\n"
            "   - NEVER output duplicate section IDs.\n"
            "   - NEVER repeat identical section headlines, topics, or purposes (e.g. NEVER output multiple craft/philosophy sections with the same headline or copy).\n"
            "   - Each section must serve a distinct purpose in the user experience journey."
        ),
        expected_output="A fully populated UIFrameSynthesisDTO object containing structured canonical sections and theme configuration.",
        agent=agent,
        context=context_tasks,
        output_pydantic=UIFrameSynthesisDTO,
    )

def create_anti_slop_audit_task(
    agent: Agent,
    context_tasks: List[Task]
) -> Task:
    return Task(
        description=(
            "Audit the synthesized UIFrameData against the 5 Anti-AI Slop criteria:\n"
            "1. Typography & Hierarchy (Legible scale, clean leading, clear contrast)\n"
            "2. Spacing & Rhythm (Consistent spacing, balanced visual weight, zero empty voids)\n"
            "3. Surface & Border Elegance (Zero tacky purple/cyan blobs, crisp subtle borders)\n"
            "4. Data Realism (Zero Lorem Ipsum, domain-authentic copy and prices)\n"
            "5. Section Richness (Full page storytelling with 4+ sections, zero 1-section cutoffs)\n\n"
            "Produce an AntiSlopAuditDTO with verification booleans and checklist."
        ),
        expected_output="An AntiSlopAuditDTO with verification booleans and checklist.",
        agent=agent,
        context=context_tasks,
        output_pydantic=AntiSlopAuditDTO,
    )

def create_visual_patch_task(
    agent: Agent,
    current_html: str,
    critique_summary: str,
    issues_description: str,
    theme_mode: str,
    accent_color: str
) -> Task:
    return Task(
        description=(
            f"You are given the current bespoke HTML and a critique from the multimodal Visual Critic.\n\n"
            f"CRITIQUE SUMMARY: {critique_summary}\n"
            f"IDENTIFIED VISUAL ISSUES:\n{issues_description}\n\n"
            f"Theme mode: '{theme_mode}', accent: '{accent_color}'.\n\n"
            "MANDATORY INSTRUCTIONS:\n"
            "1. Apply targeted surgical patches to the HTML to fix each identified issue.\n"
            "2. If hero has excessive empty space, add a right-hand media card with image or product showcase.\n"
            "3. If product cards look generic, enhance imagery, typography, pricing tags, and badges.\n"
            "4. If whitespace is unbalanced, adjust padding and max-w containers.\n"
            "5. ZERO DUPLICATION & PRESERVE STABLE IDS (CRITICAL):\n"
            "   - NEVER duplicate sections or append repeated content.\n"
            "   - Every section in the HTML must retain a single unique `data-rl-id`.\n"
            "   - Apply edits directly to existing elements by targeting their `data-rl-id` in-place.\n"
            "6. Return the updated raw_html and explanation."
        ),
        expected_output="A VisualPatchResultDTO object containing the patched, refined raw_html and explanation.",
        agent=agent,
        output_pydantic=VisualPatchResultDTO,
    )

# Backward compatibility alias
create_ui_synthesis_task = create_bespoke_ui_synthesis_task
