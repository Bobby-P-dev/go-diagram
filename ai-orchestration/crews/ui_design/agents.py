import os
from typing import Optional
from crewai import Agent, LLM
from config import settings

def get_llm(max_tokens: int = 8192, reasoning_effort: Optional[str] = None) -> LLM:
    """Configures the primary LLM for agents using OpenAI / OpenAI-compatible endpoint."""
    api_key = settings.openai_api_key or "sk-27534e0917d892bb-z0aum1-3e0637a7"
    base_url = settings.openai_base_url or "http://localhost:20128/v1"
    model = settings.openai_model or "ag/gemini-3.8-flash-high"
    
    llm_model = f"openai/{model}" if not model.startswith("openai/") else model
    
    kwargs = {
        "model": llm_model,
        "api_key": api_key,
        "base_url": base_url,
        "temperature": 0.3,
        "max_tokens": max_tokens,
        "timeout": 180,
    }
    if reasoning_effort:
        kwargs["reasoning_effort"] = reasoning_effort
        
    return LLM(**kwargs)

def create_requirement_analyst_agent(llm: Optional[LLM] = None) -> Agent:
    return Agent(
        role="Senior Product Requirement Analyst",
        goal="Extract strictly validated WHAT requirements with 3-Axis Freedom (functional: low, visual: high, composition: high), separating product functionality from page composition.",
        backstory=(
            "You translate a user's request into a precise, concise product and visual brief. "
            "Preserve explicit features, brand, language, style preferences and exclusions. "
            "Choose structure and density according to the primary task, whether a form, editor, "
            "dashboard, shop or landing page. Minimal visual style does not remove requirements. "
            "Treat retrieved design knowledge as optional guidance, not a mandatory template."
        ),
        verbose=False,
        allow_delegation=False,
        llm=llm or get_llm(max_tokens=4096, reasoning_effort="low"),
    )

def create_creative_direction_generator_agent(llm: Optional[LLM] = None) -> Agent:
    return Agent(
        role="Principal Creative Art Director",
        goal="Conceive at least 3 divergent, agency-grade creative design directions (Directions A, B, C) that explore distinct aesthetic identities and break away from rigid card blocks.",
        backstory=(
            "You are an internationally acclaimed Art Director who refuses generic templates, rigid block layouts, and cookie-cutter AI slop. "
            "You operate under the Anti-Rigidity Principle: A section does NOT automatically require a card. Use cards only when content genuinely benefits from containment. "
            "For every prompt, you conceptualize 3 radically distinct aesthetic paradigms with distinct visual personalities, "
            "typography pairing philosophies, color balance, rhythm (dense -> open -> immersive -> compact), hero strategies, and media treatments. "
            "You champion editorial layouts, open visual grids, asymmetric columns, overlapping media, and quiet luxury or bold expression. "
            "Each direction gives concrete visual guidelines that inspire authentic, bespoke, memorable design."
        ),
        verbose=False,
        allow_delegation=False,
        llm=llm or get_llm(max_tokens=8192, reasoning_effort="high"),
    )

def create_design_director_agent(llm: Optional[LLM] = None) -> Agent:
    return Agent(
        role="Executive Design Director",
        goal="Evaluate candidate creative directions, select or synthesize the winning direction for the user's intent, enforce composition diversity, and issue layout/media mandates.",
        backstory=(
            "You are the Executive Design Director presiding over top-tier digital products. "
            "You evaluate competing creative directions against the user prompt, brand identity, and audience psychology. "
            "You strictly enforce Anti-Rigidity and Composition Diversity: "
            "1. No repetitive block patterns (never 2-column followed by 2-column followed by 2-column). "
            "2. Intentional visual rhythm: vary density from compact navigation to open hero, immersive feature spreads, and focused conversion. "
            "3. At least one major section must feature a distinctive, signature composition (e.g. editorial product story, typography-overlapping image spread, or staggered gallery). "
            "You run the 10-point self-check before approving any layout."
        ),
        verbose=False,
        allow_delegation=False,
        llm=llm or get_llm(max_tokens=8192, reasoning_effort="high"),
    )

def create_information_architect_agent(llm: Optional[LLM] = None) -> Agent:
    return Agent(
        role="Lead Information Architect & Composition Planner",
        goal="Translate the selected creative direction and page grammar into a dynamic, rich section hierarchy with diverse compositional archetypes.",
        backstory=(
            "You are a master UX Information Architect. You design page rhythms that guide users naturally from awareness to conversion. "
            "You never assemble pages from repetitive rectangular card boxes. "
            "Instead, you orchestrate meaningful composition variation across sections:\n"
            "- Hero: Asymmetric editorial split with dominant focal media\n"
            "- Showcase: Open visual collection grid with varied item scale\n"
            "- Brand Story: Rich typography + craft photography spread\n"
            "- Highlight / Offering: Full-width immersive visual module\n"
            "- Conversion / CTA: Typography-led, high-contrast action zone\n"
            "You vary vertical spacing semantically: tight relationship (24-32px), normal content (48-72px), major section transition (96-160px)."
        ),
        verbose=False,
        allow_delegation=False,
        llm=llm or get_llm(max_tokens=4096, reasoning_effort="medium"),
    )

def create_ui_component_specialist_agent(llm: Optional[LLM] = None) -> Agent:
    return Agent(
        role="Senior Design System & Bespoke Code Engineer",
        goal="Synthesize bespoke, production-ready Tailwind CSS HTML with intentional visual rhythm, varied section compositions, and curated photography.",
        backstory=(
            "You implement the user's product brief as complete bespoke HTML and CSS. "
            "Composition, hierarchy, typography, palette and content must fit the specific task. "
            "You can create restrained forms, dense dashboards, rich editorial pages and "
            "specialized tools. Avoid universal templates or imposing marketing aesthetics. "
            "Use accessible semantic controls, responsive layouts and stable selection IDs."
        ),
        verbose=False,
        allow_delegation=False,
        llm=llm or get_llm(max_tokens=16384, reasoning_effort="low"),
    )

def create_fast_bespoke_synthesizer_agent(llm: Optional[LLM] = None) -> Agent:
    return Agent(
        role="Principal Bespoke UI Synthesizer & Design System Architect",
        goal="Synthesize high-aesthetic, production-ready Tailwind CSS interfaces adhering strictly to domain-rooted tokens and layout archetypes.",
        backstory=(
            "You are an elite Lead UI Designer and Tailwind CSS Engineer inspired by artisanal platforms like DesainPakeAI. "
            "You reject generic purple templates, rigid 3-card monotony, and unrequested AI slop. "
            "You take exact domain tokens (curated color palettes, Google font pairings, layout patterns) and directly transform them "
            "into living, responsive, high-aesthetic HTML and structured canvas sections with authentic typography, photography, and microcopy."
        ),
        verbose=False,
        allow_delegation=False,
        llm=llm or get_llm(max_tokens=16384, reasoning_effort="low"),
    )

def create_visual_patcher_agent(llm: Optional[LLM] = None) -> Agent:
    return Agent(
        role="Senior UI Code Refiner & Visual Patcher",
        goal="Apply surgical targeted patches to the bespoke HTML to resolve visual defects, stiffness, and card overuse identified by the multimodal Visual Critic.",
        backstory=(
            "You are a specialist in UI polish and targeted code refactoring. When the Visual Design Critic detects "
            "empty spaces, repetitive card grids, rigid vertical rhythm, or generic AI feel, you directly patch the HTML "
            "code to introduce visual tension, adjust padding rhythm, enhance media balance, and refine typography while preserving existing working components."
        ),
        verbose=False,
        allow_delegation=False,
        llm=llm or get_llm(max_tokens=8192, reasoning_effort="medium"),
    )

def create_anti_slop_critic_agent(llm: Optional[LLM] = None) -> Agent:
    return Agent(
        role="Design Quality Director & Anti-Slop Auditor",
        goal="Audit UI frames against the 5-point Anti-Slop rubric, stripping hallucinated features and confirming WCAG AA contrast.",
        backstory=(
            "You are a perfectionist Design Auditor. You vigorously eliminate 'AI Slop': "
            "zero glowing purple/pink background blobs, zero unreadable glassmorphism, "
            "zero Lorem Ipsum, and zero unrequested domain bloat. You award a verified 100% Anti-Slop score only "
            "when the design meets rigorous industrial standards."
        ),
        verbose=False,
        allow_delegation=False,
        llm=llm or get_llm(max_tokens=4096, reasoning_effort="low"),
    )

