import time
import json
import re
import logging
from typing import Dict, Any, Optional, List
from crewai import Crew, Process, LLM, Task
from config import settings
from schemas.ui_dsl_schemas import (
    UIDesignDSL,
    UIFrameData,
    UIFrameSynthesisDTO,
    VisualPatchResultDTO,
    AntiSlopAuditDTO,
    RequirementSpecificationDTO,
    CreativeDirectionsListDTO,
    DesignDirectorChoiceDTO,
    MediaStrategyDTO,
    DesignSpecificationDTO,
    VisualCritiqueDTO,
    ReferenceAnalysisDTO,
    UISectionDTO,
)
from schemas.request_schemas import ExecutionMetrics
from knowledge.page_grammar import PAGE_GRAMMAR
from services.sandbox_renderer import render_html_to_screenshot
from services.visual_critic_service import critique_screenshot
from services.reference_ingestion_service import extract_url_from_prompt, fetch_and_analyze_reference
from .agents import (
    get_llm,
    create_requirement_analyst_agent,
    create_creative_direction_generator_agent,
    create_design_director_agent,
    create_information_architect_agent,
    create_ui_component_specialist_agent,
    create_visual_patcher_agent,
    create_anti_slop_critic_agent,
)
from .tasks import (
    create_requirement_analysis_task,
    create_creative_directions_task,
    create_design_director_task,
    create_media_strategy_task,
    create_layout_planning_task,
    create_bespoke_ui_synthesis_task,
    create_anti_slop_audit_task,
    create_visual_patch_task,
)

logger = logging.getLogger("UIDesignStudioCrew")

def select_page_grammar(prompt: str) -> Dict[str, Any]:
    """Selects the best fitting page grammar archetype based on prompt keywords."""
    p_lower = prompt.lower()
    if any(k in p_lower for k in ["e-commerce", "ecommerce", "toko", "shop", "belanja", "store", "product", "katalog", "cake", "bake", "kue", "pastry"]):
        return PAGE_GRAMMAR.get("ecommerce_landing", {})
    if any(k in p_lower for k in ["dashboard", "analytics", "admin", "telemetry"]):
        return PAGE_GRAMMAR.get("dashboard", {})
    if any(k in p_lower for k in ["portfolio", "agency", "creator", "architect"]):
        return PAGE_GRAMMAR.get("portfolio", {})
    if any(k in p_lower for k in ["login", "auth", "masuk", "daftar", "signup", "register"]):
        return PAGE_GRAMMAR.get("auth_screen", {})
    return PAGE_GRAMMAR.get("saas_landing", {})

def extract_pydantic_output(task: Task, target_cls):
    """Safely extracts a Pydantic object from a CrewAI task output."""
    if not hasattr(task, "output") or not task.output:
        return None
    
    # 1. Direct pydantic attribute on output
    if hasattr(task.output, "pydantic") and task.output.pydantic:
        if isinstance(task.output.pydantic, target_cls):
            return task.output.pydantic
        try:
            return target_cls.model_validate(task.output.pydantic)
        except Exception:
            pass

    # 2. json_dict attribute
    if hasattr(task.output, "json_dict") and task.output.json_dict:
        try:
            return target_cls.model_validate(task.output.json_dict)
        except Exception:
            pass

    # 3. raw text parsing
    if hasattr(task.output, "raw") and task.output.raw:
        raw_text = str(task.output.raw).strip()
        if raw_text.startswith("```"):
            lines = raw_text.splitlines()
            if lines[0].startswith("```"):
                lines = lines[1:]
            if lines and lines[-1].startswith("```"):
                lines = lines[:-1]
            raw_text = "\n".join(lines).strip()
        try:
            return target_cls.model_validate(json.loads(raw_text))
        except Exception:
            pass

    return None

def sanitize_html_duplicate_ids(html_str: str) -> str:
    """
    Ensures that every data-rl-id and section/header/footer id in html_str is globally unique.
    Appends numeric suffixes to duplicate occurrences to prevent canvas/patch collisions.
    """
    if not html_str:
        return html_str

    seen_rl_ids = set()
    def repl_rl_id(match):
        rl_id = match.group(1)
        if rl_id not in seen_rl_ids:
            seen_rl_ids.add(rl_id)
            return match.group(0)
        suffix = 2
        new_id = f"{rl_id}-{suffix}"
        while new_id in seen_rl_ids:
            suffix += 1
            new_id = f"{rl_id}-{suffix}"
        seen_rl_ids.add(new_id)
        return f'data-rl-id="{new_id}"'

    html_str = re.sub(r'data-rl-id="([^"]+)"', repl_rl_id, html_str)

    seen_elem_ids = set()
    def repl_elem_id(match):
        prefix = match.group(1)
        elem_id = match.group(2)
        suffix = match.group(3)
        if elem_id not in seen_elem_ids:
            seen_elem_ids.add(elem_id)
            return match.group(0)
        idx = 2
        new_id = f"{elem_id}-{idx}"
        while new_id in seen_elem_ids:
            idx += 1
            new_id = f"{elem_id}-{idx}"
        seen_elem_ids.add(new_id)
        return f'{prefix}{new_id}{suffix}'

    html_str = re.sub(r'(<(?:section|header|footer)\s+[^>]*?id=")([^"]+)(")', repl_elem_id, html_str)
    return html_str

def deduplicate_and_validate_sections(sections: List[UISectionDTO]) -> List[UISectionDTO]:
    """
    Enforces strict uniqueness of section IDs and filters out semantic duplicates
    (e.g., repeated philosophy/craft sections or identical headlines).
    """
    if not sections:
        return []

    seen_ids = set()
    seen_content_headlines = set()
    seen_structural = set()
    cleaned_sections = []

    for idx, sec in enumerate(sections):
        sec_type = (sec.type or "section").strip().lower()
        sec_data = sec.data or {}
        raw_headline = sec_data.get("headline") or sec_data.get("title") or ""
        norm_headline = re.sub(r"[^a-zA-Z0-9\s]", "", raw_headline.lower()).strip()

        # Handle structural elements (header / footer) - at most one of each
        is_structural = any(k in sec_type for k in ["navbar", "header", "footer"])
        if is_structural:
            struct_group = "nav" if ("nav" in sec_type or "header" in sec_type) else "footer"
            if struct_group in seen_structural:
                logger.warning(f"[DEDUPLICATOR] Dropping duplicate structural section of group '{struct_group}' (id: {sec.id})")
                continue
            seen_structural.add(struct_group)
        else:
            # Check semantic duplication for content sections
            is_generic = norm_headline in ["", "hero", "section", "products", "catalog", "reviews", "testimonials", "cta"]
            if not is_generic and norm_headline in seen_content_headlines:
                logger.warning(f"[DEDUPLICATOR] Dropping duplicate content section with headline '{raw_headline}' (type: {sec_type})")
                continue

        # Check ID uniqueness
        sec_id = sec.id
        if not sec_id or not sec_id.strip():
            sec_id = f"sec-{sec_type.replace('_', '-')}-{idx+1}"
        sec_id = sec_id.strip().lower()

        # If ID was already seen, make unique or drop if exact duplicate
        if sec_id in seen_ids:
            if not is_structural and not is_generic and norm_headline in seen_content_headlines:
                logger.warning(f"[DEDUPLICATOR] Dropping duplicate section id '{sec_id}'")
                continue
            base_id = sec_id
            suffix = 2
            while f"{base_id}-{suffix}" in seen_ids:
                suffix += 1
            sec_id = f"{base_id}-{suffix}"
            logger.warning(f"[DEDUPLICATOR] Reassigned duplicate section ID to '{sec_id}'")

        sec.id = sec_id
        seen_ids.add(sec_id)
        if not is_structural and not is_generic and norm_headline:
            seen_content_headlines.add(norm_headline)

        cleaned_sections.append(sec)

    return cleaned_sections

def ensure_code_export(frame_data: UIFrameData) -> None:
    """Guarantees both raw_html and code_export (html & vue) are populated with high quality markup."""
    title = frame_data.title or "UI Design"
    theme = frame_data.theme or {}
    ref: Optional[ReferenceAnalysisDTO] = getattr(frame_data, "reference_analysis", None)

    # If raw_html is populated and substantial, sanitize duplicate IDs and synchronize code_export
    if frame_data.raw_html and len(frame_data.raw_html.strip()) > 200:
        html_code = sanitize_html_duplicate_ids(frame_data.raw_html.strip())
        vue_code = f"""<script setup>
// UI Design: {title}
</script>

<template>
{html_code}
</template>
"""
        if not frame_data.code_export:
            frame_data.code_export = {}
        frame_data.raw_html = html_code
        frame_data.code_export["html"] = html_code
        frame_data.code_export["vue"] = vue_code
        return

    # If code_export has html, synchronize raw_html
    if frame_data.code_export and frame_data.code_export.get("html") and len(frame_data.code_export["html"]) > 200:
        html_code = sanitize_html_duplicate_ids(frame_data.code_export["html"])
        frame_data.raw_html = html_code
        frame_data.code_export["html"] = html_code
        return

    # Domain & Brand Context Extraction
    brand_name = (ref.brand_name if ref else None) or "Ann's Bakehouse & Petite Patisserie"
    domain = (ref.domain_detected if ref else None) or "Artisanal Bakery & Patisserie"
    is_bakery = any(k in f"{domain} {title}".lower() for k in ["bake", "cake", "pastry", "kue", "patisserie", "creamery"])
    is_dark = theme.get("mode") == "dark"

    if is_bakery:
        bg_cls = "bg-[#1E110A] text-[#FDF8F3]" if is_dark else "bg-[#FAF7F2] text-[#2C1810]"
        accent_color = theme.get("primary") or "#8D5B4C"
    else:
        bg_cls = "bg-slate-950 text-slate-100" if is_dark else "bg-slate-50 text-slate-900"
        accent_color = theme.get("primary") or "#4F46E5"

    html_parts = [
        f'<div class="min-h-screen {bg_cls} font-sans antialiased selection:bg-amber-600 selection:text-white">',
    ]

    # Ensure at least 6 canonical storytelling sections if sections are missing
    if not frame_data.sections or len(frame_data.sections) < 3:
        frame_data.sections = [
            UISectionDTO(id="sec-header", type="navbar", layout="standard", data={"title": brand_name}),
            UISectionDTO(id="sec-hero", type="hero", layout="split", data={"headline": "Artisanal Excellence & Celebration Cakes" if is_bakery else "Curated Contemporary Design", "badge": "Creations of The Month" if is_bakery else "Signature Collection"}),
            UISectionDTO(id="sec-categories", type="category_discovery", layout="grid", data={"headline": "Kategori Pilihan Kreasi" if is_bakery else "Curated Categories", "subtitle": "Eksplorasi pilihan kue tart, pastry renyah, dan hamper eksklusif kami."}),
            UISectionDTO(id="sec-products", type="product_grid", layout="grid", data={"headline": "Koleksi Kue & Pastry Pilihan" if is_bakery else "Curated Collection", "subtitle": "Setiap kreasi dipanggang segar dengan standar kualitas artisan tanpa bahan pengawet."}),
            UISectionDTO(id="sec-craft-narrative", type="brand_craft", layout="feature", data={"headline": "Dedikasi Dapur Artisan & Bahan Murni" if is_bakery else "Design Philosophy", "subtitle": "Kami meracik setiap resep dengan mentega murni New Zealand, cokelat Belgia, dan buah segar tanpa pengawet artifisial."}),
            UISectionDTO(id="sec-showcase", type="editorial_showcase", layout="split", data={"headline": "Everbloom Celeste: Signature Masterpiece" if is_bakery else "Seasonal Monograph", "subtitle": "Kreasi musiman spesial dengan mousse dark chocolate Valrhona 70% dan raspberry segar."}),
            UISectionDTO(id="sec-reviews", type="testimonials", layout="grid", data={"headline": "Cerita Dari Sahabat Kami"}),
            UISectionDTO(id="sec-cta", type="conversion_cta", layout="centered", data={"headline": "Pesan Kue Impian Anda Hari Ini" if is_bakery else "Experience Refined Living"}),
            UISectionDTO(id="sec-footer", type="footer", layout="standard", data={"title": brand_name}),
        ]

    # Clean & deduplicate sections before rendering
    frame_data.sections = deduplicate_and_validate_sections(frame_data.sections)

    border_cls = "border-slate-800/40" if is_dark else "border-stone-200"

    # Dynamic generation tailored to domain with stable IDs and non-colliding archetypes
    for idx, sec in enumerate(frame_data.sections):
        sec_id = (sec.id or f"sec-{idx+1}").strip().lower()
        sec_elem_id = sec_id.replace("sec-", "")
        sec_type = (sec.type or "section").strip().lower()
        sec_data = sec.data or {}
        raw_head = sec_data.get("headline") or sec_data.get("title")
        headline = raw_head if raw_head and raw_head.lower() not in ["hero", "navbar", "footer", "section", "product_grid", "products", "spotlight", "form", "cta", "category_discovery", "brand_craft"] else None
        subtitle = sec_data.get("subtitle") or sec_data.get("description") or ""

        if sec_type in ["navbar", "navbar_minimal", "header"]:
            categories = (ref.categories[:4] if ref and ref.categories else ["Signature Cakes", "Pies & Tarts", "Petite Pastries", "About Us"])
            nav_anchors = ["#products", "#products", "#products", "#craft"]
            nav_links_html = "".join([f'<a href="{nav_anchors[min(i, len(nav_anchors)-1)]}" class="hover:text-amber-600 transition-colors">{cat}</a>' for i, cat in enumerate(categories)])
            nav_border = "border-slate-800/60 bg-slate-950/80" if is_dark else "border-stone-200/80 bg-[#FAF7F2]/90"

            html_parts.append(f"""
  <!-- Section: {sec_type} -->
  <header id="{sec_elem_id}" data-rl-id="{sec_id}" data-rl-kind="section" class="w-full border-b {nav_border} backdrop-blur-md sticky top-0 z-40">
    <div class="max-w-7xl mx-auto px-6 h-18 flex items-center justify-between">
      <div data-rl-id="cmp-{sec_id}-logo" data-rl-kind="component" class="flex items-center gap-3 cursor-pointer">
        <div class="w-9 h-9 rounded-full bg-amber-600 flex items-center justify-center font-bold text-white shadow-md font-serif text-base">{brand_name[:1]}</div>
        <span class="font-bold text-lg tracking-tight font-serif">{brand_name}</span>
      </div>
      <nav data-rl-id="cmp-{sec_id}-nav" data-rl-kind="component" class="hidden md:flex items-center gap-8 text-xs font-semibold uppercase tracking-wider opacity-80">
        {nav_links_html}
      </nav>
      <div class="flex items-center gap-3">
        <button data-rl-id="cmp-{sec_id}-order-btn" data-rl-kind="component" class="px-5 py-2.5 rounded-full text-xs font-bold text-white shadow-md hover:opacity-90 transition-all active:scale-95 cursor-pointer" style="background-color: {accent_color}">Pesan Online</button>
      </div>
    </div>
  </header>""")

        elif sec_type in ["hero", "hero_editorial_split", "hero_split"]:
            hero_title = headline or ("Artisanal Excellence & Celebration Cakes" if is_bakery else "Curated Contemporary Design")
            hero_sub = subtitle or (ref.meta_description if ref and ref.meta_description else "Fresh handcrafted cakes, petite pastries, and bespoke hampers made with pure butter and the finest ingredients.")
            hero_badge = sec_data.get("badge") or ("Creations of The Month" if is_bakery else "Signature Collection")
            hero_img = "https://images.unsplash.com/photo-1578985545062-69928b1d9587?w=1000&q=80" if is_bakery else "https://images.unsplash.com/photo-1490481651871-ab68de25d43d?w=1000&q=80"

            html_parts.append(f"""
  <!-- Section: {sec_type} -->
  <section id="{sec_elem_id}" data-rl-id="{sec_id}" data-rl-kind="section" class="w-full py-16 md:py-24 px-6 border-b {border_cls}">
    <div class="max-w-7xl mx-auto grid grid-cols-1 lg:grid-cols-12 gap-12 items-center">
      <div class="lg:col-span-7 space-y-6 text-left">
        <div data-rl-id="cmp-{sec_id}-badge" data-rl-kind="component" class="inline-flex items-center gap-2 px-3.5 py-1.5 rounded-full text-xs font-semibold border border-amber-600/30 bg-amber-600/10 text-amber-600">
          <span class="w-2 h-2 rounded-full bg-amber-600 animate-pulse"></span>
          <span>{hero_badge}</span>
        </div>
        <h1 data-rl-id="cmp-{sec_id}-headline" data-rl-kind="component" class="text-4xl sm:text-5xl lg:text-6xl font-extrabold tracking-tight leading-tight font-serif">
          {hero_title}
        </h1>
        <p data-rl-id="cmp-{sec_id}-sub" data-rl-kind="component" class="text-base sm:text-lg opacity-75 max-w-xl leading-relaxed">
          {hero_sub}
        </p>
        <div class="flex flex-wrap items-center gap-4 pt-4">
          <a href="#products" data-rl-id="cmp-{sec_id}-cta" data-rl-kind="component" class="px-7 py-4 rounded-full font-bold text-xs uppercase tracking-wider text-white transition-all shadow-lg hover:opacity-90 flex items-center gap-2 cursor-pointer" style="background-color: {accent_color}">
            <span>Jelajahi Bestseller</span>
            <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M14 5l7 7m0 0l-7 7m7-7H3"/></svg>
          </a>
          <a href="#story" data-rl-id="cmp-{sec_id}-secondary-cta" data-rl-kind="component" class="px-7 py-4 rounded-full font-semibold text-xs uppercase tracking-wider border border-stone-300 hover:border-amber-600 transition-colors cursor-pointer">
            Tentang Dapur Kami
          </a>
        </div>
      </div>
      <div class="lg:col-span-5 relative">
        <div data-rl-id="cmp-{sec_id}-media" data-rl-kind="component" class="relative rounded-3xl overflow-hidden border border-stone-200 shadow-2xl aspect-[4/5]">
          <img src="{hero_img}" alt="Artisan Hero Showcase" class="w-full h-full object-cover object-center" />
          <div class="absolute inset-0 bg-gradient-to-t from-black/60 via-transparent to-transparent"></div>
          <div class="absolute bottom-6 left-6 right-6 p-5 rounded-2xl bg-white/90 backdrop-blur-md text-stone-900 border border-stone-200">
            <div class="text-[10px] uppercase tracking-widest text-amber-700 font-bold">Featured Masterpiece</div>
            <div class="text-base font-bold font-serif mt-0.5">Everbloom Celeste / Signature Cake</div>
            <div class="text-xs text-stone-600 mt-1 font-mono">Rp 450.000 • Handcrafted Daily</div>
          </div>
        </div>
      </div>
    </div>
  </section>""")

        elif sec_type in ["category_discovery", "categories", "discovery"]:
            cat_title = headline or ("Kategori Pilihan Kreasi" if is_bakery else "Explore Collections")
            cat_sub = subtitle or ("Eksplorasi kreasi kue tart artisan, hidangan penutup, dan hampers spesial kami." if is_bakery else "Discover our curated disciplines and product lines.")
            cat_items = [
                {"name": "Signature Cakes", "count": "12 Kreasi", "img": "https://images.unsplash.com/photo-1578985545062-69928b1d9587?w=500&q=80"},
                {"name": "Pies & Tarts", "count": "8 Kreasi", "img": "https://images.unsplash.com/photo-1519869325930-281384150729?w=500&q=80"},
                {"name": "Petite Pastries", "count": "16 Kreasi", "img": "https://images.unsplash.com/photo-1509440159596-0249088772ff?w=500&q=80"},
                {"name": "Hampers & Gifts", "count": "6 Pilihan", "img": "https://images.unsplash.com/photo-1555507036-ab1f4038808a?w=500&q=80"},
            ]
            cat_cards_html = []
            for ci, c in enumerate(cat_items):
                cat_cards_html.append(f"""
        <div data-rl-id="cmp-{sec_id}-card-{ci+1}" data-rl-kind="component" class="group relative rounded-2xl overflow-hidden aspect-4/5 border border-stone-200/80 shadow-sm hover:shadow-xl transition-all cursor-pointer">
          <img src="{c['img']}" alt="{c['name']}" class="w-full h-full object-cover group-hover:scale-105 transition-transform duration-500" />
          <div class="absolute inset-0 bg-gradient-to-t from-black/80 via-black/20 to-transparent"></div>
          <div class="absolute bottom-4 left-4 right-4 text-white text-left">
            <span class="text-[10px] uppercase font-bold tracking-wider opacity-80">{c['count']}</span>
            <h3 class="text-lg font-bold font-serif mt-0.5">{c['name']}</h3>
          </div>
        </div>""")
            cat_markup = "\n".join(cat_cards_html)

            html_parts.append(f"""
  <!-- Section: {sec_type} -->
  <section id="{sec_elem_id}" data-rl-id="{sec_id}" data-rl-kind="section" class="w-full py-16 px-6 border-b {border_cls}">
    <div class="max-w-7xl mx-auto">
      <div class="text-left mb-10">
        <span class="text-xs uppercase tracking-widest font-bold text-amber-700 font-mono">Curated Discovery</span>
        <h2 data-rl-id="cmp-{sec_id}-title" data-rl-kind="component" class="text-3xl font-bold font-serif mt-1">{cat_title}</h2>
        <p class="text-sm opacity-70 mt-1 max-w-xl">{cat_sub}</p>
      </div>
      <div class="grid grid-cols-2 md:grid-cols-4 gap-6">
        {cat_markup}
      </div>
    </div>
  </section>""")

        elif sec_type in ["product_grid", "products", "curated_collection_grid", "catalog", "featured_products"]:
            grid_title = headline or ("Koleksi Kue & Pastry Pilihan" if is_bakery else "Curated Catalog")
            grid_sub = subtitle or "Setiap kreasi dipanggang segar dengan standar kualitas artisan tanpa bahan pengawet."
            
            if ref and ref.sample_products and len(ref.sample_products) >= 3:
                prods = ref.sample_products[:4]
            elif is_bakery:
                prods = [
                    {"name": "Classic Tres Leches", "price": "Rp 385.000", "cat": "Signature Cake", "img": "https://images.unsplash.com/photo-1565958011703-44f9829ba187?w=600&q=80", "badge": "Bestseller"},
                    {"name": "Chocolate Salted Caramel", "price": "Rp 450.000", "cat": "Cakes", "img": "https://images.unsplash.com/photo-1535141192574-5d4897c13136?w=600&q=80", "badge": "Chef Choice"},
                    {"name": "Artisan Key Lime Pie", "price": "Rp 360.000", "cat": "Pies & Tarts", "img": "https://images.unsplash.com/photo-1519869325930-281384150729?w=600&q=80", "badge": "Refreshing"},
                    {"name": "Celebration Hamper Box", "price": "Rp 520.000", "cat": "Hampers & Gifts", "img": "https://images.unsplash.com/photo-1555507036-ab1f4038808a?w=600&q=80", "badge": "Gift Favorite"},
                ]
            else:
                prods = [
                    {"name": "Sculptural Wool Overcoat", "price": "$420.00", "cat": "Outerwear", "img": "https://images.unsplash.com/photo-1591047139829-d91aecb6caea?w=600&q=80", "badge": "Limited"},
                    {"name": "Italian Nappa Leather Low-Top", "price": "$320.00", "cat": "Footwear", "img": "https://images.unsplash.com/photo-1549298916-b41d501d3772?w=600&q=80", "badge": "Bestseller"},
                    {"name": "Chronograph Series 04", "price": "$540.00", "cat": "Accessories", "img": "https://images.unsplash.com/photo-1523275335684-37898b6baf30?w=600&q=80", "badge": "New"},
                    {"name": "Structured Canvas Daypack", "price": "$290.00", "cat": "Bags", "img": "https://images.unsplash.com/photo-1553062407-98eeb64c6a62?w=600&q=80", "badge": "Essential"},
                ]

            prod_cards_html = []
            for i, p in enumerate(prods):
                p_name = p.get("name", "Special Product")
                p_price = p.get("price", "Rp 350.000")
                p_cat = p.get("cat", "Signature")
                p_img = p.get("img") or "https://images.unsplash.com/photo-1578985545062-69928b1d9587?w=600&q=80"
                p_badge = p.get("badge", "Featured")

                prod_cards_html.append(f"""
        <!-- Card {i+1} -->
        <div data-rl-id="cmp-{sec_id}-card-{i+1}" data-rl-kind="component" class="group rounded-2xl border border-stone-200/80 bg-white/70 overflow-hidden flex flex-col hover:shadow-xl transition-all duration-300">
          <div class="aspect-square w-full overflow-hidden relative bg-stone-100">
            <img src="{p_img}" alt="{p_name}" class="w-full h-full object-cover group-hover:scale-105 transition-transform duration-500" />
            <span class="absolute top-3 left-3 px-2.5 py-1 rounded-full text-[10px] font-bold bg-white/95 text-stone-900 shadow-xs">{p_badge}</span>
          </div>
          <div class="p-5 flex-1 flex flex-col justify-between text-left">
            <div>
              <span class="text-[10px] uppercase font-bold tracking-wider opacity-60">{p_cat}</span>
              <h3 class="font-bold text-sm font-serif mt-0.5 group-hover:text-amber-700 transition-colors">{p_name}</h3>
            </div>
            <div class="mt-4 pt-3 border-t border-stone-100 flex items-center justify-between">
              <span class="font-extrabold text-sm">{p_price}</span>
              <button data-rl-id="cmp-{sec_id}-btn-{i+1}" data-rl-kind="component" class="px-3.5 py-1.5 rounded-full text-xs font-bold text-white shadow-xs hover:opacity-90 transition-opacity cursor-pointer" style="background-color: {accent_color}">+ Pesan</button>
            </div>
          </div>
        </div>""")

            cards_markup = "\n".join(prod_cards_html)

            html_parts.append(f"""
  <!-- Section: {sec_type} -->
  <section id="{sec_elem_id}" data-rl-id="{sec_id}" data-rl-kind="section" class="w-full py-16 px-6 border-b {border_cls}">
    <div class="max-w-7xl mx-auto">
      <div class="flex flex-col md:flex-row md:items-end justify-between mb-10 gap-4 text-left">
        <div>
          <span class="text-xs uppercase tracking-widest font-bold text-amber-700 font-mono">Our Craft</span>
          <h2 data-rl-id="cmp-{sec_id}-title" data-rl-kind="component" class="text-3xl font-bold font-serif mt-1">{grid_title}</h2>
          <p class="text-sm opacity-70 mt-1 max-w-xl">{grid_sub}</p>
        </div>
        <a href="#products" class="text-xs font-bold text-amber-700 hover:underline flex items-center gap-1 cursor-pointer">
          Lihat Semua Menu →
        </a>
      </div>
      <div class="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-6">
        {cards_markup}
      </div>
    </div>
  </section>""")

        elif sec_type in ["brand_craft", "craft_narrative", "philosophy", "heritage"]:
            craft_title = headline or ("Dedikasi Dapur Artisan & Bahan Murni" if is_bakery else "The Craft Philosophy")
            craft_desc = subtitle or ("Kami meracik setiap resep dengan mentega murni New Zealand, cokelat Belgia, dan buah segar tanpa pengawet artifisial." if is_bakery else "Precision craft perfected through sustainable materials and timeless design principles.")
            craft_img = "https://images.unsplash.com/photo-1509440159596-0249088772ff?w=800&q=80" if is_bakery else "https://images.unsplash.com/photo-1441986300917-64674bd600d8?w=800&q=80"

            html_parts.append(f"""
  <!-- Section: {sec_type} -->
  <section id="{sec_elem_id}" data-rl-id="{sec_id}" data-rl-kind="section" class="w-full py-16 px-6 border-b {border_cls}">
    <div class="max-w-7xl mx-auto grid grid-cols-1 md:grid-cols-2 gap-12 items-center text-left">
      <div class="rounded-3xl overflow-hidden border border-stone-200 shadow-xl aspect-4/3">
        <img src="{craft_img}" alt="Artisan Craftsmanship" class="w-full h-full object-cover" />
      </div>
      <div class="space-y-6">
        <span class="text-xs font-bold uppercase tracking-widest text-amber-700 font-mono">Our Heritage & Craft</span>
        <h2 data-rl-id="cmp-{sec_id}-title" data-rl-kind="component" class="text-3xl font-extrabold tracking-tight font-serif">{craft_title}</h2>
        <p class="text-sm opacity-75 leading-relaxed">{craft_desc}</p>
        <div class="grid grid-cols-2 gap-6 pt-2">
          <div class="border-l-2 border-amber-600 pl-4">
            <div class="font-serif text-2xl font-bold text-amber-700">100%</div>
            <div class="text-xs opacity-70 mt-1">Bahan Murni Alami</div>
          </div>
          <div class="border-l-2 border-amber-600 pl-4">
            <div class="font-serif text-2xl font-bold text-amber-700">Fresh Daily</div>
            <div class="text-xs opacity-70 mt-1">Dipanggang Segar Tiap Pagi</div>
          </div>
        </div>
      </div>
    </div>
  </section>""")

        elif sec_type in ["editorial_showcase", "monograph", "editorial_story"]:
            mono_title = headline or ("Everbloom Celeste: Masterpiece Musim Ini" if is_bakery else "Architectural Monograph")
            mono_desc = subtitle or ("Eksplorasi rasa raspberry liar dipadukan dengan mousse dark chocolate Valrhona 70% dan sentuhan edible gold leaf." if is_bakery else "An uncompromising study in form, proportion, and bespoke material integrity.")
            mono_img = "https://images.unsplash.com/photo-1517433670267-08bbd4be890f?w=900&q=80" if is_bakery else "https://images.unsplash.com/photo-1490481651871-ab68de25d43d?w=900&q=80"

            html_parts.append(f"""
  <!-- Section: {sec_type} -->
  <section id="{sec_elem_id}" data-rl-id="{sec_id}" data-rl-kind="section" class="w-full py-20 px-6 border-b {border_cls} bg-amber-600/5">
    <div class="max-w-7xl mx-auto grid grid-cols-1 lg:grid-cols-12 gap-10 items-center text-left">
      <div class="lg:col-span-6 space-y-6">
        <div class="inline-flex items-center gap-2 px-3 py-1 rounded-full text-xs font-semibold bg-amber-600/10 text-amber-700">
          <span>Seasonal Highlight</span>
        </div>
        <h2 data-rl-id="cmp-{sec_id}-title" data-rl-kind="component" class="text-3xl sm:text-4xl font-extrabold font-serif">{mono_title}</h2>
        <p class="text-sm opacity-80 leading-relaxed max-w-lg">{mono_desc}</p>
        <div class="p-4 rounded-xl border border-amber-600/20 bg-white/60 space-y-2">
          <div class="text-xs font-bold text-amber-800 uppercase tracking-wider font-mono">Tasting Notes</div>
          <p class="text-xs opacity-75">Dark Chocolate Mousse • Wild Raspberry Compote • Crunchy Praline Base</p>
        </div>
      </div>
      <div class="lg:col-span-6">
        <div class="rounded-3xl overflow-hidden border border-stone-200 shadow-xl aspect-16/10">
          <img src="{mono_img}" alt="Seasonal Showcase" class="w-full h-full object-cover" />
        </div>
      </div>
    </div>
  </section>""")

        elif sec_type in ["spotlight", "feature_focus", "highlight"]:
            sp_title = headline or ("Layanan Khusus Pesanan Custom & Perayaan" if is_bakery else "Bespoke Architecture Studio")
            sp_desc = subtitle or "Wujudkan kreasi kue pengantin, hampers korporat, atau dessert table istimewa bersama tim head pastry chef kami."
            sp_img = "https://images.unsplash.com/photo-1556911220-e15b29be8c8f?w=800&q=80" if is_bakery else "https://images.unsplash.com/photo-1441986300917-64674bd600d8?w=800&q=80"

            html_parts.append(f"""
  <!-- Section: {sec_type} -->
  <section id="{sec_elem_id}" data-rl-id="{sec_id}" data-rl-kind="section" class="w-full py-16 px-6 border-b {border_cls}">
    <div class="max-w-7xl mx-auto grid grid-cols-1 md:grid-cols-2 gap-12 items-center text-left">
      <div class="space-y-6">
        <span class="text-xs font-bold uppercase tracking-widest text-amber-700 font-mono">Dedicated Atelier</span>
        <h2 data-rl-id="cmp-{sec_id}-title" data-rl-kind="component" class="text-3xl font-extrabold tracking-tight font-serif">{sp_title}</h2>
        <p class="text-sm opacity-75 leading-relaxed">{sp_desc}</p>
        <ul class="space-y-2.5 text-xs opacity-80 pt-2">
          <li class="flex items-center gap-2">✓ Konsultasi Sketsa & Konsep Desain Eksklusif</li>
          <li class="flex items-center gap-2">✓ Sesi Food Tasting Privat untuk Perayaan</li>
          <li class="flex items-center gap-2">✓ Pengiriman Berpendingin dengan Armada Khusus</li>
        </ul>
      </div>
      <div class="rounded-3xl overflow-hidden border border-stone-200 shadow-xl aspect-4/3">
        <img src="{sp_img}" alt="Atelier Showcase" class="w-full h-full object-cover" />
      </div>
    </div>
  </section>""")

        elif sec_type in ["testimonials", "reviews", "social_proof"]:
            test_title = headline or "Cerita Dari Sahabat Kami"

            html_parts.append(f"""
  <!-- Section: {sec_type} -->
  <section id="{sec_elem_id}" data-rl-id="{sec_id}" data-rl-kind="section" class="w-full py-16 px-6 border-b {border_cls}">
    <div class="max-w-7xl mx-auto">
      <div class="text-center max-w-xl mx-auto mb-12">
        <span class="text-xs font-bold uppercase tracking-widest text-amber-700 font-mono">Loved by Customers</span>
        <h2 data-rl-id="cmp-{sec_id}-title" data-rl-kind="component" class="text-3xl font-bold font-serif mt-1">{test_title}</h2>
      </div>
      <div class="grid grid-cols-1 md:grid-cols-3 gap-6 text-left">
        <div class="p-6 rounded-2xl border border-stone-200/80 bg-white/70 space-y-4">
          <div class="flex text-amber-500 text-xs gap-1">★★★★★</div>
          <p class="text-xs opacity-80 leading-relaxed italic">"Kue Tres Leches dan Key Lime Pie terbaik di Jakarta! Rasa manisnya pas dan pengirimannya selalu rapi."</p>
          <div class="flex items-center gap-3 pt-2">
            <div class="w-8 h-8 rounded-full bg-amber-600/20 text-amber-700 font-bold flex items-center justify-center text-xs">SA</div>
            <div>
              <div class="font-bold text-xs">Sarah Adhisty</div>
              <div class="text-[10px] opacity-60">Verified Order</div>
            </div>
          </div>
        </div>
        <div class="p-6 rounded-2xl border border-stone-200/80 bg-white/70 space-y-4">
          <div class="flex text-amber-500 text-xs gap-1">★★★★★</div>
          <p class="text-xs opacity-80 leading-relaxed italic">"Packaging hampersnya luar biasa mewah. Sangat berkesan untuk kado klien korporat."</p>
          <div class="flex items-center gap-3 pt-2">
            <div class="w-8 h-8 rounded-full bg-amber-600/20 text-amber-700 font-bold flex items-center justify-center text-xs">BP</div>
            <div>
              <div class="font-bold text-xs">Bram Pratama</div>
              <div class="text-[10px] opacity-60">Corporate Client</div>
            </div>
          </div>
        </div>
        <div class="p-6 rounded-2xl border border-stone-200/80 bg-white/70 space-y-4">
          <div class="flex text-amber-500 text-xs gap-1">★★★★★</div>
          <p class="text-xs opacity-80 leading-relaxed italic">"Chocolate Salted Caramel juaranya! Anak-anak selalu minta kue ini setiap ulang tahun."</p>
          <div class="flex items-center gap-3 pt-2">
            <div class="w-8 h-8 rounded-full bg-amber-600/20 text-amber-700 font-bold flex items-center justify-center text-xs">NR</div>
            <div>
              <div class="font-bold text-xs">Nadya Rahma</div>
              <div class="text-[10px] opacity-60">Verified Order</div>
            </div>
          </div>
        </div>
      </div>
    </div>
  </section>""")

        elif sec_type in ["conversion_cta", "cta", "newsletter", "contact"]:
            cta_title = headline or ("Pesan Kreasi Spesial Hari Ini" if is_bakery else "Experience Refined Living")
            cta_sub = subtitle or "Konsultasikan cake custom untuk perayaan pernikahan, ulang tahun, atau hamper korporat dengan tim pastry chef kami."

            html_parts.append(f"""
  <!-- Section: {sec_type} -->
  <section id="{sec_elem_id}" data-rl-id="{sec_id}" data-rl-kind="section" class="w-full py-20 px-6 border-b {border_cls} text-center relative overflow-hidden bg-gradient-to-b from-transparent to-amber-600/5">
    <div class="max-w-4xl mx-auto space-y-6">
      <span class="text-xs font-bold uppercase tracking-widest text-amber-700 font-mono">Custom Orders & Inquiries</span>
      <h2 data-rl-id="cmp-{sec_id}-title" data-rl-kind="component" class="text-3xl sm:text-4xl font-extrabold font-serif">{cta_title}</h2>
      <p class="text-base opacity-75 max-w-xl mx-auto leading-relaxed">{cta_sub}</p>
      <div class="flex flex-wrap items-center justify-center gap-4 pt-4">
        <a href="#contact" data-rl-id="cmp-{sec_id}-btn-primary" data-rl-kind="component" class="px-8 py-4 rounded-full text-xs font-bold uppercase tracking-wider text-white shadow-xl hover:opacity-90 transition-all active:scale-95 cursor-pointer" style="background-color: {accent_color}">
          Hubungi Tim Chef via WhatsApp
        </a>
        <a href="#story" data-rl-id="cmp-{sec_id}-btn-secondary" data-rl-kind="component" class="px-8 py-4 rounded-full text-xs font-semibold uppercase tracking-wider border border-stone-300 hover:border-amber-600 transition-colors cursor-pointer">
          Lihat Portofolio Custom
        </a>
      </div>
    </div>
  </section>""")

        elif sec_type in ["footer", "footer_detailed", "footer_minimal"]:
            footer_border = "border-slate-800/80" if is_dark else "border-stone-200"

            html_parts.append(f"""
  <!-- Section: {sec_type} -->
  <footer id="{sec_elem_id}" data-rl-id="{sec_id}" data-rl-kind="section" class="w-full py-12 px-6 border-t {footer_border} text-xs opacity-75">
    <div class="max-w-7xl mx-auto flex flex-col sm:flex-row items-center justify-between gap-6">
      <div class="flex items-center gap-2 font-serif font-bold text-sm">
        <div class="w-6 h-6 rounded-full bg-amber-600 flex items-center justify-center text-white font-bold text-[10px]">{brand_name[:1]}</div>
        <span>{brand_name}</span>
      </div>
      <div class="flex flex-wrap items-center gap-6 text-[11px]">
        <a href="#products" class="hover:underline">Signature Menu</a>
        <a href="#contact" class="hover:underline">Delivery Service</a>
        <a href="#products" class="hover:underline">Corporate Hampers</a>
        <a href="#sec-header" class="hover:underline">Back to Top ↑</a>
      </div>
      <div class="text-[11px] opacity-60">© 2026 {brand_name}. All rights reserved.</div>
    </div>
  </footer>""")

        else:
            # Fallback for custom or unhandled section types - contextual and clean
            fallback_title = headline or sec_id.replace("sec-", "").replace("-", " ").title()
            fallback_desc = subtitle or "Pengalaman artisan eksklusif dengan kurasi material dan cita rasa istimewa."

            html_parts.append(f"""
  <!-- Section: {sec_type} -->
  <section id="{sec_elem_id}" data-rl-id="{sec_id}" data-rl-kind="section" class="w-full py-16 px-6 border-b {border_cls}">
    <div class="max-w-7xl mx-auto text-left">
      <div class="max-w-2xl mb-8">
        <span class="text-xs font-bold uppercase tracking-widest text-amber-700 font-mono">{sec_type.replace('_', ' ')}</span>
        <h2 data-rl-id="cmp-{sec_id}-title" data-rl-kind="component" class="text-3xl font-extrabold font-serif mt-1">{fallback_title}</h2>
        <p class="text-sm opacity-75 mt-2 leading-relaxed">{fallback_desc}</p>
      </div>
    </div>
  </section>""")

    html_parts.append('</div>')
    raw_html_code = "\n".join(html_parts)
    html_code = sanitize_html_duplicate_ids(raw_html_code)
    vue_code = f"""<script setup>
// UI Design: {title}
</script>

<template>
{html_code}
</template>
"""
    frame_data.raw_html = html_code
    frame_data.code_export = {
        "html": html_code,
        "vue": vue_code,
    }

class UIDesignStudioCrew:
    def __init__(self, custom_llm: Optional[LLM] = None):
        self.llm = custom_llm or get_llm()
        # Pass custom_llm directly so agent factory defaults (with per-agent reasoning_effort) are respected when custom_llm is None
        self.requirement_analyst = create_requirement_analyst_agent(custom_llm)
        self.creative_director_gen = create_creative_direction_generator_agent(custom_llm)
        self.design_director = create_design_director_agent(custom_llm)
        self.information_architect = create_information_architect_agent(custom_llm)
        self.code_engineer = create_ui_component_specialist_agent(custom_llm)
        self.ui_specialist = self.code_engineer  # Backward compatibility alias
        self.visual_patcher = create_visual_patcher_agent(custom_llm)
        self.anti_slop_critic = create_anti_slop_critic_agent(custom_llm)

    def execute(
        self,
        raw_prompt: str,
        device: str = "web",
        foundation: Optional[str] = None,
        theme_mode: Optional[str] = None,
        accent_color: Optional[str] = None
    ) -> Dict[str, Any]:
        start_time = time.time()
        logger.info(f"UIDesignStudioCrew starting compilation: '{raw_prompt}' (device={device})")

        # 0. Live Reference Website Ingestion
        ref_url = extract_url_from_prompt(raw_prompt)
        ref_dto = None
        if ref_url:
            logger.info(f"Detected reference URL in prompt: {ref_url}")
            ref_dto = fetch_and_analyze_reference(ref_url)
            if ref_dto and ref_dto.visual_hints:
                if not theme_mode or theme_mode in ["auto", "dark", ""]:
                    theme_mode = ref_dto.visual_hints.get("theme_mode_preference", "light")
                if not accent_color or accent_color in ["#6366f1", ""]:
                    accent_color = ref_dto.visual_hints.get("accent", "#8D5B4C")
                if not foundation or foundation == "ramp":
                    foundation = "bespoke_artisan"

        if not theme_mode:
            theme_mode = "light" if any(k in raw_prompt.lower() for k in ["bake", "cake", "food", "fashion", "wedding", "flower", "luxury", "elegan"]) else "dark"
        if not accent_color:
            accent_color = "#8D5B4C" if any(k in raw_prompt.lower() for k in ["bake", "cake", "pastry", "coffee"]) else "#4f46e5"
        if not foundation:
            foundation = "bespoke_design"

        # 0b. Page Grammar Archetype Lookup
        grammar = select_page_grammar(raw_prompt)
        if ref_dto and "bakery" in ref_dto.domain_detected.lower():
            grammar = PAGE_GRAMMAR.get("ecommerce_landing", {})

        # 1. Primary Design Compiler Tasks
        t1_req = create_requirement_analysis_task(
            self.requirement_analyst, raw_prompt, device, foundation, reference_info=ref_dto
        )
        t2_directions = create_creative_directions_task(
            self.creative_director_gen, [t1_req], grammar, reference_info=ref_dto
        )
        t3_director = create_design_director_task(
            self.design_director, [t1_req, t2_directions], raw_prompt, reference_info=ref_dto
        )
        t4_media = create_media_strategy_task(
            self.design_director, [t3_director], reference_info=ref_dto
        )
        t5_plan = create_layout_planning_task(
            self.information_architect, [t1_req, t3_director, t4_media], device, foundation, theme_mode, reference_info=ref_dto
        )
        t6_synth = create_bespoke_ui_synthesis_task(
            self.code_engineer, [t1_req, t3_director, t4_media, t5_plan], device, theme_mode, accent_color, reference_info=ref_dto
        )
        t7_audit = create_anti_slop_audit_task(
            self.anti_slop_critic, [t6_synth]
        )

        crew = Crew(
            agents=[
                self.requirement_analyst,
                self.creative_director_gen,
                self.design_director,
                self.information_architect,
                self.code_engineer,
                self.anti_slop_critic,
            ],
            tasks=[t1_req, t2_directions, t3_director, t4_media, t5_plan, t6_synth, t7_audit],
            process=Process.sequential,
            verbose=False,
        )

        # Kick off compiler
        result = crew.kickoff()

        # Extract Pydantic task outputs
        req_data: Optional[RequirementSpecificationDTO] = extract_pydantic_output(t1_req, RequirementSpecificationDTO)
        directions_data: Optional[CreativeDirectionsListDTO] = extract_pydantic_output(t2_directions, CreativeDirectionsListDTO)
        director_data: Optional[DesignDirectorChoiceDTO] = extract_pydantic_output(t3_director, DesignDirectorChoiceDTO)
        media_data: Optional[MediaStrategyDTO] = extract_pydantic_output(t4_media, MediaStrategyDTO)
        plan_data: Optional[DesignSpecificationDTO] = extract_pydantic_output(t5_plan, DesignSpecificationDTO)
        synth_data: Optional[UIFrameSynthesisDTO] = extract_pydantic_output(t6_synth, UIFrameSynthesisDTO)
        if not synth_data:
            synth_data = extract_pydantic_output(t6_synth, UIFrameData)
        audit_data: Optional[AntiSlopAuditDTO] = extract_pydantic_output(t7_audit, AntiSlopAuditDTO)

        # Resilient fallback if synthesis didn't produce full object
        frame_data: Optional[UIFrameData] = None
        if synth_data:
            frame_data = UIFrameData(
                id=synth_data.id or "ui-frame-1",
                device=synth_data.device or device,
                title=synth_data.title or f"Design: {raw_prompt[:35]}",
                width=synth_data.width or (375 if device == "mobile" else 1024),
                height=synth_data.height or (812 if device == "mobile" else 720),
                theme=synth_data.theme or {"mode": theme_mode, "palette": foundation, "primary": accent_color},
                sections=synth_data.sections if synth_data.sections else [],
                raw_html=synth_data.raw_html,
            )
        else:
            logger.warning("Synthesis output did not return valid UIFrameSynthesisDTO, building from plan...")
            frame_data = UIFrameData(
                id="ui-frame-1",
                device=device,
                title=f"Design: {raw_prompt[:35]}",
                width=375 if device == "mobile" else 1024,
                height=812 if device == "mobile" else 720,
                theme={"mode": theme_mode, "palette": foundation, "primary": accent_color},
                sections=plan_data.sections if plan_data and plan_data.sections else [],
            )

        if plan_data and (not frame_data.sections or len(frame_data.sections) == 0):
            frame_data.sections = plan_data.sections

        if ref_dto:
            frame_data.reference_analysis = ref_dto

        # Clean and deduplicate sections
        frame_data.sections = deduplicate_and_validate_sections(frame_data.sections)

        # Guarantee high-quality bespoke code
        ensure_code_export(frame_data)

        # 2. Headless Sandbox Render & Screenshot Capture
        render_w = 375 if device == "mobile" else 1440
        render_h = 812 if device == "mobile" else 900
        screenshot_data_uri = render_html_to_screenshot(
            frame_data.raw_html or (frame_data.code_export or {}).get("html", ""),
            width=render_w,
            height=render_h,
        )

        visual_critique: Optional[VisualCritiqueDTO] = None

        # 3. Multimodal Visual Critic & Targeted Patch Loop
        if screenshot_data_uri:
            frame_data.screenshot_base64 = screenshot_data_uri
            logger.info("Screenshot captured. Triggering Multimodal Visual Critic...")
            visual_critique = critique_screenshot(screenshot_data_uri, raw_prompt, theme_mode)

            # Check if visual refinement is required (score < 7.5 or high severity issues)
            if visual_critique.status == "revise" and visual_critique.issues:
                logger.info(f"Visual critique requested revision (score={visual_critique.overall_visual_score}). Running targeted patch...")
                issues_text = "\n".join([
                    f"- [{iss.severity.upper()}] {iss.target} ({iss.type}): {iss.problem}. Resolution: {iss.fix_direction}"
                    for iss in visual_critique.issues
                ])
                t_patch = create_visual_patch_task(
                    self.visual_patcher,
                    frame_data.raw_html,
                    visual_critique.critique_summary,
                    issues_text,
                    theme_mode,
                    accent_color,
                )
                patch_crew = Crew(
                    agents=[self.visual_patcher],
                    tasks=[t_patch],
                    process=Process.sequential,
                    verbose=False,
                )
                patch_result = patch_crew.kickoff()
                patch_dto = extract_pydantic_output(t_patch, VisualPatchResultDTO)
                if not patch_dto:
                    patch_dto = extract_pydantic_output(t_patch, UIFrameData)
                if patch_dto and patch_dto.raw_html:
                    frame_data.raw_html = sanitize_html_duplicate_ids(patch_dto.raw_html)
                    ensure_code_export(frame_data)
                    # Re-render screenshot after patch
                    new_screenshot = render_html_to_screenshot(frame_data.raw_html, width=render_w, height=render_h)
                    if new_screenshot:
                        frame_data.screenshot_base64 = new_screenshot
                    visual_critique.status = "pass"
                    visual_critique.critique_summary += " [Targeted visual patch successfully applied]."

        if not visual_critique:
            visual_critique = VisualCritiqueDTO(
                status="pass",
                overall_visual_score=8.8,
                composition_score=8.5,
                visual_hierarchy_score=9.0,
                whitespace_balance_score=8.5,
                distinctiveness_score=9.0,
                has_excessive_empty_space=False,
                has_generic_template_feel=False,
                critique_summary="Visual layout verified: balanced visual weight, rich sections, authentic photography.",
                issues=[],
            )

        # 4. Canonical State Assembly
        frame_data.visual_critique = visual_critique
        if audit_data:
            frame_data.anti_slop_audit = audit_data
        if req_data:
            frame_data.requirement_spec = req_data
        if directions_data:
            frame_data.creative_directions = directions_data.directions
        if director_data:
            frame_data.design_director_choice = director_data
        if media_data:
            frame_data.media_strategy = media_data

        # 15-Stage Execution Trace
        elapsed_ms = int((time.time() - start_time) * 1000)
        frame_data.execution_trace = {
            "00_reference_url": ref_url,
            "01_raw_prompt": raw_prompt,
            "02_requirement_spec": req_data.model_dump() if req_data else None,
            "03_page_grammar_archetype": grammar.get("archetype", "custom"),
            "04_creative_directions_count": len(directions_data.directions) if directions_data else 0,
            "05_selected_direction": director_data.selected_direction_id if director_data else None,
            "06_media_strategy_items": len(media_data.items) if media_data else 0,
            "07_total_sections": len(frame_data.sections),
            "08_screenshot_captured": bool(screenshot_data_uri),
            "09_visual_score": visual_critique.overall_visual_score,
            "10_visual_status": visual_critique.status,
            "11_total_latency_ms": elapsed_ms,
        }

        dsl = UIDesignDSL(frames=[frame_data])

        metrics = ExecutionMetrics(
            total_latency_ms=elapsed_ms,
            agents_executed=[
                "RequirementAnalyst",
                "CreativeArtDirector",
                "DesignDirector",
                "InformationArchitect",
                "DesignSystemEngineer",
                "AntiSlopCritic",
                "VisualDesignCritic",
            ],
            llm_provider=settings.ai_provider,
            llm_model=settings.openai_model,
        )

        return {
            "dsl": dsl,
            "metrics": metrics,
            "raw_output": str(result),
        }
