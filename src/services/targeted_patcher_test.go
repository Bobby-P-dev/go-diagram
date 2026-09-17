package services

import (
	"context"
	"strings"
	"testing"

	"github.com/Bobby-P-dev/go-diagram.git/src/dtos"
)

func sampleBakeryHTML() string {
	return `<div class="min-h-screen bg-[#FAF8F5] text-[#2C1810]">
  <!-- Section: navbar -->
  <header id="sec-header" data-rl-id="sec-header" data-rl-kind="section" class="w-full border-b border-stone-200/80 bg-[#FAF7F2]/90 backdrop-blur-md sticky top-0 z-40">
    <div class="max-w-7xl mx-auto px-6 h-18 flex items-center justify-between">
      <div data-rl-id="cmp-logo" data-rl-kind="component" class="flex items-center gap-3 cursor-pointer">
        <span class="font-bold text-lg font-serif">Ann's Bakehouse</span>
      </div>
      <nav data-rl-id="cmp-primary-nav" data-rl-kind="component" class="hidden md:flex items-center gap-8 text-xs font-semibold">
        <a href="#products">Signature Cakes</a>
        <a href="#story">About Us</a>
      </nav>
      <div class="flex items-center gap-3">
        <button data-rl-id="cmp-order-button" data-rl-kind="component" class="px-5 py-2.5 rounded-full text-xs font-bold text-white shadow-md" style="background-color: #D4AF37">Pesan Online</button>
      </div>
    </div>
  </header>

  <!-- Section: hero -->
  <section id="hero" data-rl-id="sec-hero" data-rl-kind="section" class="w-full py-20 px-6 border-b border-stone-200">
    <div class="max-w-7xl mx-auto grid grid-cols-12 gap-12">
      <div class="col-span-7 space-y-6">
        <h1 data-rl-id="cmp-hero-headline" data-rl-kind="component" class="text-5xl font-extrabold font-serif">Artisanal Excellence & Celebration Cakes</h1>
        <p data-rl-id="cmp-hero-sub" data-rl-kind="component" class="text-lg opacity-75">Fresh handcrafted cakes and petite pastries.</p>
      </div>
    </div>
  </section>

  <!-- Section: products -->
  <section id="products" data-rl-id="sec-products" data-rl-kind="section" class="w-full py-16 px-6 border-b border-stone-200">
    <div class="max-w-7xl mx-auto">
      <h2 class="text-3xl font-bold font-serif">Koleksi Pilihan</h2>
      <div data-rl-id="cmp-product-card-1" data-rl-kind="component" class="rounded-2xl border border-stone-200">
        <h3>Classic Tres Leches</h3>
      </div>
    </div>
  </section>

  <!-- Section: footer -->
  <footer id="sec-footer" data-rl-id="sec-footer" data-rl-kind="section" class="w-full py-12 px-6 border-t border-stone-200">
    <p>© 2026 Ann's Bakehouse</p>
  </footer>
</div>`
}

func TestTargetedPatcher_ComponentEdit(t *testing.T) {
	ctx := context.Background()
	patcher := NewTargetedPatcher(nil) // nil AI service uses deterministic heuristics

	frame := &dtos.UIFrameData{
		RawHtml: sampleBakeryHTML(),
		Theme: map[string]interface{}{
			"mode":    "light",
			"primary": "#D4AF37",
		},
		Sections: []dtos.UISectionDTO{
			{ID: "sec-header", Type: "navbar"},
			{ID: "sec-hero", Type: "hero"},
			{ID: "sec-products", Type: "products"},
			{ID: "sec-footer", Type: "footer"},
		},
	}

	plan := &dtos.ChangePlanDTO{
		Request:        "buat button ini lebih minimalis, sedikit kecil dan rounded",
		Classification: "style",
		Scope:          "component",
		Strategy:       "component_patch",
		Target: dtos.TargetElementDTO{
			Type:        "component",
			ID:          "cmp-order-button",
			ComponentID: "cmp-order-button",
			SectionID:   "sec-header",
		},
		PreserveOutsideTarget: true,
	}

	patched, explanation, err := patcher.ApplyTargetedPatch(ctx, frame, plan)
	if err != nil {
		t.Fatalf("ApplyTargetedPatch error: %v", err)
	}

	// 1. Target component must change
	if !strings.Contains(patched.RawHtml, "text-[11px]") && !strings.Contains(patched.RawHtml, "px-4 py-1.5") {
		t.Errorf("expected cmp-order-button to have reduced size, got: %s", patched.RawHtml)
	}

	// 2. Unrelated sections MUST be 100% preserved
	if !strings.Contains(patched.RawHtml, `id="hero" data-rl-id="sec-hero"`) {
		t.Errorf("hero section was accidentally modified or stripped")
	}
	if !strings.Contains(patched.RawHtml, `id="products" data-rl-id="sec-products"`) {
		t.Errorf("products section was accidentally modified or stripped")
	}
	if !strings.Contains(patched.RawHtml, `id="sec-footer" data-rl-id="sec-footer"`) {
		t.Errorf("footer section was accidentally modified or stripped")
	}

	t.Logf("Component edit passed! Explanation: %s", explanation)
}

func TestTargetedPatcher_SectionEdit(t *testing.T) {
	ctx := context.Background()
	patcher := NewTargetedPatcher(nil)

	frame := &dtos.UIFrameData{
		RawHtml: sampleBakeryHTML(),
		Theme: map[string]interface{}{
			"mode":    "light",
			"primary": "#D4AF37",
		},
		Sections: []dtos.UISectionDTO{
			{ID: "sec-header", Type: "navbar"},
			{ID: "sec-hero", Type: "hero"},
			{ID: "sec-products", Type: "products"},
			{ID: "sec-footer", Type: "footer"},
		},
	}

	plan := &dtos.ChangePlanDTO{
		Request:        "header jangan full width, buat floating dengan max width dan rounded",
		Classification: "layout",
		Scope:          "section",
		Strategy:       "section_patch",
		Target: dtos.TargetElementDTO{
			Type:      "section",
			ID:        "sec-header",
			SectionID: "sec-header",
		},
		PreserveOutsideTarget: true,
	}

	patched, explanation, err := patcher.ApplyTargetedPatch(ctx, frame, plan)
	if err != nil {
		t.Fatalf("ApplyTargetedPatch error: %v", err)
	}

	// 1. Header should be floating and rounded
	if !strings.Contains(patched.RawHtml, "rounded-3xl") || !strings.Contains(patched.RawHtml, "max-w-7xl mx-auto") {
		t.Errorf("expected header to have floating max-w-7xl and rounded-3xl, got:\n%s", patched.RawHtml)
	}

	// 2. Sections below header must be completely untouched
	if !strings.Contains(patched.RawHtml, "Artisanal Excellence & Celebration Cakes") {
		t.Errorf("hero content was mutated")
	}
	if !strings.Contains(patched.RawHtml, "Classic Tres Leches") {
		t.Errorf("product content was mutated")
	}

	t.Logf("Section edit passed! Explanation: %s", explanation)
}

func TestChangeAnalyzer_StrictLocality(t *testing.T) {
	ctx := context.Background()
	analyzer := NewChangeAnalyzer(nil)

	// Case 1: User explicitly selects component
	plan1, err := analyzer.AnalyzeTargetedChange(ctx, "buat tombol ini lebih minimalis", &dtos.TargetElementRefDTO{
		Type:      "component",
		ID:        "cmp-order-button",
		SectionID: "sec-header",
	}, nil, nil)
	if err != nil {
		t.Fatalf("AnalyzeTargetedChange error: %v", err)
	}
	if plan1.Scope != "component" || plan1.Strategy != "component_patch" {
		t.Errorf("expected scope component and strategy component_patch, got %s / %s", plan1.Scope, plan1.Strategy)
	}
	if plan1.Target.ID != "cmp-order-button" {
		t.Errorf("expected target cmp-order-button, got %s", plan1.Target.ID)
	}

	// Case 2: User explicitly selects section
	plan2, err := analyzer.AnalyzeTargetedChange(ctx, "perbaiki header nav agar floating", &dtos.TargetElementRefDTO{
		Type: "section",
		ID:   "sec-header",
	}, nil, nil)
	if err != nil {
		t.Fatalf("AnalyzeTargetedChange error: %v", err)
	}
	if plan2.Scope != "section" || plan2.Strategy != "section_patch" {
		t.Errorf("expected scope section and strategy section_patch, got %s / %s", plan2.Scope, plan2.Strategy)
	}

	// Case 3: Implicit section targeting from keyword "header"
	plan3, err := analyzer.AnalyzeTargetedChange(ctx, "perbaiki header nav agar tidak full dan buatkan rounded serta modern", nil, nil, nil)
	if err != nil {
		t.Fatalf("AnalyzeTargetedChange error: %v", err)
	}
	if plan3.Scope != "section" || plan3.Strategy != "section_patch" {
		t.Errorf("expected implicit scope section and strategy section_patch, got %s / %s", plan3.Scope, plan3.Strategy)
	}
	if plan3.Target.ID != "sec-header" {
		t.Errorf("expected implicit target sec-header, got %s", plan3.Target.ID)
	}

	t.Logf("Strict locality test passed!")
}

func TestTargetedPatcher_ImplementationSourceSync(t *testing.T) {
	ctx := context.Background()
	patcher := NewTargetedPatcher(nil)

	initialHTML := sampleBakeryHTML()
	frame := &dtos.UIFrameData{
		RawHtml: initialHTML,
		Implementation: &dtos.ImplementationStateDTO{
			Framework: "vue",
			Styling:   "tailwind",
			Source: dtos.ImplementationSourceDTO{
				HTML: initialHTML,
			},
			Version: 1,
		},
		Sections: []dtos.UISectionDTO{
			{ID: "sec-header", Type: "navbar"},
		},
	}

	plan := &dtos.ChangePlanDTO{
		Request:  "perbaiki header nav agar floating dan rounded",
		Scope:    "section",
		Strategy: "section_patch",
		Target: dtos.TargetElementDTO{
			Type: "section",
			ID:   "sec-header",
		},
	}

	patched, _, err := patcher.ApplyTargetedPatch(ctx, frame, plan)
	if err != nil {
		t.Fatalf("ApplyTargetedPatch error: %v", err)
	}

	// Verify RawHtml, CodeExport, and Implementation.Source.HTML are synchronized!
	if patched.RawHtml != patched.Implementation.Source.HTML {
		t.Errorf("expected Implementation.Source.HTML to match RawHtml, got: %s", patched.Implementation.Source.HTML)
	}
	if patched.RawHtml == initialHTML {
		t.Errorf("expected RawHtml to change after patch")
	}
	if patched.Implementation.Source.HTML == initialHTML {
		t.Errorf("expected Implementation.Source.HTML to change after patch")
	}
}

func TestTargetedPatcher_ZeroFalseSuccess(t *testing.T) {
	ctx := context.Background()
	patcher := NewTargetedPatcher(nil)

	frame := &dtos.UIFrameData{
		RawHtml: sampleBakeryHTML(),
	}

	// Target that does NOT exist in HTML
	planNonExistent := &dtos.ChangePlanDTO{
		Request:  "ubah warna elemen ini",
		Scope:    "component",
		Strategy: "component_patch",
		Target: dtos.TargetElementDTO{
			Type: "component",
			ID:   "cmp-non-existent-button",
		},
	}

	_, _, err := patcher.ApplyTargetedPatch(ctx, frame, planNonExistent)
	if err == nil {
		t.Fatalf("expected error for non-existent target, but got nil (false success)")
	}

	// Empty target
	planEmpty := &dtos.ChangePlanDTO{
		Request:  "ubah warna elemen ini",
		Scope:    "component",
		Strategy: "component_patch",
		Target:   dtos.TargetElementDTO{},
	}

	_, _, err = patcher.ApplyTargetedPatch(ctx, frame, planEmpty)
	if err == nil {
		t.Fatalf("expected error for empty target, but got nil (false success)")
	}
}
