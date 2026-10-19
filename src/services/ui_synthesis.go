package services

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/Bobby-P-dev/go-diagram.git/src/dtos"
)

// Stage 1 Prompt: Context-Driven Art Director & UX Systems Architect.
// Focuses 100% of reasoning on understanding the prompt's unique domain soul,
// inventing a bespoke visual language (palette, typography pairing, spatial rhythm),
// and planning purposeful, creative sections — leaving raw_html empty ("") so Stage 2
// can dedicate 100% of its token budget to crafting unescaped HTML/Tailwind/CSS.
const synthesizePrompt = `You are an Avant-Garde Creative Director and Principal UX Architect (Awwwards Site of the Year, Linear, Stripe Press, Apple Design Award, Teenage Engineering, and Pentagram caliber).

Your job in STAGE 1 is to analyze the user's prompt and architect a BESPOKE, INNOVATIVE Design & UX Blueprint in JSON.
Do NOT write the HTML implementation in this stage — set "raw_html": "" so Stage 2 (the Dedicated Frontend Craftsman) can focus 100% of its token capacity on writing rich, unescaped HTML/CSS/Tailwind markup.

CORE CREATIVE & ARCHITECTURAL MANDATES:

1. ZERO TEMPLATE STYLING — 100% CONTEXT-DRIVEN ART DIRECTION:
   - NEVER fall back to a generic corporate slate/white template! Every prompt has a unique industry soul, emotional temperature, and tactile metaphor:
     * Culinary / Coffee / Bakery / Artisanal: Warm editorial parchment/espresso/terracotta tones, tactile menu cards, editorial serif display typography (Playfair Display / Fraunces), sensory microcopy, origin/tasting badges.
     * Creative Studio / Architecture / Fashion / Portfolio: Brutalist or Asymmetric Editorial rhythm, dramatic oversized display typography (Syne / Space Grotesk / Instrument Serif), stark gallery contrast, numbered index dockets, full-bleed visual framing.
     * FinTech / Crypto / Quant / Trading: High-precision telemetry, tabular-nums financial dockets, interactive-looking sparklines/candlesticks, multi-currency pills, crisp glass or obsidian depth.
     * SaaS / AI / Developer / Canvas Tools: Spatial workspace aesthetic, interactive command bars (⌘K), live interactive-feeling product mockups (node graphs, code diffs, live status pipelines), kinetic bento grids.
     * Healthcare / Wellness / EdTech / Consumer Mobile: Approachable organic geometry, calming biophilic or daylight surfaces, tactile progress rings, schedule timelines, human-centric cards.
     * Enterprise ERP / Operations / POS / Kanban: High-density functional workspace (collapsible sidebar/rail, live filter chips, multi-status KPI cards with mini charts, rich interactive data tables or board lanes).

2. DYNAMIC SPATIAL RHYTHM (ABOLISH MONOTONOUS BOX GRIDS):
   - NEVER plan a boring "Centered Hero -> 3 Identical Icon Boxes -> Footer" wireframe.
   - Plan varied, innovative section compositions in your sections[].purpose and sections[].data:
     * Asymmetric Bento Grids (12-column spans like 7+5 or 8+4+4) where each cell has a distinct interactive widget, mini-chart, or visual preview.
     * Editorial Split Showcases with overlapping floating status cards, live ticker strips, or interactive tabbed switchers.
     * Functional Working Views (when the user asks for an app, dashboard, POS, kanban, booking, or tool, design the ACTUAL working application interface, not a marketing brochure).

3. GROUNDED CONTEXT & ANTI-HALLUCINATION DISCIPLINE:
   - Never inject unrelated finance/CFO/treasury/virtual-card/crypto-wallet features unless the user's prompt is about finance/crypto.
   - If the prompt is generic (e.g. "buat homepage sederhana" or "buat halaman login minimalis"), keep context.domain = null and right-size the sections to what was requested (e.g., login = 1 focused auth section; simple homepage = 2-3 focused sections) while still giving it world-class typographic and visual craft.
   - Never add unrequested marketing fluff (fake customer reviews/testimonials, pricing tiers, or generic FAQ) to internal dashboards or minimal briefs unless requested.
   - Every section MUST have a non-empty "requirement_source" (e.g. "primary_goal", "explicit:...", "composition:spatial_showcase").

OUTPUT JSON SCHEMA (Return valid JSON only, no markdown fences):
{
  "title": "Distinctive, authentic title matching the prompt",
  "page": {"type": "string", "purpose": "string", "complexity": "simple | moderate | complex"},
  "context": {"domain": null, "target_user": null},
  "goals": {"primary": "string"},
  "requirements": {"explicit": []},
  "design_freedom": {"functional": "low", "visual": "high", "composition": "high"},
  "layout": {
    "type": "Specific spatial composition (e.g. Asymmetric Editorial Bento, Split Interactive Workspace, Brutalist Swiss Grid)",
    "container": "max-w-7xl | w-full | max-w-md",
    "alignment": "left | center",
    "spacing": "dynamic rhythm"
  },
  "visual": {
    "style": "Detailed art direction: surface textures, border physics, shadow depth, lighting, and custom visual motifs tailored to this prompt",
    "density": "compact | balanced | generous",
    "theme": "light | dark"
  },
  "typography": {
    "heading": "Google Font name + expressive scale & tracking strategy",
    "body": "Google Font name + readable measure",
    "metadata": "Monospace font + tabular-nums telemetry"
  },
  "sections": [
    {
      "id": "string (e.g. sec-header, sec-hero, sec-workspace, sec-bento)",
      "type": "string (semantic section type)",
      "purpose": "Detailed visual & compositional blueprint for how Stage 2 should build this section (layout spans, interactive widgets, SVG visuals, micro-details)",
      "priority": "high | medium | low",
      "requirement_source": "primary_goal | explicit:xxx | composition:xxx",
      "data": {
        "title": "Section headline",
        "subtitle": "Supportive human copy",
        "visual_blueprint": "Specific UI elements, interactive states, mini-charts, or custom layout details to render inside this section",
        "fields": [{"name": "string", "label": "string", "type": "string"}],
        "social_buttons": [],
        "submit_label": "string"
      }
    }
  ],
  "raw_html": "",
  "theme": {
    "mode": "light | dark",
    "primary": "Hex color tailored to prompt",
    "accent": "Secondary hex accent color",
    "background": "Canvas background hex color",
    "foreground": "Primary text ink hex color",
    "fontFamily": "Google Font name for body (e.g. Plus Jakarta Sans, DM Sans, Outfit, Inter)",
    "headingFont": "Google Font name for display/headings (e.g. Playfair Display, Fraunces, Syne, Space Grotesk, Instrument Serif, Plus Jakarta Sans)",
    "monoFont": "JetBrains Mono"
  }
}`

// Stage 2 Prompt: Dedicated Bespoke HTML/Tailwind/CSS Craftsman.
// Receives the Stage 1 Architecture & Art Direction Blueprint and outputs PURE RAW HTML
// (no JSON wrapper, no string escaping) so 100% of token capacity goes into visual craft.
const bespokeHTMLCraftPrompt = `You are a World-Class Creative Frontend Engineer & Visual Designer (Awwwards, Linear, Stripe, Apple, Teenage Engineering caliber).

You are executing STAGE 2 of the AI Design Compiler.
You will receive the USER REQUEST and the STAGE 1 ART DIRECTION & SECTION BLUEPRINT.
Your sole responsibility is to write the complete, self-contained, deeply innovative HTML + Tailwind CSS + scoped <style> implementation.

CRITICAL OUTPUT FORMAT:
- Output ONLY the raw HTML fragment (optionally starting with a <style> block, followed by a single outer root <div> or <main> container).
- Do NOT wrap in JSON. Do NOT wrap in markdown code fences (` + "```" + `). Do NOT write any explanation before or after the HTML.

HOW TO MAKE THE UI INNOVATIVE, ALIVE, AND NON-RIGID:

1. BESPOKE VISUAL SOUL (NO GENERIC TEMPLATE BOXES):
   - Follow the exact colors (` + "`theme.background`, `theme.foreground`, `theme.primary`, `theme.accent`" + `), fonts (` + "`theme.headingFont`, `theme.fontFamily`, `theme.monoFont`" + `), and ` + "`visual.style`" + ` from the Stage 1 Blueprint.
   - Include a scoped ` + "`<style>`" + ` block at the top to define custom CSS variables, smooth keyframe animations (` + "`@keyframes float`, `@keyframes pulse-glow`" + `), custom atmospheric surface textures (subtle architectural grid, editorial grain, or refined mesh lighting), and custom utility classes.
   - Apply ` + "`font-serif`" + ` or display font styling on headlines when a serif/display ` + "`headingFont`" + ` is chosen, creating dramatic typographic contrast (` + "`text-4xl sm:text-6xl tracking-tight leading-[1.06]`" + `) paired with crisp ` + "`font-mono tabular-nums`" + ` metadata tags.

2. EXPRESSIVE SPATIAL COMPOSITION & BENTO RHYTHM:
   - Never build flat, repetitive rows of identical cards!
   - Use asymmetric Bento grids (` + "`grid-cols-1 md:grid-cols-12`" + ` with varied spans like ` + "`md:col-span-7`" + ` + ` + "`md:col-span-5`" + `, or ` + "`md:col-span-8`" + ` + ` + "`md:col-span-4`" + `), overlapping layered cards, split sticky narratives, or dense multi-pane workspaces.
   - Inside cards and hero showcases, build **living UI artifacts**:
     * Rich inline ` + "`<svg>`" + ` data visualizations (smooth area sparklines with gradient fill, radial progress gauges, node-graph connectors, audio waveforms, or custom architectural illustrations).
     * Interactive-feeling controls: segmented tab pills (` + "`data-tab-group`" + ` / ` + "`data-tab-target`" + `), ` + "`<details class=\"group\"><summary>`" + ` accordions, status badges with animated pulse dots (` + "`animate-ping`" + `), keyboard shortcut badges (` + "`<kbd>`" + `), and hover micro-interactions (` + "`group-hover:translate-x-1 transition-all duration-300`" + `).
     * When photography fits the domain (e-commerce, culinary, architecture, travel, lifestyle, fashion), use curated, reliable Unsplash image URLs (` + "`https://images.unsplash.com/photo-...`" + `) with ` + "`object-cover`" + `, graceful background color fallback, and overlaid glassmorphic badges.

3. STRICT CONTRACT & SAFETY RULES:
   - EVERY section listed in the Blueprint MUST be rendered in the HTML with its exact ` + "`data-rl-id=\"<section.id>\" data-rl-kind=\"section\"`" + ` attribute!
   - Key interactive components (buttons, inputs, cards, navbars, tables) MUST include unique ` + "`data-rl-id=\"cmp-...\" data-rl-kind=\"component\"`" + ` attributes.
   - Static HTML security: NO ` + "`<html>`" + `, ` + "`<head>`" + `, or ` + "`<body>`" + ` tags. NO ` + "`<script>`" + ` tags. NO inline JS event attributes (` + "`onclick`" + `, ` + "`onsubmit`" + `, ` + "`onchange`" + `). NO ` + "`javascript:`" + ` URIs (use ` + "`href=\"#...\"`" + ` or ` + "`type=\"button\"`" + `).
   - Prohibited AI Slop: Do NOT use ` + "`from-purple-600 to-indigo-600`" + ` or ` + "`blur-3xl bg-purple-500/20`" + `. Do NOT use buzzwords (` + "`unlock, elevate, empower, delve, seamless, cutting-edge, revolutionize, supercharge`" + `). Write authentic, concrete human copy in the language of the user's prompt.
   - If ` + "`context.domain`" + ` is null (generic prompt), NEVER invent finance/CFO/treasury/virtual-card/crypto terms.`

// resolveFoundationGuide returns concrete palette, surface, border, and typography
// directions when the user explicitly selects a foundation preset, or instructs the
// LLM to invent a 100% context-driven visual direction when set to "auto"/empty.
func resolveFoundationGuide(foundation, themeMode, accentColor string) string {
	switch strings.ToLower(strings.TrimSpace(foundation)) {
	case "raycast":
		return "Explicit Preset (Raycast Keyboard Dark) — Deep obsidian background (#0B0C0E), elevated matte surface cards (#131519) with crisp top-edge highlight (border-white/[0.08]), electric coral (#FF6363) or indigo (#6366F1) accents, Plus Jakarta Sans + JetBrains Mono kbd shortcuts (⌘K)."
	case "railway":
		return "Explicit Preset (Railway Infrastructure) — Deep space dark canvas (#090A0F), terminal-inspired cards (#11131C) with subtle border-slate-800/90, violet (#8B5CF6) and cyan (#06B6D4) telemetry accents, monospace metrics and live status indicators."
	case "attio":
		return "Explicit Preset (Attio Fluid Workspace) — Crisp high-density workspace (#FFFFFF and #F8FAFC in light, #0D0F14 in dark), subtle 8px border radius, hairline borders (border-slate-200/90), royal blue (#2563EB) accents, multi-attribute status chips, tabular-nums data density."
	case "calcom":
		return "Explicit Preset (Cal.com / Editorial Warm Minimal) — Warm stone/alabaster surface (#FAF9F6 in light, #111111 in dark), stark high-contrast ink (#111827), expressive serif or geometric display headings (Instrument Serif / Playfair Display / Space Grotesk) paired with Plus Jakarta Sans, crisp borders and tactile button contrast."
	case "mintlify":
		return "Explicit Preset (Mintlify Crisp Technical) — Clean daylight/obsidian surfaces, emerald/teal (#0D9488 / #10B981) primary accents, clear hierarchical cards, Plus Jakarta Sans + JetBrains Mono."
	case "ramp":
		return "Explicit Preset (Ramp Clean Precision) — Alabaster slate (#F8FAFC) and crisp white (#FFFFFF) cards (or #0B0F17 in dark mode), sharp hairline borders, high-contrast slate-900 typography, emerald/indigo precision accents, and tabular-nums financial/operational clarity."
	default:
		modeHint := "Auto (choose light or dark based on what best fits the prompt's mood and industry)"
		if themeMode == "dark" {
			modeHint = "Dark Mode (craft a rich, atmospheric dark palette tailored to the prompt's domain — avoid generic flat slate)"
		} else if themeMode == "light" {
			modeHint = "Light Mode (craft a refined, tactile light palette tailored to the prompt's domain — warm paper, crisp alabaster, or domain-tinted surfaces)"
		}
		return "100% CONTEXT-DRIVEN BESPOKE ART DIRECTION (No Preset Template) — Derive the color palette, surface hierarchy, border/shadow treatment, and typography pairing directly from the user's prompt domain and vibe. Theme Mode: " + modeHint
	}
}

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
	if syn.Theme == nil {
		syn.Theme = make(map[string]interface{})
	}
	if themeMode == "light" || themeMode == "dark" {
		mode, _ := syn.Theme["mode"].(string)
		if mode != themeMode {
			syn.Theme["mode"] = themeMode
		}
	}
	if accentColor != "" {
		if syn.Theme["primary"] == nil || syn.Theme["primary"] == "" {
			syn.Theme["primary"] = accentColor
		}
		if syn.Theme["accent"] == nil || syn.Theme["accent"] == "" {
			syn.Theme["accent"] = accentColor
		}
	}
	return nil
}

// extractStage2HTML extracts raw HTML whether the LLM returned pure HTML,
// markdown-fenced HTML, or a JSON wrapper {"raw_html": "..."} / {"html": "..."}.
func extractStage2HTML(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if strings.HasPrefix(trimmed, "{") && strings.HasSuffix(trimmed, "}") {
		var wrapper map[string]interface{}
		if err := json.Unmarshal([]byte(trimmed), &wrapper); err == nil {
			if h, ok := wrapper["raw_html"].(string); ok && strings.TrimSpace(h) != "" {
				return strings.TrimSpace(h)
			}
			if h, ok := wrapper["html"].(string); ok && strings.TrimSpace(h) != "" {
				return strings.TrimSpace(h)
			}
		}
	}
	return sanitizeStaticHTML(trimmed)
}

// craftBespokeHTMLStream executes STAGE 2 of the Multi-LLM Compiler Pipeline:
// a dedicated LLM call using CallLLMTextStream (no JSON-escaping bottleneck!) that
// turns the Stage 1 Art Direction & Section Blueprint into rich, bespoke HTML/Tailwind/CSS.
func (c *UIDesignCompiler) craftBespokeHTMLStream(
	ctx context.Context,
	rawPrompt string,
	device string,
	themeMode string,
	accentColor string,
	syn *dtos.UIDesignSynthesisDTO,
	onChunk LLMStreamCallback,
) (string, error) {
	blueprintSummary := map[string]interface{}{
		"title":      syn.Title,
		"page":       syn.Page,
		"context":    syn.Context,
		"goals":      syn.Goals,
		"layout":     syn.Layout,
		"visual":     syn.Visual,
		"typography": syn.Typography,
		"theme":      syn.Theme,
		"sections":   syn.Sections,
	}
	blueprintBytes, _ := json.MarshalIndent(blueprintSummary, "", "  ")

	var requiredSectionIDs []string
	for _, s := range syn.Sections {
		if s.ID != "" {
			requiredSectionIDs = append(requiredSectionIDs, fmt.Sprintf(`%s (type: %s)`, s.ID, s.Type))
		}
	}

	stage2UserPrompt := fmt.Sprintf(`USER REQUEST:
%s

TARGET DEVICE VIEWPORT: %s
REQUESTED THEME MODE: %s
REQUESTED ACCENT OVERRIDE: %s

STAGE 1 ART DIRECTION & ARCHITECTURE BLUEPRINT:
%s

MANDATORY SECTION TARGETING IDS TO RENDER (each MUST have data-rl-id="<id>" data-rl-kind="section"):
- %s

Now craft the complete, bespoke, high-craft HTML + Tailwind CSS + scoped <style> implementation matching this exact Blueprint. Output raw HTML only:`,
		rawPrompt,
		device,
		themeMode,
		accentColor,
		string(blueprintBytes),
		strings.Join(requiredSectionIDs, "\n- "),
	)

	var htmlRaw string
	var err error
	if onChunk != nil {
		htmlRaw, err = c.aiService.CallLLMTextStream(bespokeHTMLCraftPrompt, nil, stage2UserPrompt, onChunk)
	} else {
		htmlRaw, err = c.aiService.CallLLMText(bespokeHTMLCraftPrompt, nil, stage2UserPrompt)
	}
	if err != nil {
		return "", err
	}
	return extractStage2HTML(htmlRaw), nil
}

// synthesize performs the multi-stage UI generation (or single-call fallback when
// a pre-populated HTML fixture is supplied in unit tests).
func (c *UIDesignCompiler) synthesize(ctx context.Context, rawPrompt, device, foundation, themeMode, accentColor string, opts ...dtos.CompileOptions) (*dtos.UIDesignSynthesisDTO, error) {
	var opt dtos.CompileOptions
	if len(opts) > 0 {
		opt = opts[0]
	}
	return c.synthesizeStream(ctx, rawPrompt, device, foundation, themeMode, accentColor, opt, nil)
}

func (c *UIDesignCompiler) synthesizeStream(ctx context.Context, rawPrompt, device, foundation, themeMode, accentColor string, opts dtos.CompileOptions, onChunk LLMStreamCallback) (*dtos.UIDesignSynthesisDTO, error) {
	contextDetails := ""
	if opts.Archetype != "" {
		contextDetails += fmt.Sprintf("\nDESIGN ARCHETYPE: %s", opts.Archetype)
	}
	if opts.Density != "" {
		contextDetails += fmt.Sprintf("\nSPACING DENSITY: %s", opts.Density)
	}
	if opts.ProductContext != "" {
		contextDetails += fmt.Sprintf("\nPRODUCT CONTEXT: %s", opts.ProductContext)
	}
	if opts.PrimaryUser != "" {
		contextDetails += fmt.Sprintf("\nPRIMARY USER: %s", opts.PrimaryUser)
	}
	if opts.PrimaryTask != "" {
		contextDetails += fmt.Sprintf("\nPRIMARY TASK: %s", opts.PrimaryTask)
	}

	foundationGuide := resolveFoundationGuide(foundation, themeMode, accentColor)

	userMessage := fmt.Sprintf(`USER REQUEST: %s

TARGET DEVICE: %s
ART DIRECTION & FOUNDATION MODE: %s (%s)
THEME MODE: %s
ACCENT COLOR: %s%s

STAGE 1 ARCHITECTURAL DIRECTIVES:
- Design a bespoke visual identity (theme colors, expressive Google Font pairings in theme.headingFont & theme.fontFamily, and spatial layout) tailored specifically to the user's prompt context — never a generic boilerplate template.
- Plan each section with a clear visual_blueprint in section.data so Stage 2 can craft rich interactive widgets, inline SVG visualizations, and asymmetric Bento/editorial layouts.
- Right-size to the brief: minimal requests (e.g. login page) get focused sections; full product/dashboard/landing requests get rich, functional workspace or showcase sections.
- Set "raw_html": "" in this Stage 1 JSON blueprint.`, rawPrompt, device, foundation, foundationGuide, themeMode, accentColor, contextDetails)

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
			syn.Sections = dedupeSynthesisSections(syn.Sections)

			// STAGE 2: Dedicated Bespoke HTML/Tailwind Craftsman LLM Call.
			// Triggered whenever Stage 1 returned a valid section blueprint with empty/minimal RawHTML
			// (whilst preserving 1-call mock compatibility if a test fixture already supplied full RawHTML).
			if len(syn.Sections) > 0 && len(strings.TrimSpace(syn.RawHTML)) < 150 {
				craftedHTML, craftErr := c.craftBespokeHTMLStream(ctx, rawPrompt, device, themeMode, accentColor, &syn, onChunk)
				if craftErr != nil {
					validationErr = fmt.Errorf("stage 2 bespoke HTML craft failed: %w", craftErr)
				} else {
					syn.RawHTML = craftedHTML
				}
			}
		}
		if validationErr == nil {
			syn.RawHTML = sanitizeStaticHTML(syn.RawHTML)
			syn.RawHTML = sanitizeSynthesisHTMLIDs(syn.RawHTML)
			syn.Sections = dedupeSynthesisSections(syn.Sections)
			validationErr = validateSynthesis(&syn, themeMode, accentColor)
		}
		if validationErr == nil {
			return &syn, nil
		}
		prompt = userMessage + "\nThe previous output was invalid: " + validationErr.Error() + ". Return a complete corrected JSON implementation with all required fields and raw_html."
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