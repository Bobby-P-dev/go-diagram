package services

import (
	"context"
	"testing"

	"github.com/Bobby-P-dev/go-diagram.git/src/dtos"
)

func TestChangeAnalyzerDetectsInsertFrame(t *testing.T) {
	analyzer := &ChangeAnalyzer{}
	cases := []struct {
		name     string
		req      string
		pageType string
		device   string
	}{
		{"pricing-mobile", "tambah halaman pricing mobile", "pricing", "mobile"},
		{"pricing-web", "tambah halaman pricing", "pricing", "web"},
		{"register", "buat screen register", "register", "web"},
		{"register-mobile", "buat screen register mobile", "register", "mobile"},
		{"dashboard-desktop", "buat halaman dashboard desktop", "dashboard", "desktop"},
		{"landing", "tambah frame landing", "landing", "web"},
		// Natural Indonesian affixes (-kan, -nya) & word order variations
		{"user-reported-login-suffix", "tambahkan halaman loginnya yang minimalis tetapi modern", "login", "web"},
		{"tambahkan-halaman-loginnya", "tambahkan halaman loginnya", "login", "web"},
		{"tambahkan-halaman-login", "tambahkan halaman login", "login", "web"},
		{"buatkan-screen-register", "buatkan screen register", "register", "web"},
		{"tambah-login-page", "tambah login page", "login", "web"},
		{"buatkan-login-screen", "buatkan login screen", "login", "web"},
		{"bikin-page-checkout", "bikin page checkout", "checkout", "web"},
		{"halaman-baru-profil", "halaman baru untuk profil", "profile", "web"},
		{"tambah-1-halaman-lagi", "tambah 1 halaman lagi", "new_page", "web"},
		{"screen-baru-mobile", "screen baru untuk checkout mobile", "checkout", "mobile"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			plan, err := analyzer.AnalyzeTargetedChange(context.Background(), tc.req, nil, nil, nil)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if plan.Operation != dtos.OpInsertFrame {
				t.Fatalf("expected operation INSERT_FRAME, got %q (strategy=%s)", plan.Operation, plan.Strategy)
			}
			if plan.Strategy != "insert_frame" {
				t.Fatalf("expected strategy insert_frame, got %q", plan.Strategy)
			}
			if tc.pageType != "" && plan.Target.SectionID != tc.pageType {
				t.Fatalf("expected page type %q, got %q", tc.pageType, plan.Target.SectionID)
			}
			if plan.Target.Device != tc.device {
				t.Fatalf("expected device %q, got %q", tc.device, plan.Target.Device)
			}
			if plan.Scope != "project" || plan.PreserveOutsideTarget != true {
				t.Fatalf("expected scope=project and preserve_outside_target=true, got scope=%s preserve=%v", plan.Scope, plan.PreserveOutsideTarget)
			}
		})
	}
}

func TestChangeAnalyzerDetectsSectionOperations(t *testing.T) {
	analyzer := &ChangeAnalyzer{}
	cases := []struct {
		name      string
		req       string
		op        dtos.OperationType
		targetSec string
	}{
		{"buatkan-section", "buatkan section untuk testimoni", dtos.OpInsertSection, ""},
		{"tambah-section-setelah", "tambahkan section faq setelah hero", dtos.OpInsertSection, "hero"},
		{"buatkan-timeline-natural", "buatkan juga untuk menu timeline nya di sesuaikan dengan design yang sudah ada saat ini", dtos.OpInsertSection, ""},
		{"tambah-timeline", "tambahkan timeline", dtos.OpInsertSection, ""},
		{"buatkan-menu-timeline", "buatkan menu timeline", dtos.OpInsertSection, ""},
		{"hapus-section", "hapus section testimonial", dtos.OpDeleteSection, "sec-testimonial"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			plan, err := analyzer.AnalyzeTargetedChange(context.Background(), tc.req, nil, nil, nil)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if plan.Operation != tc.op {
				t.Fatalf("expected operation %s, got %s", tc.op, plan.Operation)
			}
			if tc.targetSec != "" && plan.Target.SectionID != tc.targetSec {
				t.Fatalf("expected target section %s, got %s", tc.targetSec, plan.Target.SectionID)
			}
		})
	}
}

func TestChangeAnalyzerDoesNotMisclassifyPatchesAsInsertFrame(t *testing.T) {
	analyzer := &ChangeAnalyzer{}
	patches := []string{
		"ubah judul section hero menjadi X",
		"buat tombol login lebih kecil",
		"tambah section pricing setelah hero",
		"ubah login page menjadi dashboard admin",
		"pindahkan form ke kanan",
		"tambahkan tombol login di halaman ini",
		"tambahkan input email ke form",
	}
	for _, req := range patches {
		plan, err := analyzer.AnalyzeTargetedChange(context.Background(), req, nil, nil, nil)
		if err != nil {
			t.Fatalf("unexpected error for %q: %v", req, err)
		}
		if plan.Operation == dtos.OpInsertFrame {
			t.Fatalf("patch %q must NOT be classified as INSERT_FRAME (got strategy=%s op=%s)", req, plan.Strategy, plan.Operation)
		}
	}
}

func TestChangeAnalyzerTimelineWithSelectedProductTarget(t *testing.T) {
	analyzer := &ChangeAnalyzer{}
	req := "buatkan juga untuk menu timeline nya di sesuaikan dengan design yang sudah ada saat ini"
	targetRef := &dtos.TargetElementRefDTO{
		Type: "section",
		ID:   "sec-products",
	}
	plan, err := analyzer.AnalyzeTargetedChange(context.Background(), req, targetRef, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if plan.Operation != dtos.OpInsertSection {
		t.Fatalf("expected OpInsertSection, got %s", plan.Operation)
	}
	if plan.Strategy != "insert_section" {
		t.Fatalf("expected strategy insert_section, got %s", plan.Strategy)
	}
	if plan.Target.SectionID != "sec-products" {
		t.Fatalf("expected target anchor sec-products, got %s", plan.Target.SectionID)
	}
}

func TestChangeAnalyzerExplicitNewScreenScope(t *testing.T) {
	analyzer := &ChangeAnalyzer{}
	req := "buatkan juga untuk menu timeline nya di sesuaikan dengan design yang sudah ada saat ini"
	selCtx := &dtos.SelectionContextDTO{
		Scope: "new_screen",
	}
	plan, err := analyzer.AnalyzeTargetedChange(context.Background(), req, nil, selCtx, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if plan.Operation != dtos.OpInsertFrame {
		t.Fatalf("expected OpInsertFrame when scope is new_screen, got %s", plan.Operation)
	}
	if plan.Strategy != "insert_frame" {
		t.Fatalf("expected strategy insert_frame, got %s", plan.Strategy)
	}
	if plan.Target.SectionID != "timeline" {
		t.Fatalf("expected page type timeline, got %s", plan.Target.SectionID)
	}
}

func TestChangeAnalyzerDeviceModeSwitch(t *testing.T) {
	analyzer := &ChangeAnalyzer{}
	cases := []struct {
		name        string
		req         string
		expectedDev string
	}{
		{
			name:        "user-reported-switch-to-web",
			req:         "buatkan di mode web jangan di mode mobile ubah ke mode web",
			expectedDev: "web",
		},
		{
			name:        "user-reported-switch-to-web-2",
			req:         "ubah dari mobile ke bentuk web",
			expectedDev: "web",
		},
		{
			name:        "ubah-ke-mode-web",
			req:         "ubah ke mode web",
			expectedDev: "web",
		},
		{
			name:        "ganti-ke-mode-mobile",
			req:         "ganti ke mode mobile",
			expectedDev: "mobile",
		},
		{
			name:        "versi-desktop",
			req:         "buatkan versi desktop",
			expectedDev: "desktop",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			plan, err := analyzer.AnalyzeTargetedChange(context.Background(), tc.req, nil, nil, nil)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if plan.Operation != dtos.OpDeviceModeSwitch {
				t.Fatalf("expected OpDeviceModeSwitch, got %s", plan.Operation)
			}
			if plan.Strategy != "device_switch" {
				t.Fatalf("expected strategy device_switch, got %s", plan.Strategy)
			}
			if plan.Target.Device != tc.expectedDev {
				t.Fatalf("expected target device %q, got %q", tc.expectedDev, plan.Target.Device)
			}
			if !plan.PreserveOutsideTarget {
				t.Fatalf("expected PreserveOutsideTarget=true")
			}
		})
	}
}

func TestChangeAnalyzerDeleteFrame(t *testing.T) {
	analyzer := &ChangeAnalyzer{}
	cases := []struct {
		name string
		req  string
	}{
		{
			name: "hapus-screen-ini",
			req:  "hapus screen ini",
		},
		{
			name: "hapus-halaman",
			req:  "tolong hapus halaman ini",
		},
		{
			name: "delete-screen",
			req:  "delete this screen",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			plan, err := analyzer.AnalyzeTargetedChange(context.Background(), tc.req, nil, nil, nil)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if plan.Operation != dtos.OpDeleteFrame {
				t.Fatalf("expected OpDeleteFrame, got %s", plan.Operation)
			}
			if plan.Strategy != "delete_frame" {
				t.Fatalf("expected strategy delete_frame, got %s", plan.Strategy)
			}
		})
	}
}