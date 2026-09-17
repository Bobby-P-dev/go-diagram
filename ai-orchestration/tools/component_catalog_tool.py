"""
Component Primitives & Design System Catalog
Provides fundamental building blocks (primitives) and composition guidelines
allowing AI to assemble unique, expressive page compositions instead of rigid templates.
"""

from typing import Dict, Any, List

PRIMITIVE_CATALOG: Dict[str, Dict[str, Any]] = {
    "Button": {
        "variants": ["primary", "secondary", "outline", "ghost", "link", "pill"],
        "sizes": ["xs", "sm", "md", "lg"],
        "tailwind_tokens": "inline-flex items-center justify-center font-medium transition-all duration-200 active:scale-95 disabled:opacity-50",
        "description": "Interactive action trigger supporting icons, loading states, and variant styling."
    },
    "Input": {
        "variants": ["default", "filled", "underline", "search"],
        "tailwind_tokens": "w-full rounded-xl border px-3.5 py-2.5 text-sm transition-colors focus:outline-none focus:ring-2",
        "description": "Single-line or multi-line text input with label and helper states."
    },
    "Badge": {
        "variants": ["neutral", "accent", "success", "warning", "outline"],
        "tailwind_tokens": "inline-flex items-center gap-1.5 px-2.5 py-0.5 rounded-full text-[11px] font-semibold tracking-wide border",
        "description": "Compact metadata indicator for status, categories, or highlights."
    },
    "CardPrimitive": {
        "variants": ["bordered", "subtle_fill", "elevated", "interactive_hover"],
        "tailwind_tokens": "rounded-2xl border p-6 transition-all duration-300 backdrop-blur-xs",
        "description": "Foundational container primitive for grouping related content, products, or metrics."
    },
    "ImageContainer": {
        "ratios": ["16:9", "4:5", "1:1", "3:4", "21:9"],
        "tailwind_tokens": "relative overflow-hidden rounded-2xl bg-slate-800/40 object-cover",
        "description": "Visual media frame with aspect-ratio constraint, subtle placeholder, and responsive loading."
    },
    "Grid": {
        "layouts": ["cols_2", "cols_3", "cols_4", "asymmetric_split_1_2", "asymmetric_split_2_1", "masonry"],
        "tailwind_tokens": "grid gap-6 md:gap-8",
        "description": "CSS Grid layout primitive for balanced or asymmetrical multi-column compositions."
    },
    "Stack": {
        "directions": ["vertical", "horizontal", "wrap"],
        "gaps": ["gap-2", "gap-4", "gap-6", "gap-8", "gap-12"],
        "tailwind_tokens": "flex",
        "description": "Flexbox flow primitive for linear or wrapping content arrangements."
    },
    "Heading": {
        "levels": ["display", "h1", "h2", "h3", "h4"],
        "weights": ["font-black", "font-extrabold", "font-bold", "font-semibold"],
        "tailwind_tokens": "tracking-tight text-slate-900 dark:text-white",
        "description": "Typographic heading primitive with calibrated scale and tight letter-spacing."
    },
    "Text": {
        "variants": ["lead", "body", "caption", "mono"],
        "tailwind_tokens": "text-slate-600 dark:text-slate-400",
        "description": "Body and supporting copy with comfortable line-height for readability."
    },
    "Divider": {
        "variants": ["horizontal", "vertical", "with_label"],
        "tailwind_tokens": "border-slate-200 dark:border-slate-800/80",
        "description": "Subtle 1px structural separator."
    }
}

SEMANTIC_SECTIONS: Dict[str, str] = {
    "navbar": "Top navigation header with brand mark, link hierarchy, and action cluster",
    "hero": "Visual introduction with dominant typography, value narrative, media placement, and primary CTA",
    "feature_grid": "Composition of capability cards, icon features, or interactive value props",
    "product_grid": "Catalog display with authentic imagery, price tags, ratings, and add-to-cart affordances",
    "pricing": "Tiered subscription or licensing plan matrix with feature breakdowns",
    "social_proof": "Customer testimonials, verified ratings, press quotes, or client logo clouds",
    "data_table": "Dense operational table with sorting, status pills, and pagination",
    "kpi_grid": "Metric cards summarizing real-time metrics with trend vectors",
    "form": "Structured data collection or authentication interface",
    "faq": "Accordion or multi-column objection handling questions and answers",
    "conversion_cta": "High-contrast closing statement designed to drive immediate conversion",
    "footer": "Site navigation index, legal links, language toggle, and copyright"
}

def get_primitive_catalog() -> Dict[str, Dict[str, Any]]:
    """Returns primitive components catalog."""
    return PRIMITIVE_CATALOG

def get_component_info(component_type: str) -> Dict[str, Any]:
    """Returns information for a primitive or section."""
    if component_type in PRIMITIVE_CATALOG:
        return PRIMITIVE_CATALOG[component_type]
    if component_type in SEMANTIC_SECTIONS:
        return {"description": SEMANTIC_SECTIONS[component_type], "type": "section"}
    return {"description": "Custom UI primitive", "type": "primitive"}

def list_all_components() -> List[str]:
    """Returns all component names (primitives + semantic sections)."""
    return list(PRIMITIVE_CATALOG.keys()) + list(SEMANTIC_SECTIONS.keys())

# Backward compatibility alias
COMPONENT_CATALOG = PRIMITIVE_CATALOG

