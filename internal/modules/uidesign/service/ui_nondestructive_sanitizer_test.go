package service

import (
	"context"
	"encoding/json"
	"regexp"
	"strings"
	"testing"

	dtos "github.com/Bobby-P-dev/go-diagram.git/internal/modules/uidesign/dto"
)

// TestNonDestructiveSanitizer_PreservesSemanticFooter verifies that semantic <footer>
// is preserved and NOT mutated into <div data-rl-kind="footer">, nor falsely stripped as "cfo".
func TestNonDestructiveSanitizer_PreservesSemanticFooter(t *testing.T) {
	syn := dtos.UIDesignSynthesisDTO{
		Title: "Portal Berita",
		Page:  dtos.RequirementPageDTO{Type: "news", Purpose: "Berita lokal", Complexity: "simple"},
		Goals: dtos.RequirementGoalsDTO{Primary: "Baca berita"},
		Requirements: dtos.RequirementsListDTO{
			Explicit: []string{"Portal berita"},
		},
		Sections: []dtos.UISectionDTO{
			{
				ID:                "sec-main",
				Type:              "articles",
				Purpose:           "Daftar berita",
				RequirementSource: "primary_goal",
			},
			{
				ID:                "sec-footer",
				Type:              "footer",
				Purpose:           "Navigasi kaki halaman",
				RequirementSource: "composition:footer",
			},
		},
		RawHTML: `<main class="p-6">
  <section data-rl-id="sec-main" data-rl-kind="section">
    <h1>Berita Terkini</h1>
  </section>
  <footer id="sec-footer" data-rl-id="sec-footer" data-rl-kind="section" class="py-8 border-t border-slate-200">
    <p>© 2026 Portal Berita. Hak Cipta Dilindungi.</p>
  </footer>
</main>`,
		Theme: map[string]interface{}{"mode": "light"},
	}
	rawBytes, _ := json.Marshal(syn)

	calls := 0
	compiler := NewUIDesignCompiler(bespokeMockAI(t, []string{string(rawBytes)}, &calls))
	dsl, err := compiler.Compile(context.Background(), "Portal berita sederhana", "web", "auto", "light", "")
	if err != nil {
		t.Fatalf("CompileStream failed: %v", err)
	}
	if len(dsl.Frames) == 0 {
		t.Fatal("expected at least 1 frame")
	}

	frame := dsl.Frames[0]
	html := frame.CodeExport["html"]

	// Semantic <footer> must be preserved
	if !strings.Contains(html, "<footer") || !strings.Contains(html, "</footer>") {
		t.Errorf("semantic <footer> tag was destroyed or converted: %s", html)
	}
	if strings.Contains(html, `<div data-rl-kind="footer"`) {
		t.Errorf("found unwanted legacy replacement <div data-rl-kind=\"footer\"")
	}
}

// TestNonDestructiveSanitizer_PreservesBodyAttributes verifies that if LLM wrapped
// output in <body class="...">, the classes are retained on a div instead of being lost.
func TestNonDestructiveSanitizer_PreservesBodyAttributes(t *testing.T) {
	input := `<body class="min-h-screen bg-slate-950 text-white font-sans"><div data-rl-id="sec-main">Content</div></body>`
	sanitized := sanitizeStaticHTML(input)

	if strings.Contains(sanitized, "<body") || strings.Contains(sanitized, "</body>") {
		t.Errorf("body wrapper tag was not converted or stripped: %s", sanitized)
	}
	if !strings.Contains(sanitized, "class=\"min-h-screen bg-slate-950 text-white font-sans\"") {
		t.Errorf("body styling classes were dropped: %s", sanitized)
	}
	if !strings.Contains(sanitized, "Content") {
		t.Errorf("inner content lost: %s", sanitized)
	}
}

// TestNonDestructiveSanitizer_TextContentOnlyReplacement verifies that prohibited domain
// words are only cleansed from visible text nodes, never corrupting attributes, classes, or URLs.
func TestNonDestructiveSanitizer_TextContentOnlyReplacement(t *testing.T) {
	input := `<div class="crypto-box p-4" data-target="crypto"><a href="#crypto" class="text-blue-500">Beli Crypto di Sini</a></div>`
	pattern := regexp.MustCompile(`(?i)\bcrypto\b`)

	result := replaceTextContentOnly(input, pattern, "Digital")

	// Attribute class="crypto-box" and href="#crypto" and data-target="crypto" MUST BE UNTOUCHED
	if !strings.Contains(result, `class="crypto-box p-4"`) {
		t.Errorf("class was corrupted: %s", result)
	}
	if !strings.Contains(result, `href="#crypto"`) {
		t.Errorf("href was corrupted: %s", result)
	}
	if !strings.Contains(result, `data-target="crypto"`) {
		t.Errorf("data-target was corrupted: %s", result)
	}
	// Only text node "Beli Crypto di Sini" should become "Beli Digital di Sini"
	if !strings.Contains(result, "Beli Digital di Sini") {
		t.Errorf("text node was not replaced: %s", result)
	}
}

// TestNonDestructiveSanitizer_PreservesStyleBlockVariables verifies that <style> block
// CSS rules and custom keyframes are untouched during text node replacement.
func TestNonDestructiveSanitizer_PreservesStyleBlockVariables(t *testing.T) {
	input := `<style>
@keyframes pulseGlow { 0%, 100% { opacity: 1; } }
.crypto-card { border-color: rgba(99, 102, 241, 0.2); }
</style>
<div class="crypto-card">Crypto Dashboard</div>`

	pattern := regexp.MustCompile(`(?i)\bcrypto\b`)
	result := replaceTextContentOnly(input, pattern, "Digital")

	if !strings.Contains(result, ".crypto-card { border-color") {
		t.Errorf("CSS rule in <style> was corrupted: %s", result)
	}
	if !strings.Contains(result, "Digital Dashboard") {
		t.Errorf("text node outside style was not cleansed: %s", result)
	}
}

// TestNonDestructiveSanitizer_TableWrappingSafe verifies responsive table handling
// wraps unscrollable tables without forcing hardcoded opinionated border classes.
func TestNonDestructiveSanitizer_TableWrappingSafe(t *testing.T) {
	// Case 1: Raw unwrapped table
	rawTable := `<table class="w-full text-sm"><thead><tr><th>Nama</th></tr></thead><tbody><tr><td>Budi</td></tr></tbody></table>`
	wrapped := sanitizeStaticHTML(rawTable)

	if wrapped != rawTable {
		t.Errorf("sanitizer must not inject table wrappers: %s", wrapped)
	}
	// Ensure no hardcoded conflicting border classes injected
	if strings.Contains(wrapped, "border-slate-200/80") {
		t.Errorf("found unwanted hardcoded border class in table wrapper: %s", wrapped)
	}

	// Case 2: Already wrapped table must NOT be double-wrapped
	alreadyWrapped := `<div class="w-full overflow-x-auto p-2"><table class="w-full"><tr><td>Cell</td></tr></table></div>`
	notDoubleWrapped := sanitizeStaticHTML(alreadyWrapped)
	if strings.Count(notDoubleWrapped, "overflow-x-auto") != 1 {
		t.Errorf("table was double-wrapped: %s", notDoubleWrapped)
	}
}

// TestNonDestructiveSanitizer_SafeSectionStripping verifies that stripping an element
// with deep nested divs does not butcher the parent container or leave unclosed tags.
func TestNonDestructiveSanitizer_SafeSectionStripping(t *testing.T) {
	html := `<main class="root-container">
  <section data-rl-id="sec-hero" class="hero">
    <div class="inner-1">
      <div class="inner-2">
        <h1>Welcome</h1>
      </div>
    </div>
  </section>
  <section data-rl-id="sec-bad" class="bad">
    <div class="child-1">
      <div class="child-2">
        <p>Unwanted</p>
      </div>
    </div>
  </section>
  <section data-rl-id="sec-content" class="content">
    <p>Keep Me</p>
  </section>
</main>`

	_, start, end, err := findHTMLTarget(html, "sec-bad", false)
	if err != nil {
		t.Fatal(err)
	}
	html = html[:start] + html[end:]

	if strings.Contains(html, "Unwanted") || strings.Contains(html, "sec-bad") {
		t.Errorf("target section sec-bad was not stripped: %s", html)
	}
	if !strings.Contains(html, "Welcome") || !strings.Contains(html, "sec-hero") {
		t.Errorf("prior sibling sec-hero was damaged: %s", html)
	}
	if !strings.Contains(html, "Keep Me") || !strings.Contains(html, "sec-content") {
		t.Errorf("posterior sibling sec-content was damaged: %s", html)
	}
	if !strings.HasSuffix(strings.TrimSpace(html), "</main>") {
		t.Errorf("root container closing tag was damaged: %s", html)
	}
}
