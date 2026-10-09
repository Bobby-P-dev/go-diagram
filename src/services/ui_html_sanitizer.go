package services

import (
	"regexp"
	"strings"
)

var (
	disallowedScriptBlock   = regexp.MustCompile(`(?is)<\s*script\b[^>]*>.*?<\s*/\s*script\s*>`)
	disallowedScriptTag     = regexp.MustCompile(`(?i)<\s*/?\s*script\b[^>]*>`)
	disallowedIframeBlock   = regexp.MustCompile(`(?is)<\s*iframe\b[^>]*>.*?<\s*/\s*iframe\s*>`)
	disallowedIframeTag     = regexp.MustCompile(`(?i)<\s*/?\s*iframe\b[^>]*>`)
	disallowedObjectBlock   = regexp.MustCompile(`(?is)<\s*object\b[^>]*>.*?<\s*/\s*object\s*>`)
	disallowedObjectTag     = regexp.MustCompile(`(?i)<\s*/?\s*object\b[^>]*>`)
	disallowedEmbedTag      = regexp.MustCompile(`(?i)<\s*/?\s*embed\b[^>]*>`)
	disallowedHeadBlock     = regexp.MustCompile(`(?is)<\s*head\b[^>]*>.*?<\s*/\s*head\s*>`)
	disallowedDocType       = regexp.MustCompile(`(?i)<!DOCTYPE\b[^>]*>`)
	disallowedHTMLWrapper   = regexp.MustCompile(`(?i)<\s*/?\s*(?:html|body)\b[^>]*>`)
	disallowedEventHandler  = regexp.MustCompile(`(?i)(?:\s+)?\bon[a-zA-Z]{2,30}\s*=\s*(?:"[^"]*"|'[^']*'|[^\s>]+)`)
	disallowedJavascriptURI = regexp.MustCompile(`(?i)\b(?:href|src|action)\s*=\s*["']\s*javascript:[^"']*["']`)
	disallowedRawJavascript = regexp.MustCompile(`(?i)javascript\s*:\s*(?:void\s*\(\s*0\s*\)|;)?`)
	disallowedSlopGradient  = regexp.MustCompile(`(?i)\bfrom-(?:purple|violet)-[56]00\s+(?:via-[a-z]+-[56]00\s+)?to-(?:indigo|purple)-[67]00\b`)
	disallowedGlowBlob      = regexp.MustCompile(`(?is)<div\b[^>]*class="[^"]*\bblur-[23]xl\b[^"]*\bbg-(?:purple|indigo|violet|pink)-[456]00/[123]0\b[^"]*"[^>]*>\s*</div>`)
)

// sanitizeStaticHTML cleanses LLM-generated HTML by stripping scripts,
// embedding tags, document wrappers, inline event handlers, and javascript: URIs.
// This ensures the canvas fragment remains 100% static, safe, and valid without
// causing validation failures or triggering expensive LLM repair loops.
func sanitizeStaticHTML(htmlStr string) string {
	if htmlStr == "" {
		return htmlStr
	}

	// 0. Clean accidental markdown fences
	htmlStr = strings.TrimSpace(htmlStr)
	htmlStr = strings.TrimPrefix(htmlStr, "```html")
	htmlStr = strings.TrimPrefix(htmlStr, "```HTML")
	htmlStr = strings.TrimPrefix(htmlStr, "```")
	htmlStr = strings.TrimSuffix(htmlStr, "```")
	htmlStr = strings.TrimSpace(htmlStr)

	// 1. Strip script, iframe, object, embed blocks and standalone tags
	htmlStr = disallowedScriptBlock.ReplaceAllString(htmlStr, "")
	htmlStr = disallowedScriptTag.ReplaceAllString(htmlStr, "")
	htmlStr = disallowedIframeBlock.ReplaceAllString(htmlStr, "")
	htmlStr = disallowedIframeTag.ReplaceAllString(htmlStr, "")
	htmlStr = disallowedObjectBlock.ReplaceAllString(htmlStr, "")
	htmlStr = disallowedObjectTag.ReplaceAllString(htmlStr, "")
	htmlStr = disallowedEmbedTag.ReplaceAllString(htmlStr, "")

	// 2. Strip head blocks and doctype/html/body wrappers (preserving body contents)
	htmlStr = disallowedDocType.ReplaceAllString(htmlStr, "")
	htmlStr = disallowedHeadBlock.ReplaceAllString(htmlStr, "")
	htmlStr = disallowedHTMLWrapper.ReplaceAllString(htmlStr, "")

	// 3. Strip inline event handlers (onsubmit, onclick, onchange, etc.)
	htmlStr = disallowedEventHandler.ReplaceAllString(htmlStr, "")

	// 4. Neutralize javascript: URIs into safe anchor '#'
	htmlStr = disallowedJavascriptURI.ReplaceAllString(htmlStr, `href="#"`)
	htmlStr = disallowedRawJavascript.ReplaceAllString(htmlStr, "#")

	// 5. Clean AI Slop visual clichés (generic purple/indigo gradients and blurry glow orbs)
	htmlStr = disallowedSlopGradient.ReplaceAllString(htmlStr, "from-slate-900 to-slate-950")
	htmlStr = disallowedGlowBlob.ReplaceAllString(htmlStr, "")

	return strings.TrimSpace(htmlStr)
}
