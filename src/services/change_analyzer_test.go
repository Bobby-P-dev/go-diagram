package services

import (
	"context"
	"testing"

	"github.com/Bobby-P-dev/go-diagram.git/src/dtos"
)

// TEST 6: "buat tombol menjadi lebih kecil" -> STYLE, PATCH, button target, component scope
func TestBenchmark6_ButtonSizeSmaller(t *testing.T) {
	analyzer := NewChangeAnalyzer(nil)
	ctx := context.Background()

	currentFrame := &dtos.UIFrameData{
		Device: "web",
		Title:  "Login Minimalis",
		Sections: []dtos.UISectionDTO{
			{
				ID:   "sec-auth-form",
				Type: "form",
				Data: map[string]interface{}{
					"title":       "Masuk",
					"submit_size": "standard",
				},
			},
		},
	}

	plan, err := analyzer.AnalyzeChange(ctx, "buat tombol login lebih kecil", currentFrame)
	if err != nil {
		t.Fatalf("AnalyzeChange error: %v", err)
	}

	if plan.Classification != "style" {
		t.Errorf("Expected classification 'style', got '%s'", plan.Classification)
	}
	if plan.Strategy != "patch" {
		t.Errorf("Expected strategy 'patch', got '%s'", plan.Strategy)
	}
	if plan.Regenerate != false {
		t.Errorf("Expected regenerate false, got true")
	}
	if plan.Target.Component != "button" {
		t.Errorf("Expected target component 'button', got '%s'", plan.Target.Component)
	}

	// Test PatchEngine execution
	engine := NewPatchEngine()
	patched, explanation, err := engine.ApplyPatch(currentFrame, plan)
	if err != nil {
		t.Fatalf("ApplyPatch error: %v", err)
	}

	formSec := patched.Sections[0]
	if formSec.Data["submit_size"] != "small" {
		t.Errorf("Expected submit_size 'small', got '%v'", formSec.Data["submit_size"])
	}
	if patched.ChangePlan == nil {
		t.Errorf("Expected ChangePlan attached to frame")
	}
	t.Logf("Test 6 passed. Explanation: %s", explanation)
}

// TEST 7: "ubah warna primary menjadi hitam" -> STYLE, PATCH, token update
func TestBenchmark7_ColorPrimaryBlack(t *testing.T) {
	analyzer := NewChangeAnalyzer(nil)
	ctx := context.Background()

	plan, err := analyzer.AnalyzeChange(ctx, "ubah warna primary menjadi hitam", nil)
	if err != nil {
		t.Fatalf("AnalyzeChange error: %v", err)
	}

	if plan.Classification != "style" {
		t.Errorf("Expected classification 'style', got '%s'", plan.Classification)
	}
	if plan.Strategy != "patch" {
		t.Errorf("Expected strategy 'patch', got '%s'", plan.Strategy)
	}
	if plan.Regenerate != false {
		t.Errorf("Expected regenerate false, got true")
	}

	// Test PatchEngine
	frame := &dtos.UIFrameData{
		Theme: map[string]interface{}{"mode": "dark", "primary": "#6366f1"},
	}
	engine := NewPatchEngine()
	patched, _, err := engine.ApplyPatch(frame, plan)
	if err != nil {
		t.Fatalf("ApplyPatch error: %v", err)
	}

	if patched.Theme["primary"] != "#000000" {
		t.Errorf("Expected primary color '#000000', got '%v'", patched.Theme["primary"])
	}
	t.Log("Test 7 passed.")
}

// TEST 8: "pindahkan form ke kanan" -> LAYOUT, target form
func TestBenchmark8_MoveFormRight(t *testing.T) {
	analyzer := NewChangeAnalyzer(nil)
	ctx := context.Background()

	plan, err := analyzer.AnalyzeChange(ctx, "form login pindahkan ke sebelah kanan", nil)
	if err != nil {
		t.Fatalf("AnalyzeChange error: %v", err)
	}

	if plan.Classification != "layout" {
		t.Errorf("Expected classification 'layout', got '%s'", plan.Classification)
	}
	if plan.Strategy != "patch" {
		t.Errorf("Expected strategy 'patch', got '%s'", plan.Strategy)
	}
	if plan.Target.Section != "form" {
		t.Errorf("Expected target section 'form', got '%s'", plan.Target.Section)
	}
	t.Log("Test 8 passed.")
}

// TEST 9: "ubah seluruh desain menjadi dark mode" -> GLOBAL_STYLE
func TestBenchmark9_GlobalDarkMode(t *testing.T) {
	analyzer := NewChangeAnalyzer(nil)
	ctx := context.Background()

	plan, err := analyzer.AnalyzeChange(ctx, "ubah seluruh desain menjadi dark mode", nil)
	if err != nil {
		t.Fatalf("AnalyzeChange error: %v", err)
	}

	if plan.Classification != "global_style" {
		t.Errorf("Expected classification 'global_style', got '%s'", plan.Classification)
	}
	if plan.Strategy != "patch" {
		t.Errorf("Expected strategy 'patch', got '%s'", plan.Strategy)
	}

	frame := &dtos.UIFrameData{
		Theme: map[string]interface{}{"mode": "light", "primary": "#6366f1"},
	}
	engine := NewPatchEngine()
	patched, _, err := engine.ApplyPatch(frame, plan)
	if err != nil {
		t.Fatalf("ApplyPatch error: %v", err)
	}
	if patched.Theme["mode"] != "dark" {
		t.Errorf("Expected theme mode 'dark', got '%v'", patched.Theme["mode"])
	}
	t.Log("Test 9 passed.")
}

// TEST 10: "ubah login page menjadi dashboard admin" -> STRUCTURAL / PAGE_REBUILD
func TestBenchmark10_LoginToDashboardRebuild(t *testing.T) {
	analyzer := NewChangeAnalyzer(nil)
	ctx := context.Background()

	plan, err := analyzer.AnalyzeChange(ctx, "ubah login page ini menjadi dashboard admin", nil)
	if err != nil {
		t.Fatalf("AnalyzeChange error: %v", err)
	}

	if plan.Classification != "structural" {
		t.Errorf("Expected classification 'structural', got '%s'", plan.Classification)
	}
	if plan.Strategy != "rebuild" {
		t.Errorf("Expected strategy 'rebuild', got '%s'", plan.Strategy)
	}
	if plan.Regenerate != true {
		t.Errorf("Expected regenerate true, got false")
	}
	t.Log("Test 10 passed.")
}

// TEST: Feature addition - Google login added explicitly
func TestBenchmark_AddGoogleLogin(t *testing.T) {
	analyzer := NewChangeAnalyzer(nil)
	ctx := context.Background()

	plan, err := analyzer.AnalyzeChange(ctx, "tambahkan login dengan Google", nil)
	if err != nil {
		t.Fatalf("AnalyzeChange error: %v", err)
	}

	if plan.Classification != "functionality" {
		t.Errorf("Expected classification 'functionality', got '%s'", plan.Classification)
	}
	if plan.Strategy != "patch" {
		t.Errorf("Expected strategy 'patch', got '%s'", plan.Strategy)
	}

	frame := &dtos.UIFrameData{
		Sections: []dtos.UISectionDTO{
			{
				Type: "form",
				Data: map[string]interface{}{
					"title": "Masuk",
				},
			},
		},
	}
	engine := NewPatchEngine()
	patched, _, err := engine.ApplyPatch(frame, plan)
	if err != nil {
		t.Fatalf("ApplyPatch error: %v", err)
	}
	socialBtns, ok := patched.Sections[0].Data["social_buttons"].([]string)
	if !ok || len(socialBtns) == 0 || socialBtns[0] != "Google" {
		t.Errorf("Expected social_buttons to contain Google, got %v", patched.Sections[0].Data["social_buttons"])
	}
	t.Log("Add Google login test passed.")
}
