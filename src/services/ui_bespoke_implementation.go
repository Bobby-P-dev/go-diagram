package services

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/Bobby-P-dev/go-diagram.git/src/dtos"
)

const bespokeImplementationPrompt = `You are a UI designer implementing a specific user brief, not filling a template.
Return JSON only: {"html":"complete HTML fragment including its scoped CSS","theme":{"mode":"light or dark","primary":"CSS color","accent":"CSS color","background":"CSS color","foreground":"CSS color","palette":"visual direction"}}.
Implement the supplied validated requirement and design specification as a finished responsive screen. Preserve its exact content, language, section purposes, domain, and explicitly requested visual style. Use typography, composition, density and media appropriate to this particular screen. No mandatory split hero, card grid, testimonials or section count. No fabricated reviews, performance claims or unrelated products. Blank/auto visual options mean infer from the brief; explicit options override recommendations.
Use semantic HTML, scoped CSS and optionally Tailwind utilities. Include a style element for custom fonts, layout and responsive behavior when needed. Support 375px through desktop without overflow. Body content only, no html/head/body wrappers, scripts, event handlers or framework directives. Use accessible labels, focus styles and suitable contrast.
Every section and component in the specification must appear exactly once with its exact data-rl-id and data-rl-kind="section" or "component". Extra implementation elements may have unique IDs. Never duplicate targeting IDs. These IDs support future canvas edits.
Use one outer root element. The output must contain actual finished HTML, never an empty string, code fences, a plan or placeholders. Do not claim visual testing has been performed.`

type bespokeImplementation struct {
	HTML  string                 `json:"html"`
	Theme map[string]interface{} `json:"theme"`
}

var bespokeTargetPattern = regexp.MustCompile(`(?i)\bdata-rl-id\s*=\s*["']([^"']+)["']`)
var bespokeStylePattern = regexp.MustCompile(`(?is)<style\b[^>]*>(.*?)</style\s*>`)
var bespokeDisallowedPattern = regexp.MustCompile(`(?i)<\s*(script|iframe|object|embed|html|head|body)\b|\bon[a-z]+\s*=|javascript\s*:`)

func validateBespokeImplementation(out bespokeImplementation, sections []dtos.UISectionDTO, themeMode, accentColor string) error {
	if len(strings.TrimSpace(out.HTML)) < 150 || !strings.Contains(out.HTML, "</") {
		return fmt.Errorf("missing complete HTML implementation")
	}
	if bespokeDisallowedPattern.MatchString(out.HTML) {
		return fmt.Errorf("HTML must be a static body fragment without scripts or event handlers")
	}
	seen := map[string]bool{}
	for _, match := range bespokeTargetPattern.FindAllStringSubmatch(out.HTML, -1) {
		if seen[match[1]] {
			return fmt.Errorf("duplicate target ID %q", match[1])
		}
		seen[match[1]] = true
	}
	for _, section := range sections {
		if !seen[section.ID] {
			return fmt.Errorf("missing section target %q", section.ID)
		}
		for _, component := range section.Components {
			if component.ID != "" && !seen[component.ID] {
				return fmt.Errorf("missing component target %q", component.ID)
			}
		}
	}
	mode, _ := out.Theme["mode"].(string)
	if mode != "light" && mode != "dark" {
		return fmt.Errorf("theme.mode must resolve to light or dark")
	}
	if (themeMode == "light" || themeMode == "dark") && mode != themeMode {
		return fmt.Errorf("requested theme mode %q was not preserved", themeMode)
	}
	if accentColor != "" {
		accent, _ := out.Theme["accent"].(string)
		if !strings.EqualFold(accent, accentColor) || !strings.Contains(strings.ToLower(out.HTML), strings.ToLower(accentColor)) {
			return fmt.Errorf("requested accent %q must be present in theme and HTML", accentColor)
		}
	}
	return nil
}

// splitHTMLToCodeExport converts a self-contained HTML fragment (with optional
// embedded <style> blocks) into the canonical code_export map: the raw html
// plus a Vue SFC whose <style> blocks live outside the <template>.
func splitHTMLToCodeExport(html string) map[string]string {
	html = strings.TrimSpace(html)
	styles := bespokeStylePattern.FindAllString(html, -1)
	markup := bespokeStylePattern.ReplaceAllString(html, "")
	vue := "<template>\n" + strings.TrimSpace(markup) + "\n</template>\n" + strings.Join(styles, "\n")
	return map[string]string{"html": html, "vue": vue}
}

func (c *UIDesignCompiler) generateBespokeImplementation(ctx context.Context, req *dtos.RequirementSpecificationDTO, spec *dtos.DesignSpecificationDTO, device, foundation, themeMode, accentColor string) (map[string]string, map[string]interface{}, error) {
	if err := NewDuplicateValidator().ValidateDuplicateSectionIDs(spec.Sections); err != nil {
		return nil, nil, err
	}
	payload, err := json.Marshal(map[string]interface{}{"requirements": req, "design": spec, "device": device, "foundation": foundation, "theme_mode": themeMode, "accent_color": accentColor})
	if err != nil {
		return nil, nil, err
	}
	prompt := string(payload)
	var validationErr error
	for attempt := 0; attempt < 2; attempt++ {
		if err := ctx.Err(); err != nil {
			return nil, nil, err
		}
		raw, err := c.aiService.CallLLM(bespokeImplementationPrompt, nil, prompt)
		if err != nil {
			return nil, nil, err
		}
		var out bespokeImplementation
		validationErr = json.Unmarshal([]byte(c.aiService.SanitizeJSON(raw)), &out)
		if validationErr == nil {
			validationErr = validateBespokeImplementation(out, spec.Sections, themeMode, accentColor)
		}
		if validationErr == nil {
			html := strings.TrimSpace(out.HTML)
			codeExport := splitHTMLToCodeExport(html)
			if foundation != "" && foundation != "auto" {
				out.Theme["palette"] = foundation
			}
			return codeExport, out.Theme, nil
		}
		prompt = string(payload) + "\nThe previous implementation was invalid: " + validationErr.Error() + ". Return a complete corrected JSON implementation."
	}
	return nil, nil, fmt.Errorf("bespoke output invalid after one repair: %w", validationErr)
}
