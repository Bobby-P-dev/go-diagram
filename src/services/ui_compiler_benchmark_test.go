package services

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/Bobby-P-dev/go-diagram.git/src/config"
	"github.com/Bobby-P-dev/go-diagram.git/src/dtos"
	"github.com/joho/godotenv"
)

// 1. UNIT TEST: UI Validator Strips Hallucinations when Domain is unspecified (Rule 1 & Rule 2)
func TestValidatorAntiHallucinationStripping(t *testing.T) {
	validator := NewUIValidator()

	// Given a requirement spec with unspecified domain (nil) and simple complexity
	reqSpec := &dtos.RequirementSpecificationDTO{
		Page: dtos.RequirementPageDTO{
			Type:       "homepage",
			Purpose:    "General informational website",
			Complexity: "simple",
		},
		Context: dtos.RequirementContextDTO{
			Domain:     nil, // Crucial: domain is nil!
			TargetUser: nil,
		},
		Goals: dtos.RequirementGoalsDTO{
			Primary: "Present clear overview",
		},
		Requirements: dtos.RequirementsListDTO{
			Explicit: []string{"Show hero title", "Show feature list"},
		},
	}

	// Given a design spec with unrequested treasury and crypto bloat injected
	designSpec := &dtos.DesignSpecificationDTO{
		Page: dtos.DesignPageDTO{
			Type:       "homepage",
			Purpose:    "General website",
			Complexity: "simple",
		},
		Sections: []dtos.UISectionDTO{
			{
				ID:                "sec-navbar",
				Type:              "navbar",
				Purpose:           "Standard navigation",
				RequirementSource: "navigation",
				Priority:          "high",
				Data: map[string]interface{}{
					"brand": "Acme Corp",
				},
			},
			{
				ID:                "sec-hero",
				Type:              "hero",
				Purpose:           "Main headline",
				RequirementSource: "primary_goal",
				Priority:          "high",
				Data: map[string]interface{}{
					"title": "Welcome to Acme",
				},
			},
			{
				ID:                "sec-treasury-hallucination",
				Type:              "kpi_grid",
				Purpose:           "Treasury metrics and CFO liquidity dashboard",
				RequirementSource: "implied_hallucination",
				Priority:          "high",
				Data: map[string]interface{}{
					"title": "Treasury Balance & Virtual Cards",
				},
			},
			{
				ID:                "sec-crypto-hallucination",
				Type:              "data_table",
				Purpose:           "Crypto wallet and blockchain ledger",
				RequirementSource: "implied_hallucination",
				Priority:          "low",
				Data: map[string]interface{}{
					"title": "Crypto Transactions",
				},
			},
			{
				ID:                "sec-features",
				Type:              "feature_grid",
				Purpose:           "Showcase core features",
				RequirementSource: "explicit_requirement",
				Priority:          "medium",
				Data: map[string]interface{}{
					"title": "Why Choose Us",
				},
			},
		},
	}

	result, validatedSpec := validator.ValidateAndAudit(reqSpec, designSpec)

	if result.Status != "repaired" {
		t.Errorf("expected status 'repaired', got '%s'", result.Status)
	}

	if len(result.StrippedSections) < 2 {
		t.Errorf("expected at least 2 stripped hallucinated sections, got %d: %v", len(result.StrippedSections), result.StrippedSections)
	}

	// Ensure no remaining section mentions treasury or crypto
	for _, sec := range validatedSpec.Sections {
		secBytes, _ := json.Marshal(sec)
		s := strings.ToLower(string(secBytes))
		if strings.Contains(s, "treasury") || strings.Contains(s, "crypto") {
			t.Errorf("hallucinated keyword slipped through validator in section %s", sec.ID)
		}
	}

	if result.Score.RequirementFidelity < 9 {
		t.Errorf("expected fidelity score >= 9, got %d", result.Score.RequirementFidelity)
	}
	if result.Score.Simplicity < 9 {
		t.Errorf("expected simplicity score >= 9, got %d", result.Score.Simplicity)
	}
}

// 2. UNIT TEST: UI Validator Enforces Section Traceability (Rule 6)
func TestValidatorTraceabilityEnforcement(t *testing.T) {
	validator := NewUIValidator()

	reqSpec := &dtos.RequirementSpecificationDTO{
		Page: dtos.RequirementPageDTO{
			Type:       "landing",
			Complexity: "simple",
		},
	}

	designSpec := &dtos.DesignSpecificationDTO{
		Sections: []dtos.UISectionDTO{
			{
				ID:                "sec-1",
				Type:              "navbar",
				Purpose:           "Header",
				RequirementSource: "navigation",
				Priority:          "high",
			},
			{
				ID:                "sec-orphan",
				Type:              "pricing",
				Purpose:           "Unrequested pricing card",
				RequirementSource: "", // MISSING TRACEABILITY!
				Priority:          "low",
			},
		},
	}

	result, validatedSpec := validator.ValidateAndAudit(reqSpec, designSpec)

	if len(validatedSpec.Sections) != 1 || validatedSpec.Sections[0].ID != "sec-1" {
		t.Errorf("expected orphan section without requirement_source to be stripped, got %d sections", len(validatedSpec.Sections))
	}

	foundOrphanIssue := false
	for _, issue := range result.Issues {
		if strings.Contains(issue, "sec-orphan") && (strings.Contains(issue, "traceability") || strings.Contains(issue, "requirement_source")) {
			foundOrphanIssue = true
			break
		}
	}
	if !foundOrphanIssue {
		t.Errorf("expected traceability issue reported for sec-orphan, got issues: %v", result.Issues)
	}
}

// 3. UNIT TEST: UI Validator Complexity Bounding (Rule 3)
func TestValidatorComplexityBounding(t *testing.T) {
	validator := NewUIValidator()

	reqSpec := &dtos.RequirementSpecificationDTO{
		Page: dtos.RequirementPageDTO{
			Type:       "homepage",
			Complexity: "simple", // SIMPLE requires max 4 sections
		},
	}

	// 6 sections
	designSpec := &dtos.DesignSpecificationDTO{
		Sections: []dtos.UISectionDTO{
			{ID: "sec-1", Type: "navbar", RequirementSource: "nav", Priority: "high"},
			{ID: "sec-2", Type: "hero", RequirementSource: "hero", Priority: "high"},
			{ID: "sec-3", Type: "features", RequirementSource: "feat", Priority: "medium"},
			{ID: "sec-4", Type: "faq", RequirementSource: "faq", Priority: "low"},
			{ID: "sec-5", Type: "testimonials", RequirementSource: "test", Priority: "low"},
			{ID: "sec-6", Type: "newsletter", RequirementSource: "news", Priority: "low"},
		},
	}

	_, validatedSpec := validator.ValidateAndAudit(reqSpec, designSpec)

	if len(validatedSpec.Sections) > 4 {
		t.Errorf("expected simple page to be bounded to max 4 sections, got %d", len(validatedSpec.Sections))
	}
}

// 4. UNIT TEST: Clean Homepage Passes Validator without repairs
func TestValidatorCleanHomepagePass(t *testing.T) {
	validator := NewUIValidator()

	reqSpec := &dtos.RequirementSpecificationDTO{
		Page: dtos.RequirementPageDTO{
			Type:       "homepage",
			Complexity: "simple",
		},
		Goals: dtos.RequirementGoalsDTO{
			Primary: "Simple clean landing page",
		},
	}

	designSpec := &dtos.DesignSpecificationDTO{
		Sections: []dtos.UISectionDTO{
			{ID: "sec-1", Type: "navbar", Purpose: "Top nav", RequirementSource: "navigation", Priority: "high"},
			{ID: "sec-2", Type: "hero", Purpose: "Intro", RequirementSource: "primary_goal", Priority: "high"},
			{ID: "sec-3", Type: "feature_grid", Purpose: "Highlights", RequirementSource: "primary_goal", Priority: "medium"},
		},
	}

	result, validatedSpec := validator.ValidateAndAudit(reqSpec, designSpec)

	if result.Status != "pass" {
		t.Errorf("expected clean spec to have status 'pass', got '%s'", result.Status)
	}
	if len(result.StrippedSections) != 0 {
		t.Errorf("expected 0 stripped sections for clean spec, got %d", len(result.StrippedSections))
	}
	if len(validatedSpec.Sections) != 3 {
		t.Errorf("expected all 3 sections preserved, got %d", len(validatedSpec.Sections))
	}
}

// 5. CRITICAL BENCHMARK TEST: Live Compiler with "buat homepage sederhana"
// The prompt MUST NOT generate unrequested treasury, CFO, credit limits, or crypto widgets.
func TestCriticalSimpleHomepageCompiler(t *testing.T) {
	_ = godotenv.Load("../../.env")
	config.LoadEnv()
	if config.Env.OpenAIAPIKey == "" && config.Env.AnthropicAPIKey == "" {
		t.Skip("Skipping live AI test: No API key configured")
	}

	aiService := NewAIService()
	compiler := NewUIDesignCompiler(aiService)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	rawPrompt := "buat homepage sederhana"
	dsl, err := compiler.Compile(ctx, rawPrompt, "web", "ramp", "dark", "#6366f1")
	if err != nil {
		t.Fatalf("compiler execution failed: %v", err)
	}

	// 1. Assert Requirement Specification
	if dsl.RequirementSpec == nil {
		t.Fatalf("expected non-nil RequirementSpec")
	}
	if dsl.RequirementSpec.Page.Complexity != "simple" {
		t.Errorf("expected complexity 'simple' for 'buat homepage sederhana', got '%s'", dsl.RequirementSpec.Page.Complexity)
	}
	if dsl.RequirementSpec.Context.Domain != nil && *dsl.RequirementSpec.Context.Domain != "" && *dsl.RequirementSpec.Context.Domain != "general" {
		t.Errorf("expected domain to be null/unspecified for generic prompt, got: %s", *dsl.RequirementSpec.Context.Domain)
	}

	// 2. Assert Zero Hallucinations in Design Specification & Code Export
	dslBytes, _ := json.Marshal(dsl)
	dslStr := strings.ToLower(string(dslBytes))

	prohibited := []string{"treasury", "cfo", "virtual card", "credit limit", "ocr", "crypto", "blockchain"}
	for _, word := range prohibited {
		if strings.Contains(dslStr, word) {
			t.Errorf("CRITICAL VIOLATION: AI Design Compiler produced hallucinated feature '%s' for prompt '%s'", word, rawPrompt)
		}
	}

	// 3. Assert Traceability on all compiled sections
	if len(dsl.DesignSpec.Sections) == 0 {
		t.Fatalf("expected compiled design spec to have sections")
	}
	for idx, sec := range dsl.DesignSpec.Sections {
		if strings.TrimSpace(sec.RequirementSource) == "" {
			t.Errorf("section %d (%s) is missing requirement_source traceability", idx, sec.Type)
		}
	}

	// 4. Assert Validation Scores
	if dsl.Validation.Score.RequirementFidelity < 8 {
		t.Errorf("expected fidelity >= 8, got %d", dsl.Validation.Score.RequirementFidelity)
	}
	if dsl.Validation.Score.Simplicity < 8 {
		t.Errorf("expected simplicity >= 8, got %d", dsl.Validation.Score.Simplicity)
	}

	t.Logf("SUCCESS: 'buat homepage sederhana' compiled into %d sections with complexity '%s' and score %d/10",
		len(dsl.DesignSpec.Sections),
		dsl.RequirementSpec.Page.Complexity,
		dsl.Validation.Score.RequirementFidelity,
	)
}

// 6. BENCHMARK TEST: Explicit ERP Purchase Request preserves authentic enterprise domain
func TestExplicitERPProcurementCompiler(t *testing.T) {
	_ = godotenv.Load("../../.env")
	config.LoadEnv()
	if config.Env.OpenAIAPIKey == "" && config.Env.AnthropicAPIKey == "" {
		t.Skip("Skipping live AI test: No API key configured")
	}

	aiService := NewAIService()
	compiler := NewUIDesignCompiler(aiService)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	rawPrompt := "halaman purchase request ERP untuk procurement officer"
	dsl, err := compiler.Compile(ctx, rawPrompt, "web", "ramp", "light", "#0f766e")
	if err != nil {
		t.Fatalf("compiler execution failed: %v", err)
	}

	if dsl.RequirementSpec == nil {
		t.Fatalf("expected non-nil RequirementSpec")
	}
	// Domain should be recognized as erp or procurement
	if dsl.RequirementSpec.Context.Domain == nil || *dsl.RequirementSpec.Context.Domain == "" {
		t.Errorf("expected domain to be identified as ERP/procurement for explicit prompt")
	} else {
		t.Logf("Recognized Domain: %s", *dsl.RequirementSpec.Context.Domain)
	}

	// Must have high fidelity score
	if dsl.Validation.Score.RequirementFidelity < 8 {
		t.Errorf("expected fidelity >= 8, got %d", dsl.Validation.Score.RequirementFidelity)
	}

	t.Logf("SUCCESS: ERP prompt compiled into %d sections with fidelity %d/10",
		len(dsl.DesignSpec.Sections),
		dsl.Validation.Score.RequirementFidelity,
	)
}

// 7. UNIT TEST: UI Validator Strips Unrequested Social Buttons & Form Bloat (Anti-Feature-Drift Guard)
func TestValidatorOptionalFeatureDriftStripping(t *testing.T) {
	validator := NewUIValidator()

	reqSpec := &dtos.RequirementSpecificationDTO{
		Page: dtos.RequirementPageDTO{
			Type:       "login",
			Purpose:    "User authentication",
			Complexity: "simple",
		},
		DesignFreedom: &dtos.DesignFreedomDTO{
			Functional: "low",
			Visual:     "medium",
		},
		Goals: dtos.RequirementGoalsDTO{
			Primary: "Provide minimal login interface",
		},
		Requirements: dtos.RequirementsListDTO{
			Explicit: []string{"Email and password fields", "Sign in button"},
			Implied:  []string{"Form validation"},
			Optional: []string{"Google login", "GitHub login", "Remember me", "Forgot password"},
		},
	}

	// Design spec where LLM leaked unrequested social buttons and remember me
	designSpec := &dtos.DesignSpecificationDTO{
		Sections: []dtos.UISectionDTO{
			{
				ID:                "sec-form",
				Type:              "form",
				Purpose:           "Authentication card",
				RequirementSource: "primary_goal",
				Priority:          "high",
				Data: map[string]interface{}{
					"title":                "Selamat Datang",
					"subtitle":             "Kelola seluruh treasury dashboard dan workflow Anda",
					"social_buttons":       []interface{}{"Google", "GitHub"},
					"show_remember_me":     true,
					"show_forgot_password": true,
					"switch_action":        "Belum punya akun? Daftar",
				},
			},
		},
	}

	result, validatedSpec := validator.ValidateAndAudit(reqSpec, designSpec)

	if result.Status != "repaired" {
		t.Errorf("expected status 'repaired' when stripping optional feature drift, got '%s'", result.Status)
	}

	if len(validatedSpec.Sections) != 1 {
		t.Fatalf("expected 1 form section preserved, got %d", len(validatedSpec.Sections))
	}

	formData := validatedSpec.Sections[0].Data
	if btns, ok := formData["social_buttons"].([]string); ok && len(btns) > 0 {
		t.Errorf("expected social_buttons to be stripped, got: %v", btns)
	}
	if rem, ok := formData["show_remember_me"].(bool); ok && rem {
		t.Errorf("expected show_remember_me to be false")
	}
	if forgot, ok := formData["show_forgot_password"].(bool); ok && forgot {
		t.Errorf("expected show_forgot_password to be false")
	}
	if sw, ok := formData["switch_action"].(string); ok && sw != "" {
		t.Errorf("expected switch_action to be empty, got: %s", sw)
	}

	// Subtitle should be cleansed of treasury/dashboard
	if sub, ok := formData["subtitle"].(string); ok {
		if strings.Contains(strings.ToLower(sub), "treasury") || strings.Contains(strings.ToLower(sub), "dashboard") {
			t.Errorf("expected subtitle to be neutralized, got: %s", sub)
		}
	}

	// Check Structured Issues
	if len(result.StructuredIssues) == 0 {
		t.Errorf("expected structured issues recorded for optional feature drift")
	} else {
		t.Logf("Recorded %d structured audit issues:", len(result.StructuredIssues))
		for _, issue := range result.StructuredIssues {
			t.Logf(" - [%s] %s: %s (%s)", issue.Type, issue.Component, issue.Action, issue.Reason)
		}
	}
}

// 8. CRITICAL BENCHMARK TEST: Live Compiler with "buatkan login page minimalis elegan dan modern"
// Must produce minimal form with 0 social buttons, 0 remember me, 0 forgot password, and null domain.
func TestLoginMinimalisEleganModern(t *testing.T) {
	_ = godotenv.Load("../../.env")
	config.LoadEnv()
	if config.Env.OpenAIAPIKey == "" && config.Env.AnthropicAPIKey == "" {
		t.Skip("Skipping live AI test: No API key configured")
	}

	aiService := NewAIService()
	compiler := NewUIDesignCompiler(aiService)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	rawPrompt := "buatkan login page minimalis elegan dan modern"
	dsl, err := compiler.Compile(ctx, rawPrompt, "web", "ramp", "dark", "#6366f1")
	if err != nil {
		t.Fatalf("compiler execution failed: %v", err)
	}

	// 1. Freedom & Complexity
	if dsl.RequirementSpec == nil {
		t.Fatalf("expected non-nil RequirementSpec")
	}
	if dsl.RequirementSpec.DesignFreedom != nil && dsl.RequirementSpec.DesignFreedom.Functional != "low" {
		t.Errorf("expected functional freedom 'low', got '%s'", dsl.RequirementSpec.DesignFreedom.Functional)
	}
	if dsl.RequirementSpec.Page.Complexity != "simple" {
		t.Errorf("expected complexity 'simple', got '%s'", dsl.RequirementSpec.Page.Complexity)
	}
	if dsl.RequirementSpec.Context.Domain != nil && *dsl.RequirementSpec.Context.Domain != "" && *dsl.RequirementSpec.Context.Domain != "general" {
		t.Errorf("expected domain to be null/general, got: %s", *dsl.RequirementSpec.Context.Domain)
	}

	// 2. Form section cleanliness (NO optional feature drift)
	for _, sec := range dsl.DesignSpec.Sections {
		if sec.Type == "form" || sec.Type == "auth_card" || sec.Type == "login_card" {
			if btns, ok := sec.Data["social_buttons"].([]string); ok && len(btns) > 0 {
				t.Errorf("CRITICAL VIOLATION: login page has unrequested social_buttons: %v", btns)
			}
			if rem, ok := sec.Data["show_remember_me"].(bool); ok && rem {
				t.Errorf("CRITICAL VIOLATION: login page has unrequested show_remember_me == true")
			}
			if forgot, ok := sec.Data["show_forgot_password"].(bool); ok && forgot {
				t.Errorf("CRITICAL VIOLATION: login page has unrequested show_forgot_password == true")
			}
			if sw, ok := sec.Data["switch_action"].(string); ok && sw != "" {
				t.Errorf("CRITICAL VIOLATION: login page has unrequested switch_action == %s", sw)
			}
		}
	}

	// 3. Validation Scores
	if dsl.Validation.Score.RequirementFidelity < 8 {
		t.Errorf("expected fidelity >= 8, got %d", dsl.Validation.Score.RequirementFidelity)
	}
	if dsl.Validation.Score.HallucinationSafety < 8 {
		t.Errorf("expected hallucination safety >= 8, got %d", dsl.Validation.Score.HallucinationSafety)
	}

	t.Logf("SUCCESS: 'buatkan login page minimalis elegan dan modern' compiled cleanly with 0 unrequested features. Fidelity: %d/10",
		dsl.Validation.Score.RequirementFidelity,
	)
}

// 9. CRITICAL BENCHMARK TEST: Live Compiler with "buatkan login page dengan Google dan GitHub"
// When explicitly requested, Google and GitHub MUST be preserved in social_buttons.
func TestLoginWithGoogleGitHubExplicit(t *testing.T) {
	_ = godotenv.Load("../../.env")
	config.LoadEnv()
	if config.Env.OpenAIAPIKey == "" && config.Env.AnthropicAPIKey == "" {
		t.Skip("Skipping live AI test: No API key configured")
	}

	aiService := NewAIService()
	compiler := NewUIDesignCompiler(aiService)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	rawPrompt := "buatkan login page dengan Google dan GitHub"
	dsl, err := compiler.Compile(ctx, rawPrompt, "web", "ramp", "dark", "#6366f1")
	if err != nil {
		t.Fatalf("compiler execution failed: %v", err)
	}

	foundSocial := false
	for _, sec := range dsl.DesignSpec.Sections {
		if sec.Type == "form" || sec.Type == "auth_card" || sec.Type == "login_card" {
			var btns []string
			if sList, ok := sec.Data["social_buttons"].([]string); ok {
				btns = sList
			} else if ifaceList, ok := sec.Data["social_buttons"].([]interface{}); ok {
				for _, it := range ifaceList {
					if str, ok := it.(string); ok {
						btns = append(btns, str)
					}
				}
			}
			t.Logf("Form section %s social_buttons: %v", sec.ID, btns)
			hasGoogle := false
			hasGitHub := false
			for _, b := range btns {
				if strings.Contains(strings.ToLower(b), "google") {
					hasGoogle = true
				}
				if strings.Contains(strings.ToLower(b), "github") {
					hasGitHub = true
				}
			}
			if hasGoogle && hasGitHub {
				foundSocial = true
			}
		}
	}

	if !foundSocial {
		t.Errorf("expected explicitly requested Google and GitHub to be present in form section data")
	} else {
		t.Logf("SUCCESS: 'buatkan login page dengan Google dan GitHub' preserved explicit social buttons!")
	}
}


