package services

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/Bobby-P-dev/go-diagram.git/src/dtos"
)

type bespokeMockTransport func(*http.Request) (*http.Response, error)

func (f bespokeMockTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func bespokeMockAI(t *testing.T, outputs []string, calls *int) *AIService {
	t.Helper()
	return &AIService{provider: "openai", model: "mock", baseURL: "https://example.invalid", client: &http.Client{Transport: bespokeMockTransport(func(r *http.Request) (*http.Response, error) {
		index := *calls
		*calls++
		if index >= len(outputs) {
			t.Fatalf("unexpected model call %d", index+1)
		}
		body, _ := json.Marshal(map[string]interface{}{"choices": []interface{}{map[string]interface{}{"message": map[string]interface{}{"content": outputs[index]}}}})
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(string(body))), Header: make(http.Header)}, nil
	})}}
}

func bespokeFixtureOutput(t *testing.T) string {
	t.Helper()
	out := bespokeImplementation{HTML: `<style>.studio{background:#fff;color:#123456;display:grid;gap:2rem;padding:2rem}@media(max-width:600px){.studio{display:block}}</style><main class="studio" data-rl-id="sec-program" data-rl-kind="section"><h1 data-rl-id="cmp-studio_program-main" data-rl-kind="component">Program studio tari</h1><p>Jadwal latihan dan kelas pilihan Anda.</p></main>`, Theme: map[string]interface{}{"mode": "light", "accent": "#123456", "primary": "#123456"}}
	raw, _ := json.Marshal(out)
	return string(raw)
}

func strPtr(s string) *string { return &s }

// synthesisFixtureOutput is a single consolidated synthesis response (the shape
// the merged analyze+plan+implement LLM call returns).
func synthesisFixtureOutput(t *testing.T) string {
	t.Helper()
	syn := dtos.UIDesignSynthesisDTO{
		Title: "Program studio tari",
		Page: dtos.RequirementPageDTO{Type: "studio", Purpose: "show dance program", Complexity: "moderate"},
		Context: dtos.RequirementContextDTO{Domain: strPtr("dance education")},
		Goals:   dtos.RequirementGoalsDTO{Primary: "show dance program"},
		Requirements: dtos.RequirementsListDTO{
			Explicit: []string{"program studio tari"},
		},
		DesignFreedom: &dtos.DesignFreedomDTO{Functional: "low", Visual: "high", Composition: "high"},
		Layout:        dtos.DesignLayoutDTO{Type: "editorial program"},
		Visual:        dtos.DesignVisualDTO{Style: "minimal editorial", Density: "balanced", Theme: "light"},
		Sections: []dtos.UISectionDTO{
			{ID: "sec-program", Type: "studio_program", Purpose: "show dance program", RequirementSource: "primary_goal", Data: map[string]interface{}{"title": "Program studio tari"}},
		},
		RawHTML: `<style>.studio{background:#fff;color:#123456;display:grid;gap:2rem;padding:2rem}@media(max-width:600px){.studio{display:block}}</style><main class="studio" data-rl-id="sec-program" data-rl-kind="section"><h1 data-rl-id="cmp-studio_program-main" data-rl-kind="component">Program studio tari</h1><p>Jadwal latihan dan kelas pilihan Anda.</p></main>`,
		Theme: map[string]interface{}{"mode": "light", "accent": "#123456", "primary": "#123456"},
	}
	raw, _ := json.Marshal(syn)
	return string(raw)
}

func TestBespokeCompilerPreservesMinimalDomainAndModelHTML(t *testing.T) {
	calls := 0
	compiler := NewUIDesignCompiler(bespokeMockAI(t, []string{synthesisFixtureOutput(t)}, &calls))
	dsl, err := compiler.Compile(context.Background(), "Program studio tari minimalis", "web", "custom", "light", "#123456")
	if err != nil {
		t.Fatal(err)
	}
	if calls != 1 {
		t.Fatalf("expected a single consolidated synthesis call, got %d calls", calls)
	}
	if dsl.RequirementSpec.Context.Domain == nil || *dsl.RequirementSpec.Context.Domain != "dance education" {
		t.Fatal("grounded domain was erased by keyword filtering")
	}
	if dsl.RequirementSpec.Page.Complexity != "moderate" || dsl.RequirementSpec.DesignFreedom.Visual != "high" {
		t.Fatal("minimal style incorrectly reduced composition")
	}
	frame := dsl.Frames[0]
	if !strings.Contains(frame.CodeExport["html"], ".studio{background:#fff") || strings.Contains(frame.CodeExport["html"], "Jelajahi Menu") {
		t.Fatal("model implementation was replaced with template")
	}
	if frame.Theme["accent"] != "#123456" || frame.Theme["palette"] != "custom" {
		t.Fatal("requested visual options lost")
	}
	if frame.Implementation.Source.HTML != frame.CodeExport["html"] {
		t.Fatal("canonical implementation diverges from preview")
	}
	if frame.Audit.VisualReview.Status != "not_evaluated" || frame.AntiSlopAudit != nil {
		t.Fatal("unperformed visual audit marked passed")
	}
	if strings.Index(frame.CodeExport["vue"], "<style>") < strings.Index(frame.CodeExport["vue"], "</template>") {
		t.Fatal("Vue style must live outside template")
	}
}

func TestBespokeImplementationRepairsOnceAndFailsExplicitly(t *testing.T) {
	for _, success := range []bool{true, false} {
		name := "fails"
		second := `{"html":""}`
		if success {
			name = "repairs"
			second = bespokeFixtureOutput(t)
		}
		t.Run(name, func(t *testing.T) {
			calls := 0
			compiler := NewUIDesignCompiler(bespokeMockAI(t, []string{`{"html":""}`, second}, &calls))
			spec := &dtos.DesignSpecificationDTO{Sections: []dtos.UISectionDTO{{ID: "sec-program"}}}
			output, _, err := compiler.generateBespokeImplementation(context.Background(), &dtos.RequirementSpecificationDTO{}, spec, "web", "", "light", "#123456")
			if calls != 2 {
				t.Fatalf("repair must be bounded to one extra call; got %d", calls)
			}
			if success && (err != nil || output["html"] == "") {
				t.Fatalf("expected repaired implementation: %v", err)
			}
			if !success && (err == nil || output != nil) {
				t.Fatal("invalid output must fail without generic fallback")
			}
		})
	}
}

func TestBespokeRejectsLostTargetsAndExplicitOptions(t *testing.T) {
	var out bespokeImplementation
	json.Unmarshal([]byte(bespokeFixtureOutput(t)), &out)
	sections := []dtos.UISectionDTO{{ID: "sec-program", Components: []dtos.UIComponentDTO{{ID: "cmp-task"}}}}
	if validateBespokeImplementation(out, sections, "light", "#123456") == nil {
		t.Fatal("lost component target accepted")
	}
	sections[0].Components = nil
	if validateBespokeImplementation(out, sections, "dark", "#123456") == nil {
		t.Fatal("theme override accepted")
	}
	if validateBespokeImplementation(out, sections, "light", "#aa0000") == nil {
		t.Fatal("accent override accepted")
	}
	out.HTML += `<div data-rl-id="sec-program"></div>`
	if validateBespokeImplementation(out, sections, "light", "#123456") == nil {
		t.Fatal("duplicate target accepted")
	}
}

func TestValidatorEmptyOutputDoesNotInventHomepage(t *testing.T) {
	result, spec := NewUIValidator().ValidateAndAudit(&dtos.RequirementSpecificationDTO{}, &dtos.DesignSpecificationDTO{})
	if result.Status != "fail" || len(spec.Sections) != 0 {
		t.Fatal("empty output must fail rather than invent a hero")
	}
}

func TestSynthesisSingleCallRepairsOnceAndFailsExplicitly(t *testing.T) {
	for _, success := range []bool{true, false} {
		name := "fails"
		second := `{"raw_html":"","theme":{"mode":"light"}}`
		if success {
			name = "repairs"
			second = synthesisFixtureOutput(t)
		}
		t.Run(name, func(t *testing.T) {
			calls := 0
			compiler := NewUIDesignCompiler(bespokeMockAI(t, []string{`{"raw_html":"","theme":{"mode":"light"}}`, second}, &calls))
			syn, err := compiler.synthesize(context.Background(), "Program studio tari minimalis", "web", "custom", "light", "#123456")
			if calls != 2 {
				t.Fatalf("repair must be bounded to one extra call; got %d", calls)
			}
			if success && (err != nil || syn == nil || syn.RawHTML == "") {
				t.Fatalf("expected repaired synthesis: %v", err)
			}
			if !success && (err == nil || syn != nil) {
				t.Fatal("invalid output must fail without generic fallback")
			}
		})
	}
}

func TestSynthesisSingleCallDedupesAndGrounds(t *testing.T) {
	// Duplicate section IDs, repeated headlines, and duplicated structural kinds
	// must collapse deterministically after the single call.
	sections := []dtos.UISectionDTO{
		{ID: "sec-hero", Type: "hero", RequirementSource: "primary_goal", Data: map[string]interface{}{"title": "Alpha"}},
		{ID: "sec-hero", Type: "hero", RequirementSource: "primary_goal", Data: map[string]interface{}{"title": "Alpha"}}, // dup id + dup headline
		{ID: "sec-navbar", Type: "navbar", RequirementSource: "navigation", Data: map[string]interface{}{"title": "Beta"}},
		{ID: "sec-header", Type: "header", RequirementSource: "navigation", Data: map[string]interface{}{"title": "Beta"}}, // structural dup nav
		{ID: "sec-footer", Type: "footer", RequirementSource: "navigation", Data: map[string]interface{}{"title": "Gamma"}},
		{ID: "sec-footer2", Type: "footer", RequirementSource: "navigation", Data: map[string]interface{}{"title": "Gamma"}}, // structural dup footer
	}
	got := dedupeSynthesisSections(sections)
	if got == nil {
		t.Fatal("dedup returned nil for non-empty input")
	}
	if len(got) > 4 {
		t.Fatalf("expected duplicates collapsed to at most 4 sections (one hero, one nav, one footer, ...), got %d", len(got))
	}
	seen := map[string]bool{}
	for _, s := range got {
		if seen[s.ID] {
			t.Fatalf("duplicate section id survived dedup: %q", s.ID)
		}
		seen[s.ID] = true
	}

	// HTML duplicate data-rl-id / element id cleanup.
	html := `<section id="main" data-rl-id="sec-main"><div data-rl-id="cmp-card"></div><section id="main" data-rl-id="sec-main"><div data-rl-id="cmp-card"></div>`
	cleaned := sanitizeSynthesisHTMLIDs(html)
	if strings.Count(cleaned, `id="main"`) != 1 {
		t.Fatalf("expected duplicate element id to be suffix-renamed once, got: %s", cleaned)
	}
	if !strings.Contains(cleaned, `sec-main-2`) || !strings.Contains(cleaned, `cmp-card-2`) {
		t.Fatalf("expected duplicate data-rl-ids to be suffix-renamed, got: %s", cleaned)
	}
}
