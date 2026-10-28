package service_test

import (
	"encoding/json"
	"testing"

	dtos "github.com/Bobby-P-dev/go-diagram.git/internal/modules/uidesign/dto"
)

func fixtureStringPointer(value string) *string { return &value }

func bakeryBenchmarkPayload(t testing.TB) string {
	t.Helper()
	syn := dtos.UIDesignSynthesisDTO{
		Title: "Toko Roti & Kue Oma",
		Page: dtos.RequirementPageDTO{
			Type:       "bakery_catalog",
			Purpose:    "Katalog roti artisan dan kue tradisional",
			Complexity: "moderate",
		},
		Context: dtos.RequirementContextDTO{
			Domain:     fixtureStringPointer("artisanal bakery"),
			TargetUser: fixtureStringPointer("Pelanggan roti lokal"),
		},
		Goals: dtos.RequirementGoalsDTO{Primary: "Pesan roti panggang segar dan kue tradisional"},
		Requirements: dtos.RequirementsListDTO{
			Explicit: []string{"katalog roti", "daftar harga", "roti panggang segar"},
		},
		DesignFreedom: &dtos.DesignFreedomDTO{Functional: "low", Visual: "high", Composition: "high"},
		Layout:        dtos.DesignLayoutDTO{Type: "Warm Editorial Menu", Container: "max-w-md", Spacing: "generous"},
		Visual:        dtos.DesignVisualDTO{Style: "Warm parchment editorial", Density: "balanced", Theme: "light"},
		Typography:    dtos.DesignTypographyDTO{Heading: "Playfair Display", Body: "Plus Jakarta Sans", Metadata: "Plus Jakarta Sans"},
		Sections: []dtos.UISectionDTO{
			{
				ID:                "sec-hero",
				Type:              "bakery_hero",
				Purpose:           "Sambutan hangat dan jam panggang harian",
				Priority:          "high",
				RequirementSource: "primary_goal",
				Data:              map[string]interface{}{"title": "Roti Hangat dari Dapur Oma"},
			},
			{
				ID:                "sec-catalog",
				Type:              "product_menu",
				Purpose:           "Daftar roti signature dengan harga dan tombol pesan",
				Priority:          "high",
				RequirementSource: "explicit:katalog roti",
				Data:              map[string]interface{}{"title": "Pilihan Roti Pagi"},
			},
			{
				ID:                "sec-footer",
				Type:              "footer",
				Purpose:           "Alamat toko, jam operasional, dan hak cipta",
				Priority:          "low",
				RequirementSource: "composition:footer",
				Data:              map[string]interface{}{"title": "Informasi Toko"},
			},
		},
		RawHTML: `<style>
.bakery-font-display { font-family: 'Playfair Display', serif; }
</style>
<main class="min-h-screen bg-[#FAF8F5] text-stone-900 p-5 font-sans">
  <section data-rl-id="sec-hero" data-rl-kind="section" class="text-center py-6 border-b border-stone-200">
    <span class="text-xs uppercase tracking-widest text-amber-700 font-semibold">Dipanggang Segar Tiap Subuh</span>
    <h1 class="text-2xl font-bold bakery-font-display mt-2 text-stone-900">Roti Hangat dari Dapur Oma</h1>
    <p class="text-xs text-stone-600 mt-1 max-w-xs mx-auto">Resep warisan keluarga sejak 1968 tanpa bahan pengawet.</p>
  </section>

  <section data-rl-id="sec-catalog" data-rl-kind="section" class="py-6 space-y-4">
    <h2 class="text-sm font-bold uppercase tracking-wider text-stone-700">Menu Hari Ini</h2>
    <div data-rl-id="cmp-item-sourdough" data-rl-kind="component" class="flex items-center justify-between p-3.5 bg-white rounded-xl border border-stone-200 shadow-sm">
      <div>
        <h3 class="font-semibold text-sm">Sourdough Gandum Utuh</h3>
        <p class="text-xs text-stone-500">Fermentasi alami 24 jam</p>
        <span class="text-xs font-bold text-amber-800 mt-1 inline-block">Rp 45.000</span>
      </div>
      <button data-rl-id="cmp-btn-pesan-1" data-rl-kind="component" class="px-3 py-1.5 bg-amber-700 hover:bg-amber-800 text-white rounded-lg text-xs font-medium">Pesan</button>
    </div>
    <div data-rl-id="cmp-item-croissant" data-rl-kind="component" class="flex items-center justify-between p-3.5 bg-white rounded-xl border border-stone-200 shadow-sm">
      <div>
        <h3 class="font-semibold text-sm">Croissant Mentega Asli</h3>
        <p class="text-xs text-stone-500">Mentega Normandia berlapis renyah</p>
        <span class="text-xs font-bold text-amber-800 mt-1 inline-block">Rp 28.000</span>
      </div>
      <button data-rl-id="cmp-btn-pesan-2" data-rl-kind="component" class="px-3 py-1.5 bg-amber-700 hover:bg-amber-800 text-white rounded-lg text-xs font-medium">Pesan</button>
    </div>
  </section>

  <footer id="sec-footer" data-rl-id="sec-footer" data-rl-kind="section" class="pt-6 border-t border-stone-200 text-center text-xs text-stone-500">
    <p>Jl. Senopati No. 14, Jakarta Selatan</p>
    <p class="mt-1">Buka Selasa - Minggu: 07.00 - 18.00 WIB</p>
    <p class="mt-4 text-[11px] text-stone-400">&copy; 2026 Toko Roti Oma. Semua hak cipta dilindungi.</p>
  </footer>
</main>`,
		Theme: map[string]interface{}{
			"mode":        "light",
			"primary":     "#B45309",
			"accent":      "#D97706",
			"background":  "#FAF8F5",
			"foreground":  "#1C1917",
			"fontFamily":  "Plus Jakarta Sans",
			"headingFont": "Playfair Display",
		},
	}
	payload, err := json.Marshal(syn)
	if err != nil {
		t.Fatal(err)
	}
	return string(payload)
}

func telemetryBenchmarkPayload(t testing.TB) string {
	t.Helper()
	syn := dtos.UIDesignSynthesisDTO{
		Title: "CloudOps Telemetry Dashboard",
		Page: dtos.RequirementPageDTO{
			Type:       "dashboard",
			Purpose:    "Monitor cluster latency and pod health",
			Complexity: "complex",
		},
		Context: dtos.RequirementContextDTO{
			Domain:     fixtureStringPointer("cloud infrastructure"),
			TargetUser: fixtureStringPointer("SRE Engineers"),
		},
		Goals: dtos.RequirementGoalsDTO{Primary: "Real-time cluster telemetry and incident triage"},
		Requirements: dtos.RequirementsListDTO{
			Explicit: []string{"cluster telemetry", "latency sparkline", "pod health", "event log"},
		},
		DesignFreedom: &dtos.DesignFreedomDTO{Functional: "low", Visual: "high", Composition: "high"},
		Layout:        dtos.DesignLayoutDTO{Type: "Asymmetric Bento Grid", Container: "max-w-7xl", Spacing: "compact"},
		Visual:        dtos.DesignVisualDTO{Style: "Obsidian telemetry workspace", Density: "compact", Theme: "dark"},
		Typography:    dtos.DesignTypographyDTO{Heading: "Plus Jakarta Sans", Body: "Inter", Metadata: "JetBrains Mono"},
		Sections: []dtos.UISectionDTO{
			{
				ID:                "sec-nav",
				Type:              "navbar",
				Purpose:           "Cluster region switch and deploy action",
				Priority:          "high",
				RequirementSource: "explicit:cluster telemetry",
				Data:              map[string]interface{}{"title": "CloudOps Telemetry"},
			},
			{
				ID:                "sec-bento",
				Type:              "bento_grid",
				Purpose:           "Asymmetric 8+4 metric widgets with live sparkline and pod meters",
				Priority:          "high",
				RequirementSource: "primary_goal",
				Data:              map[string]interface{}{"title": "Cluster Metrics"},
			},
			{
				ID:                "sec-log",
				Type:              "incident_log",
				Purpose:           "Recent cluster incidents log",
				Priority:          "medium",
				RequirementSource: "explicit:event log",
				Data:              map[string]interface{}{"title": "Live Stream"},
			},
		},
		RawHTML: `<main class="min-h-screen bg-[#090A0F] text-slate-100 p-6 font-sans">
  <header data-rl-id="sec-nav" data-rl-kind="section" class="flex items-center justify-between border-b border-white/[0.08] pb-4 mb-6">
    <div class="flex items-center gap-3">
      <div class="h-8 w-8 rounded-lg bg-indigo-600/20 border border-indigo-500/30 flex items-center justify-center font-bold text-indigo-400 font-mono text-xs">CO</div>
      <span class="font-semibold text-sm tracking-tight text-white">CloudOps Telemetry</span>
      <span class="text-[11px] px-2 py-0.5 rounded-full bg-emerald-500/10 text-emerald-400 border border-emerald-500/20 font-mono">cluster-prod-01</span>
    </div>
    <div class="flex items-center gap-2">
      <kbd class="px-2 py-1 text-xs font-mono bg-white/[0.05] border border-white/[0.1] rounded text-slate-400">Ctrl+K</kbd>
      <button data-rl-id="cmp-deploy" data-rl-kind="component" class="px-3 py-1.5 rounded-lg bg-indigo-600 text-xs font-medium text-white hover:bg-indigo-500 transition-colors">Deploy</button>
    </div>
  </header>

  <section data-rl-id="sec-bento" data-rl-kind="section" class="grid grid-cols-1 md:grid-cols-12 gap-4 mb-6">
    <div data-rl-id="cmp-latency" data-rl-kind="component" class="md:col-span-8 rounded-xl bg-[#11131C] border border-white/[0.08] p-5">
      <div class="flex items-center justify-between mb-4">
        <div>
          <p class="text-xs font-mono uppercase text-slate-400">P99 Gateway Latency</p>
          <p class="text-3xl font-semibold tracking-tight font-mono text-white mt-1">38.4<span class="text-sm font-normal text-slate-400">ms</span></p>
        </div>
        <span class="text-xs text-emerald-400 font-mono">OK Operational</span>
      </div>
      <div class="h-24 w-full">
        <svg class="w-full h-full text-indigo-500" viewBox="0 0 300 70" fill="none" preserveAspectRatio="none">
          <path d="M0,50 Q60,20 120,40 T240,25 T300,10 L300,70 L0,70 Z" fill="rgba(99,102,241,0.15)"/>
          <path d="M0,50 Q60,20 120,40 T240,25 T300,10" stroke="#6366F1" stroke-width="2"/>
        </svg>
      </div>
    </div>

    <div data-rl-id="cmp-pods" data-rl-kind="component" class="md:col-span-4 rounded-xl bg-[#11131C] border border-white/[0.08] p-5">
      <p class="text-xs font-mono uppercase text-slate-400 mb-3">Healthy Pods</p>
      <div class="space-y-2.5 font-mono text-xs">
        <div class="flex justify-between"><span class="text-slate-300">ingress-nginx</span><span class="text-emerald-400">24/24</span></div>
        <div class="flex justify-between"><span class="text-slate-300">api-server</span><span class="text-emerald-400">18/18</span></div>
        <div class="flex justify-between"><span class="text-slate-300">worker-queue</span><span class="text-indigo-400">12/12</span></div>
      </div>
    </div>
  </section>

  <section data-rl-id="sec-log" data-rl-kind="section" class="rounded-xl bg-[#11131C] border border-white/[0.08] p-5">
    <h3 class="text-xs font-semibold uppercase font-mono text-slate-400 mb-3">Recent Deploy Log</h3>
    <div class="font-mono text-xs space-y-2 text-slate-300">
      <div class="flex items-center gap-3">
        <span class="text-slate-500">10:14:02</span>
        <span class="px-1.5 py-0.5 rounded bg-emerald-500/10 text-emerald-400 border border-emerald-500/20 text-[10px]">SUCCESS</span>
        <span>Canary traffic shifted to v2.4.1 (100% rollout)</span>
      </div>
    </div>
  </section>
</main>`,
		Theme: map[string]interface{}{
			"mode":        "dark",
			"primary":     "#6366F1",
			"accent":      "#10B981",
			"background":  "#090A0F",
			"foreground":  "#F8FAFC",
			"fontFamily":  "Inter",
			"headingFont": "Plus Jakarta Sans",
			"monoFont":    "JetBrains Mono",
		},
	}
	payload, err := json.Marshal(syn)
	if err != nil {
		t.Fatal(err)
	}
	return string(payload)
}

func authBenchmarkPayload(t testing.TB) string {
	t.Helper()
	syn := dtos.UIDesignSynthesisDTO{
		Title: "Masuk ke Akun",
		Page: dtos.RequirementPageDTO{
			Type:       "login",
			Purpose:    "Autentikasi akun pengguna",
			Complexity: "simple",
		},
		Context: dtos.RequirementContextDTO{
			Domain:     nil, // generic brief
			TargetUser: nil,
		},
		Goals: dtos.RequirementGoalsDTO{Primary: "Masuk dengan email dan kata sandi"},
		Requirements: dtos.RequirementsListDTO{
			Explicit: []string{"halaman login minimalis"},
		},
		DesignFreedom: &dtos.DesignFreedomDTO{Functional: "low", Visual: "high", Composition: "high"},
		Layout:        dtos.DesignLayoutDTO{Type: "Centered Auth Card", Container: "max-w-md", Spacing: "compact"},
		Visual:        dtos.DesignVisualDTO{Style: "Clean high-contrast minimalism", Density: "compact", Theme: "light"},
		Typography:    dtos.DesignTypographyDTO{Heading: "Plus Jakarta Sans", Body: "Plus Jakarta Sans", Metadata: "Inter"},
		Sections: []dtos.UISectionDTO{
			{
				ID:                "sec-login",
				Type:              "form",
				Purpose:           "Formulir login utama",
				Priority:          "high",
				RequirementSource: "primary_goal",
				Data: map[string]interface{}{
					"title":        "Masuk ke Portal",
					"subtitle":     "Masukkan kredensial Anda untuk melanjutkan.",
					"submit_label": "Masuk Sekarang",
				},
			},
		},
		RawHTML: `<main class="min-h-screen bg-slate-50 flex items-center justify-center p-4 font-sans">
  <section data-rl-id="sec-login" data-rl-kind="section" class="w-full max-w-sm bg-white p-8 rounded-2xl border border-slate-200/80 shadow-sm">
    <div class="mb-6 text-center">
      <h1 class="text-xl font-bold tracking-tight text-slate-900">Masuk ke Portal</h1>
      <p class="text-xs text-slate-500 mt-1">Masukkan kredensial Anda untuk melanjutkan.</p>
    </div>
    <form class="space-y-4">
      <div>
        <label for="login-email" class="block text-xs font-medium text-slate-700 mb-1">Alamat Email</label>
        <input id="login-email" type="email" placeholder="nama@perusahaan.com" class="w-full px-3 py-2 text-xs rounded-lg border border-slate-200 focus:outline-none focus:ring-2 focus:ring-slate-900" />
      </div>
      <div>
        <div class="flex justify-between items-center mb-1">
          <label for="login-password" class="block text-xs font-medium text-slate-700">Kata Sandi</label>
          <a href="#forgot" class="text-[11px] text-slate-500 hover:text-slate-900">Lupa sandi?</a>
        </div>
        <input id="login-password" type="password" placeholder="********" class="w-full px-3 py-2 text-xs rounded-lg border border-slate-200 focus:outline-none focus:ring-2 focus:ring-slate-900" />
      </div>
      <button data-rl-id="cmp-login-btn" data-rl-kind="component" type="button" class="w-full py-2.5 bg-slate-900 hover:bg-slate-800 text-white rounded-lg text-xs font-semibold transition-colors mt-2">Masuk Sekarang</button>
    </form>
  </section>
</main>`,
		Theme: map[string]interface{}{
			"mode":        "light",
			"primary":     "#0F172A",
			"accent":      "#64748B",
			"background":  "#F8FAFC",
			"foreground":  "#0F172A",
			"fontFamily":  "Plus Jakarta Sans",
			"headingFont": "Plus Jakarta Sans",
		},
	}
	payload, err := json.Marshal(syn)
	if err != nil {
		t.Fatal(err)
	}
	return string(payload)
}

func architectureBenchmarkPayload(t testing.TB) string {
	t.Helper()
	syn := dtos.UIDesignSynthesisDTO{
		Title: "Atelier Studio Arsitektur",
		Page: dtos.RequirementPageDTO{
			Type:       "portfolio",
			Purpose:    "Galeri portofolio arsitektur dan interior kontemporer",
			Complexity: "moderate",
		},
		Context: dtos.RequirementContextDTO{
			Domain:     fixtureStringPointer("architecture & spatial design"),
			TargetUser: fixtureStringPointer("Klien hunian privat & komersial"),
		},
		Goals: dtos.RequirementGoalsDTO{Primary: "Eksplorasi karya arsitektur dan konsultasi desain"},
		Requirements: dtos.RequirementsListDTO{
			Explicit: []string{"studio arsitektur", "galeri proyek", "kontak konsultasi"},
		},
		DesignFreedom: &dtos.DesignFreedomDTO{Functional: "low", Visual: "high", Composition: "high"},
		Layout:        dtos.DesignLayoutDTO{Type: "Brutalist Swiss Grid", Container: "max-w-7xl", Spacing: "dynamic rhythm"},
		Visual:        dtos.DesignVisualDTO{Style: "Monochrome architectural gallery", Density: "generous", Theme: "dark"},
		Typography:    dtos.DesignTypographyDTO{Heading: "Space Grotesk", Body: "DM Sans", Metadata: "JetBrains Mono"},
		Sections: []dtos.UISectionDTO{
			{
				ID:                "sec-nav",
				Type:              "header",
				Purpose:           "Brand studio dan navigasi galeri",
				Priority:          "high",
				RequirementSource: "primary_goal",
				Data:              map[string]interface{}{"title": "Atelier Arsitektur"},
			},
			{
				ID:                "sec-projects",
				Type:              "project_gallery",
				Purpose:           "Asymmetric showcase karya arsitektur terpilih",
				Priority:          "high",
				RequirementSource: "explicit:galeri proyek",
				Data:              map[string]interface{}{"title": "Karya Pilihan 2024-2026"},
			},
			{
				ID:                "sec-footer",
				Type:              "footer",
				Purpose:           "Informasi kontak studio dan alamat kantor",
				Priority:          "medium",
				RequirementSource: "explicit:kontak konsultasi",
				Data:              map[string]interface{}{"title": "Kontak Studio"},
			},
		},
		RawHTML: `<main class="min-h-screen bg-[#0C0D0E] text-neutral-100 p-8 font-sans">
  <header data-rl-id="sec-nav" data-rl-kind="section" class="flex justify-between items-center border-b border-neutral-800 pb-6 mb-12">
    <span class="text-sm font-mono tracking-widest uppercase">Atelier Studio</span>
    <nav class="flex gap-6 text-xs text-neutral-400">
      <a href="#proyek" class="hover:text-white transition-colors">Proyek</a>
      <a href="#filosofi" class="hover:text-white transition-colors">Filosofi</a>
      <a href="#sec-footer" class="hover:text-white transition-colors">Kontak</a>
    </nav>
  </header>

  <section data-rl-id="sec-projects" data-rl-kind="section" class="mb-16">
    <div class="grid grid-cols-1 md:grid-cols-12 gap-8">
      <div data-rl-id="cmp-proj-1" data-rl-kind="component" class="md:col-span-7 border-t border-neutral-800 pt-4">
        <span class="text-xs font-mono text-neutral-500">01 / Hunian Privat</span>
        <h2 class="text-2xl font-light tracking-tight mt-2 text-white">Villa Batu Karang - Uluwatu</h2>
        <p class="text-xs text-neutral-400 mt-2 leading-relaxed">Struktur kantilever beton ekspos dengan orientasi angin laut Samudra Hindia.</p>
      </div>
      <div data-rl-id="cmp-proj-2" data-rl-kind="component" class="md:col-span-5 border-t border-neutral-800 pt-4">
        <span class="text-xs font-mono text-neutral-500">02 / Ruang Komersial</span>
        <h2 class="text-2xl font-light tracking-tight mt-2 text-white">Paviliun Tropis - Bandung</h2>
        <p class="text-xs text-neutral-400 mt-2 leading-relaxed">Perpaduan bambu laminasi dan pencahayaan alami atap skylight.</p>
      </div>
    </div>
  </section>

  <footer id="sec-footer" data-rl-id="sec-footer" data-rl-kind="section" class="border-t border-neutral-800 pt-8 flex justify-between text-xs text-neutral-500 font-mono">
    <span>Studio Bandung & Denpasar</span>
    <span>atelier@studiouidesign.id</span>
  </footer>
</main>`,
		Theme: map[string]interface{}{
			"mode":        "dark",
			"primary":     "#E5E5E5",
			"accent":      "#737373",
			"background":  "#0C0D0E",
			"foreground":  "#FAFAFA",
			"fontFamily":  "DM Sans",
			"headingFont": "Space Grotesk",
			"monoFont":    "JetBrains Mono",
		},
	}
	payload, err := json.Marshal(syn)
	if err != nil {
		t.Fatal(err)
	}
	return string(payload)
}
