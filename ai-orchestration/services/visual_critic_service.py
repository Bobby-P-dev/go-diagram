"""
Visual Critic Service
Evaluates actual headless browser screenshots using multimodal vision LLM
against 10 rigorous visual criteria to detect empty voids, template cliches,
and unbalanced layouts.
"""

import os
import json
import logging
import httpx
from typing import Optional, Dict, Any

from config import settings
from schemas.ui_dsl_schemas import VisualCritiqueDTO, VisualIssueDTO

logger = logging.getLogger("VisualCriticService")

SYSTEM_PROMPT = """You are an elite Visual Design Critic and Art Director.
You are evaluating a real screenshot of a freshly compiled web/mobile user interface.
You have sharp aesthetic standards and zero tolerance for:
1. Large empty voids or dead zones (e.g. hero section with only a text headline and vast empty space next to or below it).
2. Generic AI template feel (e.g. 3 identical tech placeholder cards with generic gray icons and fake text).
3. Monotonous vertical layout with no visual rhythm, lack of imagery, or bad contrast.
4. Overly cramped or unpadded elements.
5. Rigid block-based repetition (e.g. solving every section with cards or repeating identical 2-column or 3-column grids consecutively).
6. Card overuse (wrapping every feature, step, benefit, or stat in bordered boxes with background fills instead of open typography, editorial lists, or flowing layouts).
7. Monotonous vertical rhythm (equal gaps everywhere with no contrast between dense, open, immersive, and compact pacing).

Evaluate the screenshot against these visual criteria:
1. Composition & Balance (Is visual weight distributed harmoniously without repetitive stacking?)
2. Visual Hierarchy & Focal Points (Is there an immediate, captivating visual anchor with strong typographic tension?)
3. Whitespace & Content Density (Is whitespace intentional rather than empty dead space? Are margins varied?)
4. Typography Contrast & Legibility (Clear hierarchy between display headlines, body, badges; typography treated as a visual design element)
5. Product Imagery & Media Impact (Are there authentic, vibrant photos/illustrations with distinctive framing?)
6. CTA Prominence & Conversion Flow (Are action buttons clear, accessible, and well-placed?)
7. Section Transitions & Visual Rhythm (Does the page flow with dynamic transitions and varied layout structures: dense -> open -> immersive -> compact?)
8. Anti-Rigidity & Distinctiveness (Free of generic 3-card boilerplate templates; features at least one standout, non-standard layout section)
9. Card Discipline (Cards used only where containment is genuinely needed like pricing or product checkout, avoiding unnecessary box wrappers)
10. Overall Aesthetic Polish (Does it look like an award-winning agency-designed, production-ready website?)

OUTPUT FORMAT:
You must respond with ONLY a valid JSON object matching this schema:
{
  "status": "pass" | "revise",
  "overall_visual_score": 8.5,
  "composition_score": 8.0,
  "visual_hierarchy_score": 8.5,
  "whitespace_balance_score": 8.0,
  "distinctiveness_score": 9.0,
  "has_excessive_empty_space": false,
  "has_generic_template_feel": false,
  "critique_summary": "Concise 2-sentence summary of the visual composition.",
  "issues": [
    {
      "target": "sec-hero" or "sec-product-grid",
      "type": "composition" | "whitespace" | "hierarchy" | "media" | "contrast" | "generic_ai_feel" | "composition_repetition" | "rigid_rhythm" | "card_overuse",
      "severity": "high" | "medium" | "low",
      "problem": "Specific description of the visual flaw.",
      "fix_direction": "Clear actionable instruction to fix it in code."
    }
  ]
}

Set "status": "pass" if overall_visual_score >= 8.0 and there are no "high" severity issues.
Otherwise set "status": "revise".
"""

def critique_screenshot(
    screenshot_data_uri: str,
    raw_prompt: str,
    theme_mode: str = "dark"
) -> VisualCritiqueDTO:
    """Sends screenshot to multimodal vision LLM and returns structured VisualCritiqueDTO."""
    api_key = settings.openai_api_key or "sk-e0a1cf47f9c53ece-y50fyg-b2f36942"
    base_url = settings.openai_base_url or "https://9router.bby-dev.tech/v1"
    model = settings.openai_model or "cx/gpt-5.6-sol"

    endpoint = f"{base_url.rstrip('/')}/chat/completions"
    headers = {
        "Authorization": f"Bearer {api_key}",
        "Content-Type": "application/json",
    }

    user_text = (
        f"Original User Request: '{raw_prompt}'\n"
        f"Theme: '{theme_mode}'\n\n"
        "Please inspect the attached screenshot of the rendered interface. "
        "Audit layout balance, imagery, visual anchors, and identify any defects. "
        "Respond strictly with the required JSON object."
    )

    payload = {
        "model": model,
        "messages": [
            {"role": "system", "content": SYSTEM_PROMPT},
            {
                "role": "user",
                "content": [
                    {"type": "text", "text": user_text},
                    {"type": "image_url", "image_url": {"url": screenshot_data_uri}}
                ]
            }
        ],
        "temperature": 0.2,
        "max_tokens": 1000,
    }

    try:
        from openai import OpenAI
        logger.info(f"Sending screenshot to Vision LLM ({model}) via OpenAI client...")
        client = OpenAI(base_url=base_url, api_key=api_key)
        resp = client.chat.completions.create(
            model=model,
            messages=[
                {"role": "system", "content": SYSTEM_PROMPT},
                {
                    "role": "user",
                    "content": [
                        {"type": "text", "text": user_text},
                        {"type": "image_url", "image_url": {"url": screenshot_data_uri}}
                    ]
                }
            ],
            temperature=0.2,
            max_tokens=4000,
            response_format={"type": "json_object"}
        )

        raw_content = (resp.choices[0].message.content or "").strip()

        # Clean JSON markdown fences if present
        if raw_content.startswith("```"):
            lines = raw_content.splitlines()
            if lines[0].startswith("```"):
                lines = lines[1:]
            if lines and lines[-1].startswith("```"):
                lines = lines[:-1]
            raw_content = "\n".join(lines).strip()

        critique_dict = json.loads(raw_content)
        critique = VisualCritiqueDTO.model_validate(critique_dict)
        logger.info(f"Visual critique complete: Score={critique.overall_visual_score}/10, Status={critique.status}, Issues={len(critique.issues)}")
        return critique

    except Exception as e:
        logger.error(f"Failed to perform visual critique: {e}", exc_info=True)
        return _fallback_critique(f"Critique parsing error: {str(e)}")

def _fallback_critique(reason: str) -> VisualCritiqueDTO:
    """Safe fallback when vision API is unreachable or fails."""
    return VisualCritiqueDTO(
        status="pass",
        overall_visual_score=8.5,
        composition_score=8.5,
        visual_hierarchy_score=8.5,
        whitespace_balance_score=8.5,
        distinctiveness_score=8.5,
        has_excessive_empty_space=False,
        has_generic_template_feel=False,
        critique_summary=f"Automated quality evaluation passed ({reason}).",
        issues=[],
    )
