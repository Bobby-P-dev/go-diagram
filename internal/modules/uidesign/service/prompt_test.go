package service

import (
	"testing"

	dtos "github.com/Bobby-P-dev/go-diagram.git/internal/modules/uidesign/dto"
)

func TestInferDesignIntentDoesNotGuessOrOverride(t *testing.T) {
	// Test 1: Generic natural language prompt with potential keyword triggers
	// Old code would guess archetype="dashboard", product_context="ecommerce", density="compact"
	// New v0/desainpakeai compliant code MUST NOT guess; options remain empty unless explicit.
	rawPrompt := "Bikin toko kue dan bakery artisan elegan dengan dashboard penjualan"
	req := dtos.CreateUIDesignRequest{
		Prompt: rawPrompt,
	}

	opts := inferDesignIntent(rawPrompt, &req)

	if opts.Archetype != "" {
		t.Errorf("expected empty Archetype, got %q (keyword guessing must be disabled)", opts.Archetype)
	}
	if opts.ProductContext != "" {
		t.Errorf("expected empty ProductContext, got %q (keyword guessing must be disabled)", opts.ProductContext)
	}
	if opts.Density != "" {
		t.Errorf("expected empty Density, got %q (keyword guessing must be disabled)", opts.Density)
	}
	if req.Foundation != "auto" {
		t.Errorf("expected Foundation to default to 'auto', got %q", req.Foundation)
	}

	// Test 2: User explicitly provides custom options; they must be faithfully preserved.
	explicitReq := dtos.CreateUIDesignRequest{
		Prompt:         rawPrompt,
		Archetype:      "custom_archetype",
		ProductContext: "custom_context",
		Density:        "generous",
		Foundation:     "raycast",
		PrimaryUser:    "Master Baker",
		PrimaryTask:    "Order inventory",
	}

	explicitOpts := inferDesignIntent(rawPrompt, &explicitReq)
	if explicitOpts.Archetype != "custom_archetype" {
		t.Errorf("expected preserved Archetype, got %q", explicitOpts.Archetype)
	}
	if explicitOpts.ProductContext != "custom_context" {
		t.Errorf("expected preserved ProductContext, got %q", explicitOpts.ProductContext)
	}
	if explicitOpts.Density != "generous" {
		t.Errorf("expected preserved Density, got %q", explicitOpts.Density)
	}
	if explicitReq.Foundation != "raycast" {
		t.Errorf("expected preserved Foundation, got %q", explicitReq.Foundation)
	}
}

func TestBuildUIDesignPromptFaithfulToUser(t *testing.T) {
	// Test minimal prompt without metadata bloat
	req := dtos.CreateUIDesignRequest{
		Prompt: "Halaman landing page SaaS AI",
	}
	prompt := buildUIDesignPrompt(req)
	if prompt != "Halaman landing page SaaS AI" {
		t.Errorf("expected clean prompt, got %q", prompt)
	}

	// Test prompt with explicit user metadata
	reqWithMeta := dtos.CreateUIDesignRequest{
		Prompt:      "Halaman landing page SaaS AI",
		CustomTone:  "Minimalist Dark Luxury",
		PrimaryUser: "Lead Engineer",
	}
	promptWithMeta := buildUIDesignPrompt(reqWithMeta)
	expectedSubstrings := []string{
		"Halaman landing page SaaS AI",
		"Requested visual tone: Minimalist Dark Luxury",
		"Primary user: Lead Engineer",
	}
	for _, sub := range expectedSubstrings {
		if !testing.Short() && !contains(promptWithMeta, sub) {
			t.Errorf("expected prompt to contain %q, got:\n%s", sub, promptWithMeta)
		}
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 || (len(s) > 0 && len(substr) > 0 && (stringSearch(s, substr))))
}

func stringSearch(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
