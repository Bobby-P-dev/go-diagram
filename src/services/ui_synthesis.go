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

CORE PRINCIPLES (ANTI-SLOP CRAFT & HIGH AESTHETIC):
1. Bespoke & Prompt-Grounded: Never use a rigid cookie-cutter template. Tailor the layout, tone, and components directly to the user's specific domain and intent. Do NOT invent unrelated business domains, stats, or reviews. If domain is not specified, keep context.domain null and complexity simple. For generic requests (e.g. 'buat homepage sederhana'), NEVER generate financial/CFO/crypto/treasury/credit limit widgets or titles. Common UI pattern is NOT a requirement — never invent unrequested features.
2. Dynamic Composition & Rhythm: Avoid uniform 3-card grids or monotonous centered heroes. Use rhythmic asymmetry and bento-style compositions (e.g. 60/40 hero + tactile widget/stat card, varied column spans).
3. Domain-Rooted Visual Direction:
   - F&B, Bakery, Hospitality: Warm appetizing palette (creams #FDFBF7, warm ambers, terracotta, deep cocoa #382419), tactile dockets/cards.
   - Dev, SaaS, Cloud: Obsidian/slate-900 surfaces, hairline borders (border-slate-800), clean indigo/cyan accents, dark cards.
   - Enterprise, ERP, FinTech: Crisp neutral surfaces, high-contrast data tables, monospace metrics, emerald/navy badges.
   - Minimalist, Studio, Portfolio: Bone/warm-white, stark deep black type, generous editorial whitespace, refined borders.
   - Digital Marketplace & Ecommerce: Clean neutral/warm-white background, prominent search with instant filter pills, rich product cards with real preview thumbnails, seller badges, pricing, star ratings, and clear purchase CTAs.
4. Tactile Realism & Authentic Microcopy:
   - Use concrete, human text and realistic data (e.g. real pricing, genuine metrics like "99.98% SLA", realistic product names). Never use generic buzzwords ("supercharge your workflow", "seamless experience") or "Lorem ipsum".
   - Rich interactive details: active filter pills, search input with keyboard shortcut ("⌘K"), avatar stacks ("+12"), status indicators with pulsing dots (w-2 h-2 rounded-full bg-emerald-500 animate-pulse).
   - Clean inline SVG icons: Use concise inline SVG icons (viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2") where appropriate. Do NOT use emojis as icons.
5. Section Coverage & Completeness:
   - If the user brief specifies explicit sections: You MUST implement and include ALL of those requested sections in BOTH the "sections" array AND raw_html. Do NOT drop requested sections.
   - If the request is brief or unspecified, provide 3-5 comprehensive sections that tell a complete product story.
   - ANTI-OVERGENERATION FOR "DESIGN SYSTEM" OR APP PROMPTS: If the user asks for a "design system", "design sistem", or app (e.g. 'design sistem massage apps', 'pos app', 'chat app'): DO NOT produce a 20-component documentation manual! INSTEAD, design the actual primary application workspace screen (e.g. Navigation Header, Sidebar/Channels, Main Workspace & Feed, Action Tools) showcasing the design system in practice.
   - STRICT SECTION BOUNDS: Strictly cap output to 3-5 high-craft sections. Never exceed 5 sections unless the user explicitly gave an enumerated list with more sections.
   - FAST & LEAN GENERATION: To fit within token limits and maximize generation speed, render 3-4 distinct representative items with realistic data and polish (e.g. 3-4 chat messages, cards, or pills) rather than repeating 10+ identical rows.
   - ALWAYS output the complete page markup all the way down to the final footer element and closing outer root container (</div>). Never stop generating halfway.
6. Technical Implementation:
   - Implement a complete, responsive HTML fragment using Pure Tailwind CSS utility classes directly on HTML elements.
   - Do NOT output <style> tags or CSS blocks (use Tailwind utilities exclusively for fast, clean rendering).
   - One outer root element. Body content only — NO <html>/<head>/<body>, NO <script>, NO event handlers (NO onclick, NO onsubmit, NO on* attributes), NO <iframe>, NO javascript: URIs (use href="#" or button type="button").
   - Responsive from 375px through desktop. Ensure input fields have text-base sm:text-sm to prevent mobile auto-zoom. Comfortable touch targets (min 44px height).
7. Targeting IDs:
   - Every section in the HTML MUST have a unique data-rl-id and data-rl-kind="section" matching the "sections" array.
   - Key interactive components (buttons, inputs, cards) get unique data-rl-id and data-rl-kind="component". Never duplicate targeting IDs.

8. CONCISE OUTPUT (speed & responsiveness):
   - Focus generation tokens directly on the rich HTML UI. Avoid unnecessary JSON metadata bloat.
   - raw_html must be a purpose-built fragment with elegant Tailwind classes and clean hierarchy.

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

Output valid JSON only, no markdown fences, no commentary. The raw_html must contain actual finished markup, never a placeholder. Do not claim visual testing was performed.`

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

CRITICAL SCOPING & TOKEN BOUNDS:
- Common UI pattern does not equal user requirement. Every section needs a requirement_source.
- If user requests a "design system" or app (e.g. messaging, pos, dashboard), design the primary working application screen showcasing the cohesive design system in practice (3-5 focused sections maximum).
- Do NOT generate an exhaustive 20-component library manual or multi-screen dump. Keep total output compact, high-craft, and strictly under 4,000 tokens. Output valid JSON matching the schema.`, rawPrompt, device, foundation, themeMode, accentColor)

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