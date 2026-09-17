package services

import (
	"testing"

	"github.com/Bobby-P-dev/go-diagram.git/src/dtos"
)

func TestSectionReconciler_FullReplace(t *testing.T) {
	reconciler := NewSectionReconciler(nil)

	current := []dtos.UISectionDTO{
		{ID: "sec-header", Type: "navbar"},
		{ID: "sec-hero", Type: "hero"},
		{ID: "sec-old-footer", Type: "footer"},
	}

	incoming := []dtos.UISectionDTO{
		{ID: "sec-header-new", Type: "navbar"},
		{ID: "sec-hero-new", Type: "hero"},
		{ID: "sec-products", Type: "products"},
		{ID: "sec-footer-new", Type: "footer"},
	}

	res, err := reconciler.Reconcile(ReconcileRequest{
		CurrentSections:  current,
		IncomingSections: incoming,
		Operation:        dtos.OpFullReplace,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if res.AfterCount != 4 {
		t.Errorf("expected 4 sections after FULL_REPLACE, got %d", res.AfterCount)
	}
	if res.Sections[0].ID != "sec-header-new" {
		t.Errorf("expected first section to be sec-header-new, got %s", res.Sections[0].ID)
	}
}

func TestSectionReconciler_SectionPatch(t *testing.T) {
	reconciler := NewSectionReconciler(nil)

	current := []dtos.UISectionDTO{
		{ID: "sec-header", Type: "navbar", Data: map[string]interface{}{"title": "Old Brand"}},
		{ID: "sec-philosophy", Type: "brand_story", Data: map[string]interface{}{"headline": "Craft & Care"}},
		{ID: "sec-footer", Type: "footer"},
	}

	patch := []dtos.UISectionDTO{
		{
			ID:   "sec-philosophy",
			Data: map[string]interface{}{"headline": "Updated Philosophy Headline", "badge": "New"},
		},
	}

	res, err := reconciler.Reconcile(ReconcileRequest{
		CurrentSections:  current,
		IncomingSections: patch,
		Operation:        dtos.OpSectionPatch,
		TargetID:         "sec-philosophy",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if res.AfterCount != 3 {
		t.Errorf("expected section count to remain 3, got %d", res.AfterCount)
	}
	if res.Sections[1].ID != "sec-philosophy" {
		t.Errorf("expected stable ID to be preserved, got %s", res.Sections[1].ID)
	}
	if res.Sections[1].Data["headline"] != "Updated Philosophy Headline" {
		t.Errorf("expected updated headline, got %v", res.Sections[1].Data["headline"])
	}
	// Verify other sections are completely intact
	if res.Sections[0].Data["title"] != "Old Brand" {
		t.Errorf("unrelated section was modified")
	}
}

func TestSectionReconciler_InsertSection(t *testing.T) {
	reconciler := NewSectionReconciler(nil)

	current := []dtos.UISectionDTO{
		{ID: "sec-header", Type: "navbar"},
		{ID: "sec-hero", Type: "hero"},
		{ID: "sec-footer", Type: "footer"},
	}

	newSec := []dtos.UISectionDTO{
		{ID: "sec-testimonials", Type: "testimonials"},
	}

	res, err := reconciler.Reconcile(ReconcileRequest{
		CurrentSections:  current,
		IncomingSections: newSec,
		Operation:        dtos.OpInsertSection,
		TargetID:         "sec-hero", // Insert after hero
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if res.AfterCount != 4 {
		t.Errorf("expected 4 sections after insert, got %d", res.AfterCount)
	}
	if res.Sections[2].ID != "sec-testimonials" {
		t.Errorf("expected sec-testimonials at index 2, got %s", res.Sections[2].ID)
	}

	// Test duplicate ID rejection
	duplicate := []dtos.UISectionDTO{
		{ID: "sec-hero", Type: "hero"},
	}
	_, errDup := reconciler.Reconcile(ReconcileRequest{
		CurrentSections:  current,
		IncomingSections: duplicate,
		Operation:        dtos.OpInsertSection,
	})
	if errDup == nil {
		t.Errorf("expected error inserting duplicate section ID, got nil")
	}
}

func TestSectionReconciler_DeleteSection(t *testing.T) {
	reconciler := NewSectionReconciler(nil)

	current := []dtos.UISectionDTO{
		{ID: "sec-header", Type: "navbar"},
		{ID: "sec-hero", Type: "hero"},
		{ID: "sec-testimonials", Type: "testimonials"},
		{ID: "sec-footer", Type: "footer"},
	}

	res, err := reconciler.Reconcile(ReconcileRequest{
		CurrentSections: current,
		Operation:       dtos.OpDeleteSection,
		TargetID:        "sec-testimonials",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if res.AfterCount != 3 {
		t.Errorf("expected 3 sections, got %d", res.AfterCount)
	}
	for _, s := range res.Sections {
		if s.ID == "sec-testimonials" {
			t.Errorf("sec-testimonials was not deleted")
		}
	}
}

func TestDuplicateValidator_HTMLDuplicates(t *testing.T) {
	validator := NewDuplicateValidator()

	cleanHTML := `
		<section data-rl-id="sec-header">...</section>
		<section data-rl-id="sec-hero">...</section>
		<section data-rl-id="sec-footer">...</section>
	`
	if err := validator.ValidateHTMLDuplicateIDs(cleanHTML); err != nil {
		t.Errorf("unexpected error on clean HTML: %v", err)
	}

	dirtyHTML := `
		<section data-rl-id="sec-spotlight">First spotlight</section>
		<section data-rl-id="sec-spotlight">Second duplicate spotlight</section>
	`
	if err := validator.ValidateHTMLDuplicateIDs(dirtyHTML); err == nil {
		t.Errorf("expected error on duplicate data-rl-id, got nil")
	}
}

func TestDuplicateValidator_SemanticDuplicates(t *testing.T) {
	validator := NewDuplicateValidator()

	cleanSections := []dtos.UISectionDTO{
		{ID: "sec-1", Type: "hero", Data: map[string]interface{}{"headline": "Fresh Artisanal Bakes"}},
		{ID: "sec-2", Type: "brand_story", Data: map[string]interface{}{"headline": "Our Heritage in Fermentation"}},
	}
	if err := validator.ValidateSemanticDuplicates(cleanSections, dtos.OpFullReplace); err != nil {
		t.Errorf("unexpected error on clean sections: %v", err)
	}

	duplicateSections := []dtos.UISectionDTO{
		{ID: "sec-story-1", Type: "spotlight", Data: map[string]interface{}{"headline": "Dedikasi Dapur Artisan & Bahan Murni", "image": "https://img.com/1.jpg"}},
		{ID: "sec-story-2", Type: "spotlight", Data: map[string]interface{}{"headline": "Dedikasi Dapur Artisan & Bahan Murni", "image": "https://img.com/1.jpg"}},
	}
	if err := validator.ValidateSemanticDuplicates(duplicateSections, dtos.OpFullReplace); err == nil {
		t.Errorf("expected error on duplicate headline and image, got nil")
	}
}
