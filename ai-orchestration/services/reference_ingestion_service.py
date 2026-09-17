import re
import html
import logging
from typing import Optional, Dict, Any, List
from html.parser import HTMLParser
import httpx
from schemas.ui_dsl_schemas import ReferenceAnalysisDTO

logger = logging.getLogger("ReferenceIngestionService")

URL_REGEX = re.compile(r"https?://[a-zA-Z0-9\-._~:/?#[\]@!$&'()*+,;=%]+")

def extract_url_from_prompt(prompt: str) -> Optional[str]:
    """Extracts the first HTTP/HTTPS URL found in the prompt string."""
    if not prompt:
        return None
    match = URL_REGEX.search(prompt)
    if not match:
        return None
    url = match.group(0)
    # Strip trailing punctuation that might be part of natural prose
    url = url.rstrip(".,;!?'\")>]} ")
    return url

class SimpleWebMetadataParser(HTMLParser):
    """Robust standard-library HTML parser to extract metadata, headings, categories, and products."""
    def __init__(self):
        super().__init__()
        self.in_title = False
        self.title = ""
        self.meta_desc = ""
        self.og_site_name = ""
        self.current_heading = None
        self.headings: List[str] = []
        self.current_anchor = False
        self.current_anchor_text = []
        self.nav_links: List[str] = []
        self.all_text_chunks: List[str] = []

    def handle_starttag(self, tag, attrs):
        attr_dict = {k.lower(): (v or "") for k, v in attrs}
        if tag == "title":
            self.in_title = True
        elif tag == "meta":
            prop = attr_dict.get("property", "").lower()
            name = attr_dict.get("name", "").lower()
            content = attr_dict.get("content", "")
            if name == "description" or prop == "og:description":
                if content and not self.meta_desc:
                    self.meta_desc = content
            elif prop == "og:site_name":
                if content and not self.og_site_name:
                    self.og_site_name = content
        elif tag in ["h1", "h2", "h3"]:
            self.current_heading = []
        elif tag == "a":
            self.current_anchor = True
            self.current_anchor_text = []

    def handle_endtag(self, tag):
        if tag == "title":
            self.in_title = False
        elif tag in ["h1", "h2", "h3"]:
            if self.current_heading is not None:
                h_text = " ".join(self.current_heading).strip()
                if 2 < len(h_text) < 120 and h_text not in self.headings:
                    self.headings.append(h_text)
                self.current_heading = None
        elif tag == "a":
            if self.current_anchor:
                a_text = " ".join(self.current_anchor_text).strip()
                if 2 < len(a_text) < 40 and a_text not in self.nav_links:
                    self.nav_links.append(a_text)
                self.current_anchor = False
                self.current_anchor_text = []

    def handle_data(self, data):
        cleaned = data.strip()
        if not cleaned:
            return
        self.all_text_chunks.append(cleaned)
        if self.in_title:
            self.title += " " + cleaned
        if self.current_heading is not None:
            self.current_heading.append(cleaned)
        if self.current_anchor:
            self.current_anchor_text.append(cleaned)

def analyze_html_content(url: str, html_text: str) -> ReferenceAnalysisDTO:
    """Parses raw HTML and extracts brand identity, structural IA, product catalog, and visual hints."""
    parser = SimpleWebMetadataParser()
    try:
        parser.feed(html_text)
    except Exception as e:
        logger.warning(f"HTML parsing note: {e}")

    raw_title = html.unescape(parser.title.strip())
    brand_name = parser.og_site_name.strip()
    if not brand_name and raw_title:
        parts = re.split(r"[\-|•|:]", raw_title)
        if len(parts) > 1:
            brand_name = parts[-1].strip()
        else:
            brand_name = parts[0].strip()

    if not brand_name:
        brand_name = url.split("//")[-1].split("/")[0].replace("www.", "")

    meta_desc = html.unescape(parser.meta_desc.strip())
    headings = parser.headings[:10]

    # Filter categories from nav links
    categories = []
    seen_cats = set()
    for lk in parser.nav_links:
        lk_clean = lk.strip()
        lk_lower = lk_clean.lower()
        if 2 < len(lk_clean) < 30 and lk_lower not in seen_cats:
            if not any(skip in lk_lower for skip in ["login", "sign in", "cart", "keranjang", "privacy", "terms", "policy", "faq", "home", "beranda"]):
                seen_cats.add(lk_lower)
                categories.append(lk_clean)
                if len(categories) >= 8:
                    break

    # Search for products and prices in text
    sample_products: List[Dict[str, Any]] = []
    price_regex = re.compile(r"(?:Rp\.?|IDR|\$|€|£)\s*[0-9]{1,3}(?:[.,][0-9]{3})*", re.I)
    
    seen_prods = set()
    for i, chunk in enumerate(parser.all_text_chunks):
        if price_regex.search(chunk):
            # Look at previous chunks for product title
            for lookback in range(1, 4):
                if i - lookback >= 0:
                    cand = parser.all_text_chunks[i - lookback]
                    if 3 < len(cand) < 60 and not price_regex.search(cand) and cand.lower() not in seen_prods:
                        if not any(k in cand.lower() for k in ["total", "subtotal", "ongkir", "shipping", "tax"]):
                            seen_prods.add(cand.lower())
                            sample_products.append({
                                "name": cand,
                                "price": chunk.strip()
                            })
                            break
        if len(sample_products) >= 6:
            break

    # Domain Heuristics
    combined_text = f"{raw_title} {meta_desc} {' '.join(headings)}".lower()
    domain_detected = "Bespoke Web Platform"
    if any(k in combined_text for k in ["cake", "bake", "patisserie", "pastry", "pie", "roti", "kue", "creamery"]):
        domain_detected = "Artisanal Bakery & Patisserie"
    elif any(k in combined_text for k in ["fashion", "apparel", "clothing", "lookbook", "couture", "wear"]):
        domain_detected = "Fashion & Apparel Brand"
    elif any(k in combined_text for k in ["saas", "software", "api", "cloud", "platform", "workflow"]):
        domain_detected = "Modern SaaS Platform"
    elif any(k in combined_text for k in ["store", "shop", "belanja", "ecommerce", "catalog"]):
        domain_detected = "Curated E-Commerce"
    elif any(k in combined_text for k in ["architect", "interior", "studio", "design agency"]):
        domain_detected = "Architectural & Creative Studio"

    visual_hints: Dict[str, Any] = {
        "vibe": "Refined, authentic visual language",
        "recommended_colors": ["#1e293b", "#f8fafc", "#4f46e5"],
        "theme_mode_preference": "light",
        "accent": "#4f46e5"
    }

    if "bakery" in domain_detected.lower() or "patisserie" in domain_detected.lower():
        visual_hints = {
            "vibe": "Warm artisanal luxury, appetizing gourmet elegance, generous ivory spacing",
            "typography_hint": "Warm sophisticated serif headings (e.g. Playfair, Cormorant) with clean geometric sans body",
            "recommended_colors": ["#FAF8F5", "#2C1810", "#8D5B4C", "#D4AF37", "#FFFFFF"],
            "theme_mode_preference": "light",
            "accent": "#8D5B4C",
            "media_theme": "Close-up artisan cakes, golden pastries, dusted cocoa, warm natural lighting",
        }
    elif "fashion" in domain_detected.lower():
        visual_hints = {
            "vibe": "Editorial haute-couture, monochrome contrast, spacious layout, minimalist borders",
            "typography_hint": "High-fashion serif or ultra-clean grotesque sans",
            "recommended_colors": ["#000000", "#FFFFFF", "#71717A", "#18181B"],
            "theme_mode_preference": "light",
            "accent": "#000000",
            "media_theme": "Editorial portraits, textured fabrics, studio lighting",
        }

    summary = (
        f"Brand '{brand_name}' ({domain_detected}). "
        f"Ethos: {meta_desc or raw_title}. "
        f"Key Sections & Products: {', '.join(headings[:4]) if headings else 'Curated offerings'}."
    )

    return ReferenceAnalysisDTO(
        url=url,
        brand_name=brand_name,
        domain_detected=domain_detected,
        page_title=raw_title,
        meta_description=meta_desc,
        headings=headings,
        categories=categories,
        sample_products=sample_products,
        visual_hints=visual_hints,
        summary=summary,
    )

def fetch_and_analyze_reference(url: str, timeout: float = 12.0) -> Optional[ReferenceAnalysisDTO]:
    """Fetches reference website content via HTTPX and parses structure and design cues."""
    logger.info(f"Ingesting reference website: {url}")
    headers = {
        "User-Agent": (
            "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 "
            "(KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36"
        ),
        "Accept": "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,*/*;q=0.8",
        "Accept-Language": "en-US,en;q=0.9,id;q=0.8",
    }
    try:
        with httpx.Client(timeout=timeout, follow_redirects=True, verify=False) as client:
            resp = client.get(url, headers=headers)
            if resp.status_code == 200 and resp.text:
                dto = analyze_html_content(url, resp.text)
                logger.info(
                    f"Successfully analyzed reference '{url}': "
                    f"Brand='{dto.brand_name}', Domain='{dto.domain_detected}', Headings={len(dto.headings)}"
                )
                return dto
            else:
                logger.warning(f"Failed to fetch reference '{url}': HTTP {resp.status_code}")
    except Exception as e:
        logger.warning(f"Error fetching reference website '{url}': {e}")

    # Fallback minimal heuristic DTO if fetch was blocked or timed out
    parsed_domain = url.split("//")[-1].split("/")[0].replace("www.", "")
    brand_guess = parsed_domain.split(".")[0].replace("-", " ").title()
    return ReferenceAnalysisDTO(
        url=url,
        brand_name=brand_guess,
        domain_detected="Artisanal Bakery & Patisserie" if "bake" in url.lower() else "Curated E-Commerce",
        page_title=f"{brand_guess} - Official Website",
        meta_description=f"Curated portfolio and product catalog for {brand_guess}.",
        headings=[f"Welcome to {brand_guess}", "Signature Collection", "Customer Favorites", "About Our Craft"],
        categories=["Signature Items", "Bestsellers", "New Releases", "Gifts & Hampers"],
        sample_products=[],
        visual_hints={
            "vibe": "Warm elegant artisanal craftsmanship",
            "recommended_colors": ["#FAF8F5", "#2C1810", "#8D5B4C"],
            "theme_mode_preference": "light",
        },
        summary=f"Reference website for {brand_guess}."
    )
