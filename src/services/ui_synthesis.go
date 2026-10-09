package services

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/Bobby-P-dev/go-diagram.git/src/dtos"
)

// synthesizePrompt merges the three stage prompts (requirement analysis,
// design planning, bespoke implementation) into ONE directive so a single LLM
// round-trip yields the complete synthesis. The deterministic guards that used
// to run between stages still run as local post-processing.
const synthesizePrompt = `You are a Principal UI/UX Engineer and Visual Designer crafting a bespoke, production-ready interface based strictly on the user brief.

MANDATORY ANTI-SLOP AI DIRECTIVES (CRAFT, ZERO SLOP & PURE CONTEXT ONLY):

1. PURE ONLY CONTEXT & ZERO FABRICATED FEATURES (R-17, R-18, R-28, R-36, R-38):
   - Every single section, card, component, and metric MUST be 100% justified by what the user requested.
   - ABSOLUTE HARD BANS ON UNREQUESTED BOILERPLATE:
     * NO fake testimonials or reviews ("What our clients say", fictional avatars, fake quotes) (R-18).
     * NO fake statistics or fabricated claims ("Trusted by 10,000+ teams", "99.99% satisfaction", "300% faster") without explicit source in brief (R-17, R-36).
     * NO fake "Trusted By" company logo clouds (e.g. logos of Stripe, Google, Meta) (R-36).
     * NO generic template FAQs ("Is my data secure?", "Can I cancel anytime?") (R-28).
     * NO unrequested pricing tiers or subscription cards unless user explicitly requested pricing/paket/harga.
     * NO navbar links pointing to ghost pages or sections that do not exist (R-24).
   - WORKING WORKSPACE OVER PROMOTIONAL SLOP:
     * If user asks for an application, system, or tool (e.g. "kanban board", "task management", "pos cashier", "clinic doctor schedule", "internal platform"):
       Design the ACTUAL FUNCTIONAL WORKING WORKSPACE (navigation/sidebar, filter pills, dense data table/board lanes, status badges, action tools) — NEVER default to a generic promotional marketing landing page!
   - MINIMAL BRIEFS: If the brief is minimal ("buat halaman login", "buat homepage sederhana"), emit ONLY the minimum focused sections (1-3 sections max) that fulfill that exact request. Never pad toward a generic 5-section template.

2. VISUAL & COLOR SLOP BANS (R-01, R-10, R-11, R-12, R-13):
   - FORBIDDEN AI COLOR CLICHÉS:
     * NO generic violet/purple/indigo gradients (e.g. from-purple-600 to-indigo-600, from-violet-500 to-cyan-500) (R-01).
     * NO blurry background radial orbs or floating colored blobs (R-01).
     * NO excessive glow on cards, buttons, and borders simultaneously (R-13). Glow is capped at max 1 key focal accent or omitted entirely.
     * NO full-page glassmorphism blur fatigue. Limit backdrop-blur to max 1 element (e.g. sticky header) or keep solid matte (R-10).
     * NO overly soft muddy blur shadows that make everything float (R-12). Ground surfaces with crisp hairline borders (border-slate-200/80 in light, border-slate-800 in dark) and subtle elevation (shadow-sm).
     * NO uniform pill-button fatigue (rounded-full on every button, badge, input, and card) (R-11). Use intentional radii hierarchy: tight pills for badges, 6-12px for cards, crisp edges for dockets/tickets.
     * NO overused emojis as icons (🚀, ✨, ⚡, 💡, 🔥) (R-04). Use clean, crisp inline SVG icons only.

3. DYNAMIC COMPOSITION & ANTI-TEMPLATE RHYTHM (R-05, R-14):
   - Abolish the cookie-cutter template formula ("Centered Hero with 2 buttons -> Uniform 3-card grid -> Stats -> Footer").
   - FORBIDDEN COPY-PASTE 3-CARD GRIDS: 3 identical cards with identical circle icons and 2 lines of text is the #1 AI slop tell.
   - Use dynamic rhythm and functional asymmetry:
     * Bento compositions with distinct card weights (e.g. 65% dominant feature/workflow + 35% live metric/filter).
     * Split-screen narrative (left-heavy hierarchy, product-as-hero showcase).
     * Interactive widgets embedded directly inside cards (steppers, status toggle pills, search with "⌘K", avatar stacks).

4. DOMAIN-ROOTED ART DIRECTION (GENUINE SOUL & MATERIALITY):
   - The palette and typography must belong to the real-world material of the product:
     * F&B, Bakery, Hospitality: Warm appetizing palette (creams #FDFBF7, warm ambers, terracotta, deep cocoa #382419), tactile order dockets, receipt cards with perforated edge details.
     * Enterprise Ops, ERP, Legal, Internal Tools: Crisp neutral surfaces, high-contrast monochrome, dense tabular data, monospace metrics, clean status badges.
     * Developer, SaaS, Cloud, Infrastructure: Slate-900 obsidian surfaces, hairline borders (border-slate-800), clean indigo/cyan accents, dark cards.
     * FinTech, Ledger: Deep navy or crisp neutral alabaster, tabular monospace numerals, high-density compact tables, clear delta badges.
     * Minimalist, Editorial: Bone/warm-white, stark deep black type, generous editorial whitespace, refined borders.

5. COPYWRITING & BUZZWORD BAN (R-16, R-36):
   - FORBIDDEN AI VOCABULARY: Never use empty words: unlock, elevate, empower, delve, showcase, testament, landscape, journey, robust, game-changer, next-level, seamless, cutting-edge, revolutionize, supercharge.
   - FORBIDDEN SIGNIFICANCE INFLATION: Never write "the future of...", "a new era of...", "marking a pivotal moment".
   - CONCRETE HUMAN MICROCOPY: State exact utility, real actions, and authentic domain details in natural human language.

6. FAST, COMPLETE & LEAN IMPLEMENTATION:
   - Implement a complete, responsive HTML fragment using Pure Tailwind CSS utility classes directly on HTML elements.
   - Do NOT output <style> tags or CSS blocks (use Tailwind utilities exclusively).
   - One outer root element. Body content only — NO <html>/<head>/<body>, NO <script>, NO event handlers (NO onclick, NO onsubmit, NO on* attributes), NO <iframe>, NO javascript: URIs (use href="#" or button type="button").
   - Responsive from 375px through desktop. Ensure input fields have text-base sm:text-sm to prevent mobile auto-zoom. Comfortable touch targets (min 44px height).
   - Targeting IDs: Every section gets a unique data-rl-id and data-rl-kind="section". Key components get data-rl-id and data-rl-kind="component".
   - Hard token budget: keep the entire response under ~4,500 tokens. Prefer fewer, denser, higher-craft sections over many thin ones.
   - Output valid JSON only, no markdown fences, no commentary.

OUTPUT JSON SCHEMA:
{
  "title": "Short descriptive page title",
  "page": {"type": "string", "purpose": "string", "complexity": "simple | moderate | complex"},
  "context": {"domain": null, "target_user": null},
  "goals": {"primary": "string"},
  "requirements": {"explicit": []},
  "sections": [
    {
      "id": "string (e.g. sec-header, sec-hero, sec-catalog)",
      "type": "string (e.g. navbar, hero, bento_grid, form, footer)",
      "purpose": "string",
      "priority": "high | medium | low",
      "requirement_source": "string",
      "data": {
        "title": "string",
        "fields": [{"name": "string", "label": "string", "type": "string"}],
        "social_buttons": ["Google", "GitHub"],
        "submit_label": "string"
      }
    }
  ],
  "raw_html": "complete HTML fragment with Tailwind classes",
  "theme": {"mode": "light | dark", "primary": "CSS color", "accent": "CSS color", "background": "CSS color", "foreground": "CSS color"}
}

WORKED EXAMPLE — MINIMAL BRIEF (what right-sized output looks like):
User: "buat halaman login minimalis"
Correct: 1 section of type "form" (email + password + submit), domain null, complexity simple. NO navbar, hero, features grid, testimonials, or footer, because none were asked for. raw_html is a single centered card. This is the standard for any brief whose request is minimal.
User: "landing page bakery lengkap (hero, menu produk, tentang, kontak)"
Correct: exactly the 4 requested sections (hero, product/menu grid, about, contact) plus a minimal footer, and nothing else. Do not add pricing, testimonials, stats, or a newsletter unless asked.`

// validateSynthesis enforces the same strictness the bespoke implementer used,
// but only requires section-level targeting IDs at synthesis time (component
// IDs are synthesized deterministically afterward).
func validateSynthesis(syn *dtos.UIDesignSynthesisDTO, themeMode, accentColor string) error {
	// First, sanitize static HTML (strips inline event handlers, scripts, document wrappers, javascript: URIs)
	syn.RawHTML = sanitizeStaticHTML(syn.RawHTML)

	// Clean up any trailing cut-off tag (e.g. `<section data-rl-id="sec` cut off before closing `>`)
	trimmedHTML := strings.TrimSpace(syn.RawHTML)
	if lastLt := strings.LastIndex(trimmedHTML, "<"); lastLt != -1 {
		lastGt := strings.LastIndex(trimmedHTML, ">")
		if lastLt > lastGt {
			trimmedHTML = strings.TrimSpace(trimmedHTML[:lastLt])
			syn.RawHTML = trimmedHTML
		}
	}
	if !strings.HasSuffix(strings.TrimSpace(syn.RawHTML), "</div>") && strings.HasPrefix(strings.TrimSpace(syn.RawHTML), "<div") {
		syn.RawHTML = syn.RawHTML + "\n</div>"
	}

	if len(strings.TrimSpace(syn.RawHTML)) < 150 || !strings.Contains(syn.RawHTML, "</") {
		return fmt.Errorf("missing complete HTML implementation")
	}
	if bespokeDisallowedPattern.MatchString(syn.RawHTML) {
		return fmt.Errorf("HTML must be a static body fragment without scripts or event handlers")
	}
	if len(syn.Sections) == 0 {
		return fmt.Errorf("no sections returned")
	}
	seen := map[string]bool{}
	var htmlTargetIDs []string
	for _, m := range bespokeTargetPattern.FindAllStringSubmatch(syn.RawHTML, -1) {
		if seen[m[1]] {
			return fmt.Errorf("duplicate target ID %q", m[1])
		}
		seen[m[1]] = true
		htmlTargetIDs = append(htmlTargetIDs, m[1])
	}
	var validSections []dtos.UISectionDTO
	for idx := range syn.Sections {
		sec := syn.Sections[idx]
		if seen[sec.ID] {
			validSections = append(validSections, sec)
			continue
		}
		// Check 1: id="..." or data-id="..." in HTML
		altPattern := regexp.MustCompile(fmt.Sprintf(`(?i)\b(id|data-id)=["']%s["']`, regexp.QuoteMeta(sec.ID)))
		if altPattern.MatchString(syn.RawHTML) {
			syn.RawHTML = altPattern.ReplaceAllString(syn.RawHTML, fmt.Sprintf(`data-rl-id="%s" data-rl-kind="section"`, sec.ID))
			seen[sec.ID] = true
			validSections = append(validSections, sec)
			continue
		}
		// Check 2: Reconcile with unmatched HTML target IDs
		secTypeClean := strings.ToLower(strings.ReplaceAll(sec.Type, "_", "-"))
		secIDClean := strings.ToLower(strings.ReplaceAll(sec.ID, "_", "-"))
		reconciled := false
		for _, hID := range htmlTargetIDs {
			hIDClaimed := false
			for j, otherSec := range syn.Sections {
				if j != idx && otherSec.ID == hID {
					hIDClaimed = true
					break
				}
			}
			if !hIDClaimed {
				hIDClean := strings.ToLower(strings.ReplaceAll(hID, "_", "-"))
				if strings.Contains(hIDClean, secTypeClean) || strings.Contains(secIDClean, hIDClean) || strings.Contains(hIDClean, secIDClean) {
					delete(seen, sec.ID)
					sec.ID = hID
					seen[hID] = true
					reconciled = true
					validSections = append(validSections, sec)
					break
				}
			}
		}
		if reconciled {
			continue
		}
		// Check 3: Check if an un-targeted <form> or <section> exists and tag it
		tagCandidate := regexp.MustCompile(`(?i)<(form|section|main|div)\b([^>]*?)>`)
		foundUntagged := false
		syn.RawHTML = tagCandidate.ReplaceAllStringFunc(syn.RawHTML, func(match string) string {
			if foundUntagged || strings.Contains(match, "data-rl-id") {
				return match
			}
			matchLower := strings.ToLower(match)
			if strings.Contains(matchLower, secTypeClean) || strings.Contains(matchLower, "form") || strings.Contains(matchLower, "table") || strings.Contains(matchLower, "section") {
				foundUntagged = true
				submatches := tagCandidate.FindStringSubmatch(match)
				return fmt.Sprintf(`<%s data-rl-id="%s" data-rl-kind="section"%s>`, submatches[1], sec.ID, submatches[2])
			}
			return match
		})
		if foundUntagged {
			seen[sec.ID] = true
			validSections = append(validSections, sec)
			continue
		}
		// Check 4: If single section and HTML has at least 1 target ID
		if len(syn.Sections) == 1 && len(htmlTargetIDs) > 0 {
			firstTarget := htmlTargetIDs[0]
			delete(seen, sec.ID)
			sec.ID = firstTarget
			seen[firstTarget] = true
			validSections = append(validSections, sec)
			continue
		}
		// Otherwise, phantom section will simply be omitted if valid sections already exist.
	}
	if len(validSections) > 0 {
		syn.Sections = validSections
	} else {
		return fmt.Errorf("no sections matched HTML targets")
	}
	if themeMode == "light" || themeMode == "dark" {
		if syn.Theme == nil {
			syn.Theme = make(map[string]interface{})
		}
		mode, _ := syn.Theme["mode"].(string)
		if mode != themeMode {
			syn.Theme["mode"] = themeMode
		}
	}
	if accentColor != "" {
		if syn.Theme == nil {
			syn.Theme = make(map[string]interface{})
		}
		if syn.Theme["primary"] == nil || syn.Theme["primary"] == "" {
			syn.Theme["primary"] = accentColor
		}
		if syn.Theme["accent"] == nil || syn.Theme["accent"] == "" {
			syn.Theme["accent"] = accentColor
		}
	}
	return nil
}

// synthesize performs the consolidated single-request generation: ONE LLM call
// returns requirement analysis + design spec + sections + raw HTML + theme,
// then local deterministic hardening (ID sanitization/dedup + requirement and
// design rule enforcement) runs before the caller validates and reconstructs
// the canonical DSL. A single bounded repair retry guards against JSON/HTML drift.
func (c *UIDesignCompiler) synthesize(ctx context.Context, rawPrompt, device, foundation, themeMode, accentColor string) (*dtos.UIDesignSynthesisDTO, error) {
	return c.synthesizeStream(ctx, rawPrompt, device, foundation, themeMode, accentColor, nil)
}

func (c *UIDesignCompiler) synthesizeStream(ctx context.Context, rawPrompt, device, foundation, themeMode, accentColor string, onChunk LLMStreamCallback) (*dtos.UIDesignSynthesisDTO, error) {
	userMessage := fmt.Sprintf(`USER REQUEST: %s

TARGET DEVICE: %s
DESIGN FOUNDATION: %s
THEME MODE: %s
ACCENT COLOR: %s

CRITICAL SCOPING & TOKEN BOUNDS (apply strictly):
- Common UI pattern does not equal user requirement. Every section must have a requirement_source tied to the brief or a justified composition need — navbar/hero/footer/cards are NOT automatic.
- Right-size to this exact request: a minimal brief warrants only the sections it states. Do not pad toward a 3-5 section landing page for a one-part ask (e.g. login, simple homepage).
- If user requests a "design system" or app (e.g. messaging, pos, dashboard), design the primary working application screen showcasing the cohesive design system in practice (3-5 focused sections maximum).
- Do NOT generate an exhaustive 20-component library manual or multi-screen dump. Keep total output compact, high-craft, and strictly under 4,500 tokens. Output valid JSON matching the schema.`, rawPrompt, device, foundation, themeMode, accentColor)

	var validationErr error
	prompt := userMessage
	for attempt := 0; attempt < 2; attempt++ {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		var raw string
		var err error
		if onChunk != nil && attempt == 0 {
			raw, err = c.aiService.CallLLMStream(synthesizePrompt, nil, prompt, onChunk)
		} else {
			raw, err = c.aiService.CallLLM(synthesizePrompt, nil, prompt)
		}
		if err != nil {
			return nil, err
		}
		var syn dtos.UIDesignSynthesisDTO
		validationErr = json.Unmarshal([]byte(c.aiService.SanitizeJSON(raw)), &syn)
		if validationErr == nil {
			syn.RawHTML = sanitizeStaticHTML(syn.RawHTML)
			syn.RawHTML = sanitizeSynthesisHTMLIDs(syn.RawHTML)
			syn.Sections = dedupeSynthesisSections(syn.Sections)
			validationErr = validateSynthesis(&syn, themeMode, accentColor)
		}
		if validationErr == nil {
			return &syn, nil
		}
		prompt = userMessage + "\nThe previous output was invalid: " + validationErr.Error() + ". Return a complete corrected JSON implementation."
	}
	return nil, fmt.Errorf("synthesis output invalid after one repair: %w", validationErr)
}

// finalizeRequirementSpec reconstructs the canonical RequirementSpecificationDTO
// from the single-request synthesis and applies the shared deterministic rules.
func finalizeRequirementSpec(syn *dtos.UIDesignSynthesisDTO, rawPrompt string) *dtos.RequirementSpecificationDTO {
	rs := &dtos.RequirementSpecificationDTO{
		RawPrompt:     rawPrompt,
		Page:          syn.Page,
		Context:       syn.Context,
		Goals:         syn.Goals,
		Requirements:  syn.Requirements,
		DesignFreedom: syn.DesignFreedom,
		Responsive:    true,
	}
	return enforceRequirementRules(rs)
}

// finalizeDesignSpec reconstructs the canonical DesignSpecificationDTO from the
// single-request synthesis: it dedupes sections, then runs the components and
// structured-decision synthesis shared with the legacy planning path.
func finalizeDesignSpec(syn *dtos.UIDesignSynthesisDTO, themeMode string, reqSpec *dtos.RequirementSpecificationDTO) *dtos.DesignSpecificationDTO {
	ds := &dtos.DesignSpecificationDTO{
		Page: dtos.DesignPageDTO{
			Type:       syn.Page.Type,
			Purpose:    syn.Page.Purpose,
			Complexity: syn.Page.Complexity,
		},
		Layout:     syn.Layout,
		Visual:     syn.Visual,
		Typography: syn.Typography,
		Responsive: dtosSynthesisResponsive(syn),
		States:     syn.States,
		Sections:   dedupeSynthesisSections(syn.Sections),
	}

	// Guarantee explicit social buttons are preserved on form sections if requested
	if reqSpec != nil {
		hasGoogle := false
		hasGithub := false
		for _, exp := range reqSpec.Requirements.Explicit {
			expLower := strings.ToLower(exp)
			if strings.Contains(expLower, "google") {
				hasGoogle = true
			}
			if strings.Contains(expLower, "github") {
				hasGithub = true
			}
		}
		if hasGoogle || hasGithub {
			foundForm := false
			for idx := range ds.Sections {
				sec := &ds.Sections[idx]
				sLow := strings.ToLower(sec.Type)
				if sLow == "form" || sLow == "auth_card" || sLow == "login_card" || strings.Contains(sLow, "login") || strings.Contains(sLow, "auth") {
					sec.Type = "form"
					foundForm = true
					if sec.Data == nil {
						sec.Data = make(map[string]interface{})
					}
					var btns []string
					if sList, ok := sec.Data["social_buttons"].([]string); ok {
						btns = sList
					} else if iList, ok := sec.Data["social_buttons"].([]interface{}); ok {
						for _, it := range iList {
							if s, ok := it.(string); ok {
								btns = append(btns, s)
							}
						}
					}
					hasGInBtns := false
					hasGhInBtns := false
					for _, b := range btns {
						bLow := strings.ToLower(b)
						if strings.Contains(bLow, "google") {
							hasGInBtns = true
						}
						if strings.Contains(bLow, "github") {
							hasGhInBtns = true
						}
					}
					if hasGoogle && !hasGInBtns {
						btns = append(btns, "Google")
					}
					if hasGithub && !hasGhInBtns {
						btns = append(btns, "GitHub")
					}
					sec.Data["social_buttons"] = btns
				}
			}
			if !foundForm && len(ds.Sections) > 0 {
				for idx := range ds.Sections {
					sec := &ds.Sections[idx]
					if !isStructuralNav(sec.Type) && !isStructuralFooter(sec.Type) {
						sec.Type = "form"
						if sec.Data == nil {
							sec.Data = make(map[string]interface{})
						}
						var btns []string
						if hasGoogle {
							btns = append(btns, "Google")
						}
						if hasGithub {
							btns = append(btns, "GitHub")
						}
						sec.Data["social_buttons"] = btns
						break
					}
				}
			}
		}
	}

	return populateDesignDetails(ds, themeMode)
}

// dtosSynthesisResponsive ensures responsive fields carry sane defaults when
// the model omitted them.
func dtosSynthesisResponsive(syn *dtos.UIDesignSynthesisDTO) dtos.DesignResponsiveDTO {
	r := syn.Responsive
	if r.Desktop == "" {
		r.Desktop = "multi-column"
	}
	if r.Tablet == "" {
		r.Tablet = "fluid responsive"
	}
	if r.Mobile == "" {
		r.Mobile = "stacked single column"
	}
	return r
}

var (
	sanitizeRlIDPattern = regexp.MustCompile(`data-rl-id="([^"]+)"`)
	sanitizeElemIDPattern = regexp.MustCompile(`(<(?:section|header|footer)\s+[^>]*?id=")([^"]+)(")`)

	genericHeadlineSet = map[string]bool{"": true, "hero": true, "section": true, "products": true, "catalog": true, "reviews": true, "testimonials": true, "cta": true}
)

// nextUniqueID appends the smallest numeric suffix not yet taken.
func nextUniqueID(seen map[string]bool, base string) string {
	suffix := 2
	for seen[fmt.Sprintf("%s-%d", base, suffix)] {
		suffix++
	}
	return fmt.Sprintf("%s-%d", base, suffix)
}

// sanitizeSynthesisHTMLIDs ensures every data-rl-id and section/header/footer id
// is globally unique by suffixing duplicates (port of the CrewAI deduplicator).
func sanitizeSynthesisHTMLIDs(htmlStr string) string {
	if htmlStr == "" {
		return htmlStr
	}
	seenRl := map[string]bool{}
	htmlStr = sanitizeRlIDPattern.ReplaceAllStringFunc(htmlStr, func(m string) string {
		id := sanitizeRlIDPattern.FindStringSubmatch(m)[1]
		if !seenRl[id] {
			seenRl[id] = true
			return m
		}
		newID := nextUniqueID(seenRl, id)
		seenRl[newID] = true
		return fmt.Sprintf(`data-rl-id="%s"`, newID)
	})

	seenElem := map[string]bool{}
	htmlStr = sanitizeElemIDPattern.ReplaceAllStringFunc(htmlStr, func(m string) string {
		parts := sanitizeElemIDPattern.FindStringSubmatch(m)
		prefix, id, suffixQuote := parts[1], parts[2], parts[3]
		if !seenElem[id] {
			seenElem[id] = true
			return m
		}
		newID := nextUniqueID(seenElem, id)
		seenElem[newID] = true
		return prefix + newID + suffixQuote
	})
	return htmlStr
}

func isStructuralNav(t string) bool {
	return strings.Contains(t, "navbar") || strings.Contains(t, "header") || strings.Contains(t, "nav")
}

func isStructuralFooter(t string) bool {
	return strings.Contains(t, "footer")
}

func normHeadline(sec dtos.UISectionDTO) string {
	raw := ""
	if sec.Data != nil {
		raw, _ = sec.Data["headline"].(string)
	}
	if raw == "" && sec.Data != nil {
		raw, _ = sec.Data["title"].(string)
	}
	re := regexp.MustCompile(`[^a-zA-Z0-9\s]`)
	lower := re.ReplaceAllString(strings.ToLower(raw), "")
	return strings.TrimSpace(lower)
}

// dedupeSynthesisSections enforces strict uniqueness of section IDs and filters
// semantic duplicates (repeated structural or identical-headline sections).
func dedupeSynthesisSections(sections []dtos.UISectionDTO) []dtos.UISectionDTO {
	if len(sections) == 0 {
		return nil
	}
	seenIDs := map[string]bool{}
	seenHeadlines := map[string]bool{}
	seenStructural := map[string]bool{}
	cleaned := make([]dtos.UISectionDTO, 0, len(sections))

	for idx, sec := range sections {
		sType := strings.ToLower(strings.TrimSpace(sec.Type))
		if sType == "" {
			sType = "section"
		}
		hn := normHeadline(sec)
		isGeneric := genericHeadlineSet[hn]

		switch {
		case isStructuralNav(sType):
			if seenStructural["nav"] {
				continue
			}
			seenStructural["nav"] = true
		case isStructuralFooter(sType):
			if seenStructural["footer"] {
				continue
			}
			seenStructural["footer"] = true
		default:
			if !isGeneric && seenHeadlines[hn] {
				continue
			}
		}

		secID := strings.ToLower(strings.TrimSpace(sec.ID))
		if secID == "" {
			secType := strings.ReplaceAll(sType, "_", "-")
			secID = fmt.Sprintf("sec-%s-%d", secType, idx+1)
		}
		if seenIDs[secID] {
			base := secID
			suffix := 2
			for seenIDs[fmt.Sprintf("%s-%d", base, suffix)] {
				suffix++
			}
			secID = fmt.Sprintf("%s-%d", base, suffix)
		}
		sec.ID = secID
		seenIDs[secID] = true
		if !isGeneric && hn != "" {
			seenHeadlines[hn] = true
		}
		cleaned = append(cleaned, sec)
	}
	return cleaned
}