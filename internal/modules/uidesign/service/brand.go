package service

import (
	"regexp"
	"strings"
)

// extractBrandSnippet finds the logo/brand element in the HTML
func extractBrandSnippet(html, navSnippet string) string {
	// 1. Try finding explicit brand/logo component IDs
	for _, id := range []string{"comp-brand-logo", "cmp-logo", "brand-logo", "logo", "comp-logo"} {
		if snip, _, _, err := findElementSnippet(html, id); err == nil && snip != "" {
			return snip
		}
	}

	// 2. Search inside navSnippet for the container enclosing the SVG icon AND brand text
	if navSnippet != "" {
		svgIdx := strings.Index(navSnippet, "<svg")
		if svgIdx != -1 {
			openDiv := strings.LastIndex(navSnippet[:svgIdx], "<div")
			if openDiv != -1 {
				// Check parent div if openDiv is just the icon wrapper
				parentDiv := strings.LastIndex(navSnippet[:openDiv], "<div")
				if parentDiv != -1 {
					if parentSnip, _, _, err := findBalancedTagFromIndex(navSnippet, parentDiv); err == nil && parentSnip != "" && len(parentSnip) <= 1500 {
						// Ensure this parent actually contains brand name text
						textOnly := strings.TrimSpace(regexp.MustCompile(`<[^>]*>`).ReplaceAllString(parentSnip, ""))
						if len(textOnly) > 0 {
							return parentSnip
						}
					}
				}
				// If parentDiv wasn't the brand wrapper, try openDiv directly
				if snip, _, _, err := findBalancedTagFromIndex(navSnippet, openDiv); err == nil && snip != "" && len(snip) <= 1200 {
					textOnly := strings.TrimSpace(regexp.MustCompile(`<[^>]*>`).ReplaceAllString(snip, ""))
					if len(textOnly) > 0 {
						return snip
					}
				}
			}
		}

		// Fallback: first div inside navSnippet up to 2500 chars if it has text
		startDiv := strings.Index(navSnippet, "<div")
		if startDiv != -1 {
			if snip, _, _, err := findBalancedTagFromIndex(navSnippet, startDiv); err == nil && snip != "" && len(snip) <= 2500 {
				return snip
			}
		}
	}

	return ""
}
