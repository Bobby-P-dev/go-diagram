"""
Page Grammar Knowledge Library
Provides structural guidance and composition possibilities for page archetypes.
NOT A FIXED TEMPLATE — gives the AI architectural knowledge and creative options.
"""

from typing import Dict, Any, List

PAGE_GRAMMAR: Dict[str, Dict[str, Any]] = {
    "ecommerce_landing": {
        "archetype": "ecommerce_landing",
        "structural_needs": [
            "Introduce offer / brand collection",
            "Enable intuitive product discovery",
            "Establish social proof & trust signals",
            "Create clear, low-friction conversion paths",
            "Highlight key craftsmanship / shipping / guarantee differentiators",
        ],
        "possible_compositions": [
            {
                "name": "Editorial Premium Commerce",
                "recommended_sections": [
                    "navbar_minimal",
                    "hero_editorial_split",
                    "brand_ethos_banner",
                    "curated_collection_grid",
                    "product_spotlight_feature",
                    "social_proof_reviews",
                    "conversion_cta",
                    "footer_detailed",
                ],
                "composition_notes": "Generous vertical whitespace, serif accents, large dominant photography, subtle borders."
            },
            {
                "name": "Contemporary Product-Led Commerce",
                "recommended_sections": [
                    "announcement_bar",
                    "navbar_with_cart",
                    "hero_product_showcase",
                    "benefit_highlights_grid",
                    "featured_products_catalog",
                    "specifications_comparison",
                    "customer_ratings_carousel",
                    "guarantee_and_checkout_banner",
                    "footer_minimal",
                ],
                "composition_notes": "High information density, bold contrast, direct price badges, interactive variant chips."
            },
            {
                "name": "Minimal Curated Showcase",
                "recommended_sections": [
                    "navbar_simple",
                    "hero_typographic_statement",
                    "product_grid_asymmetrical",
                    "customer_story_split",
                    "faq_accordion",
                    "footer_simple",
                ],
                "composition_notes": "Monochrome palette, stark typographic hierarchy, 1px dividers, zero clutter."
            }
        ],
        "suggested_primitives": ["Navbar", "Hero", "Card", "Badge", "PriceTag", "RatingStars", "Button", "ImageGrid", "Accordion", "Footer"]
    },
    "saas_landing": {
        "archetype": "saas_landing",
        "structural_needs": [
            "Clear value proposition & target audience alignment",
            "Visual demonstration of software interface / workflow",
            "Customer logos & quantitative proof metrics",
            "Feature breakdown with concrete outcomes",
            "Pricing tier comparison & self-serve onboarding CTA",
            "Objection handling (Security, FAQ, integrations)",
        ],
        "possible_compositions": [
            {
                "name": "Linear / Terminal Infrastructure Style",
                "recommended_sections": [
                    "navbar_sticky",
                    "hero_with_app_terminal_preview",
                    "trusted_by_logos",
                    "interactive_kpi_grid",
                    "deep_dive_feature_tabs",
                    "security_compliance_badges",
                    "pricing_tiers",
                    "final_cta_banner",
                    "footer_sitemap",
                ],
                "composition_notes": "Deep dark theme, crisp 1px borders, monospace accents, telemetry status pills."
            },
            {
                "name": "Human-Centric Calm Workspace",
                "recommended_sections": [
                    "navbar_floating",
                    "hero_centered_with_video_modal",
                    "social_proof_metrics",
                    "benefit_cards_masonry",
                    "workflow_step_sequence",
                    "customer_quote_card",
                    "pricing_simple_toggle",
                    "faq_section",
                    "footer_clean",
                ],
                "composition_notes": "Soft stone/slate backgrounds, generous padding, warm typography, high contrast CTA."
            }
        ],
        "suggested_primitives": ["Navbar", "Hero", "AppWindowFrame", "KpiCard", "FeatureCard", "PricingCard", "Accordion", "Button", "Badge"]
    },
    "portfolio": {
        "archetype": "portfolio",
        "structural_needs": [
            "Immediate creator identity & specialized positioning",
            "Curated showcase of top works with context and outcomes",
            "Skills / tech stack / capabilities matrix",
            "About narrative & professional philosophy",
            "Direct contact & booking inquiry channel",
        ],
        "possible_compositions": [
            {
                "name": "Brutalist Studio Architecture",
                "recommended_sections": [
                    "minimal_header",
                    "hero_bold_typography",
                    "project_gallery_fullbleed",
                    "selected_client_index",
                    "biography_split",
                    "contact_action_footer",
                ],
                "composition_notes": "Heavy font weights, stark monochrome or single neon accent, architectural line grids."
            }
        ],
        "suggested_primitives": ["Header", "ProjectCard", "TagBadge", "MediaViewer", "ContactForm", "Footer"]
    },
    "auth_screen": {
        "archetype": "auth_screen",
        "structural_needs": [
            "Unambiguous user identification & credential entry",
            "Clear single primary submit action",
            "Secondary account recovery or switch link",
            "Zero distracting visual clutter or marketing noise",
        ],
        "possible_compositions": [
            {
                "name": "Editorial Split Auth",
                "recommended_sections": [
                    "split_visual_showcase",
                    "credential_form_card",
                ],
                "composition_notes": "Left: high-impact brand artwork and quote; Right: perfectly centered, distraction-free form."
            },
            {
                "name": "Minimalist Floating Card",
                "recommended_sections": [
                    "brand_centered_header",
                    "credential_form_card",
                    "security_badges_footer",
                ],
                "composition_notes": "Centered on subtle textured or dark background, high focus states, crisp 1px borders."
            }
        ],
        "suggested_primitives": ["Input", "Button", "Checkbox", "Link", "BrandLogo", "Alert"]
    },
    "dashboard": {
        "archetype": "dashboard",
        "structural_needs": [
            "High-level telemetry / status summary",
            "Actionable KPI cards with trend vectors",
            "Primary operational data table or task queue",
            "Contextual filter & search toolbar",
            "Recent activity or audit trail feed",
        ],
        "possible_compositions": [
            {
                "name": "Operational Control Center",
                "recommended_sections": [
                    "dashboard_header_toolbar",
                    "kpi_metrics_grid",
                    "data_filter_bar",
                    "primary_data_table",
                    "recent_activity_feed",
                ],
                "composition_notes": "Compact density, tabular numbers, status pills (green/amber/rose), sticky table headers."
            }
        ],
        "suggested_primitives": ["TopNav", "KpiCard", "FilterBar", "DataTable", "StatusBadge", "Pagination", "FeedList"]
    }
}

def get_page_grammar(page_type: str) -> Dict[str, Any]:
    """Retrieves structural grammar guidelines for a page archetype."""
    normalized = page_type.lower().replace("-", "_").replace(" ", "_")
    for key, grammar in PAGE_GRAMMAR.items():
        if key in normalized or normalized in key:
            return grammar
    # Generic landing fallback
    return PAGE_GRAMMAR["saas_landing"]
