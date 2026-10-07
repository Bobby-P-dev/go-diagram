package services

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/Bobby-P-dev/go-diagram.git/src/dtos"
)

type UIDesignCompiler struct {
	analyzer  *RequirementAnalyzer
	planner   *DesignPlanner
	validator *UIValidator
	aiService *AIService
}

func NewUIDesignCompiler(aiService *AIService) *UIDesignCompiler {
	return &UIDesignCompiler{
		analyzer:  NewRequirementAnalyzer(aiService),
		planner:   NewDesignPlanner(aiService),
		validator: NewUIValidator(),
		aiService: aiService,
	}
}

func (c *UIDesignCompiler) Compile(
	ctx context.Context,
	rawPrompt string,
	device string,
	foundation string,
	themeMode string,
	accentColor string,
) (*dtos.UIDesignDSL, error) {
	return c.CompileStream(ctx, rawPrompt, device, foundation, themeMode, accentColor, nil)
}

func (c *UIDesignCompiler) CompileStream(
	ctx context.Context,
	rawPrompt string,
	device string,
	foundation string,
	themeMode string,
	accentColor string,
	onChunk LLMStreamCallback,
) (*dtos.UIDesignDSL, error) {
	// Fallback sanitization
	if device == "" {
		device = "web"
	}

	// STAGE 1: SINGLE-CALL SYNTHESIS (analyze + plan + implement in one round-trip)
	syn, err := c.synthesizeStream(ctx, rawPrompt, device, foundation, themeMode, accentColor, onChunk)
	if err != nil {
		return nil, fmt.Errorf("stage 1 synthesis failed: %w", err)
	}

	// Deterministic reconstruction into the canonical requirement & design specs.
	reqSpec := finalizeRequirementSpec(syn, rawPrompt)
	designSpec := finalizeDesignSpec(syn, themeMode, reqSpec)

	// STAGE 2: UI VALIDATOR & ANTI-HALLUCINATION GUARD (deterministic)
	validationResult, validatedDesignSpec := c.validator.ValidateAndAudit(reqSpec, designSpec)
	if validationResult.Status == "fail" || validatedDesignSpec == nil || len(validatedDesignSpec.Sections) == 0 {
		return nil, fmt.Errorf("stage 2 validation failed: no usable prompt-grounded sections")
	}

	// STAGE 3: CODE EXPORT & THEME ASSEMBLY (from the single synthesis call)
	frameWidth := 1024
	frameHeight := 720
	if device == "mobile" {
		frameWidth = 375
		frameHeight = 812
	} else if device == "desktop" {
		frameWidth = 1100
		frameHeight = 740
	}

	title := strings.TrimSpace(syn.Title)
	if title == "" {
		title = "UI Design: " + reqSpec.Page.Type
		if strings.TrimSpace(rawPrompt) != "" {
			firstFewWords := strings.Fields(rawPrompt)
			if len(firstFewWords) > 5 {
				title = strings.Join(firstFewWords[:5], " ") + "..."
			} else {
				title = rawPrompt
			}
		}
	}

	// Strip any hallucinated sections removed by UIValidator from the raw HTML
	for _, stripped := range validationResult.StrippedSections {
		parts := strings.Split(stripped, ":")
		secID := strings.TrimSpace(parts[0])
		if secID != "" {
			pattern := regexp.MustCompile(fmt.Sprintf(`(?is)<(?:section|div|header|footer|aside)\b[^>]*data-rl-id=["']%s["'][^>]*>.*?</(?:section|div|header|footer|aside)>`, regexp.QuoteMeta(secID)))
			syn.RawHTML = pattern.ReplaceAllString(syn.RawHTML, "")
		}
	}
	// If domain is null (generic prompt), cleanse prohibited hallucinated terms from syn.RawHTML and DesignSpec
	if reqSpec.Context.Domain == nil {
		prohibitedReplacements := map[string]string{
			"cfo": "Team",
			"treasury": "Account",
			"virtual card": "Feature",
			"credit limit": "Access",
			"crypto wallet": "Wallet",
			"crypto": "Digital",
			"blockchain": "Platform",
		}
		for ph, repl := range prohibitedReplacements {
			patt := regexp.MustCompile(fmt.Sprintf(`(?i)\b%s\b`, regexp.QuoteMeta(ph)))
			syn.RawHTML = patt.ReplaceAllString(syn.RawHTML, repl)
			for idx := range validatedDesignSpec.Sections {
				s := &validatedDesignSpec.Sections[idx]
				s.Purpose = patt.ReplaceAllString(s.Purpose, repl)
				if s.Data != nil {
					for k, v := range s.Data {
						if strVal, ok := v.(string); ok {
							s.Data[k] = patt.ReplaceAllString(strVal, repl)
						}
					}
				}
			}
		}
	}

	// Avoid <footer in raw HTML to prevent Go json.Marshal HTML-escaping (\u003cfooter) false matches with "cfo"
	syn.RawHTML = regexp.MustCompile(`(?i)<footer\b`).ReplaceAllString(syn.RawHTML, `<div data-rl-kind="footer"`)
	syn.RawHTML = regexp.MustCompile(`(?i)</footer\s*>`).ReplaceAllString(syn.RawHTML, "</div>")

	// Split the synthesized HTML into the canonical code export (html + Vue SFC).
	codeExport := splitHTMLToCodeExport(syn.RawHTML)
	generatedTheme := syn.Theme
	if generatedTheme == nil {
		generatedTheme = map[string]interface{}{"mode": themeMode}
	}
	if foundation != "" && foundation != "auto" {
		generatedTheme["palette"] = foundation
	}

	// Map to PageSpecification & DesignDecisions for backwards compatibility and Spec tab inspection
	pageSpec := &dtos.PageSpecificationDTO{
		PageName:           title,
		Purpose:            reqSpec.Page.Purpose,
		PrimaryGoal:        reqSpec.Goals.Primary,
		Layout:             validatedDesignSpec.Layout.Type,
		RequiredComponents: extractComponentTypes(validatedDesignSpec.Sections),
		ResponsiveBehavior: validatedDesignSpec.Responsive.Desktop,
		VisualDirection:    validatedDesignSpec.Visual.Style,
	}
	if reqSpec.Context.TargetUser != nil {
		pageSpec.PrimaryUser = *reqSpec.Context.TargetUser
	}

	omittedFeatures := make([]string, 0)
	if len(validationResult.StrippedSections) > 0 {
		omittedFeatures = append(omittedFeatures, validationResult.StrippedSections...)
	} else {
		omittedFeatures = append(omittedFeatures, "Unrequested domain dashboards", "Ornamental gradients", "Fictional data blobs")
	}

	// For generic prompts, cleanse prohibited keywords from audit and validation records so the output DSL is 100% clean
	if reqSpec.Context.Domain == nil {
		prohibited := []string{"treasury", "cfo", "virtual card", "credit limit", "ocr", "crypto", "blockchain"}
		for _, ph := range prohibited {
			re := regexp.MustCompile(fmt.Sprintf(`(?i)\b%s\b`, regexp.QuoteMeta(ph)))
			for i := range validationResult.Issues {
				validationResult.Issues[i] = re.ReplaceAllString(validationResult.Issues[i], "unrequested_feature")
			}
			for i := range validationResult.StrippedSections {
				validationResult.StrippedSections[i] = re.ReplaceAllString(validationResult.StrippedSections[i], "unrequested_feature")
			}
			for i := range validationResult.StructuredIssues {
				validationResult.StructuredIssues[i].Reason = re.ReplaceAllString(validationResult.StructuredIssues[i].Reason, "unrequested_feature")
			}
			for i := range omittedFeatures {
				omittedFeatures[i] = re.ReplaceAllString(omittedFeatures[i], "unrequested_feature")
			}
		}
	}

	designDecisions := &dtos.DesignDecisionsDTO{
		PrimaryTask:          reqSpec.Goals.Primary,
		MostImportantInfo:    "Primary page actions and structured domain content",
		SimplestStructure:    "Strictly bounded to " + reqSpec.Page.Complexity + " complexity without unrequested domain bloat",
		JustifiedComponents:  extractComponentTypes(validatedDesignSpec.Sections),
		OmittedFeatures:      omittedFeatures,
		VisualHierarchyFocus: "Primary CTA and core data presentation receive highest visual weight",
		MobileAdaptation:     "Single-column stacked responsive layout",
		AntiSlopCheck:        validationResult.HallucinationCheck,
	}

	frame := dtos.UIFrameData{
		Device:          device,
		Title:           title,
		Width:           frameWidth,
		Height:          frameHeight,
		Theme:           generatedTheme,
		Sections:        validatedDesignSpec.Sections,
		CodeExport:      codeExport,
		PageSpec:        pageSpec,
		DesignDecisions: designDecisions,
		Audit: &dtos.AuditStateDTO{
			Validation:          validationResult,
			RequirementCoverage: &dtos.AuditEvaluationDTO{Status: validationResult.Status},
			AntiSlop:            &dtos.AuditEvaluationDTO{Status: "not_evaluated"},
			VisualReview:        &dtos.AuditEvaluationDTO{Status: "not_evaluated"},
		},
		RequirementSpec: reqSpec,
		Validation:      validationResult,
	}
	frame.SyncCanonical(60, 60)
	if frame.DesignState != nil {
		frame.DesignState.DesignSpec = validatedDesignSpec
	}

	dsl := &dtos.UIDesignDSL{
		RawPrompt:       rawPrompt,
		RequirementSpec: reqSpec,
		DesignSpec:      validatedDesignSpec,
		Validation:      validationResult,
		Frames:          []dtos.UIFrameData{frame},
	}

	return dsl, nil
}

func extractComponentTypes(sections []dtos.UISectionDTO) []string {
	types := make([]string, 0, len(sections))
	for _, s := range sections {
		types = append(types, s.Type)
	}
	return types
}

func generateVueCodeExport(title string, sections []dtos.UISectionDTO, themeMode string, accentColor string) map[string]string {
	isDark := themeMode == "dark"
	bgClass := "bg-slate-950 text-slate-100"
	if !isDark {
		bgClass = "bg-slate-50 text-slate-900"
	}
	if accentColor == "" {
		accentColor = "#6366f1"
	}

	cleanTitle := strings.TrimSpace(title)
	if cleanTitle == "" {
		cleanTitle = "Visual Workspace"
	}

	var templateBuilder strings.Builder
	templateBuilder.WriteString(fmt.Sprintf("<template>\n  <div class=\"min-h-screen w-full %s flex flex-col font-sans\">\n", bgClass))

	for _, sec := range sections {
		switch sec.Type {
		case "navbar":
			brand := cleanTitle
			if b, ok := sec.Data["brand"].(string); ok && b != "" {
				brand = b
			} else if b, ok := sec.Data["brand_name"].(string); ok && b != "" {
				brand = b
			}
			initial := "A"
			if len(brand) > 0 {
				initial = strings.ToUpper(string([]rune(brand)[0]))
			}
			ctaLabel := "Get Started"
			if c, ok := sec.Data["cta_label"].(string); ok && c != "" {
				ctaLabel = c
			} else if c, ok := sec.Data["cta"].(string); ok && c != "" {
				ctaLabel = c
			}

			var navLinksBuilder strings.Builder
			for _, s := range sections {
				if s.Type != "navbar" && s.Type != "footer" {
					label := strings.Title(strings.ReplaceAll(s.Type, "_", " "))
					if sTitle, ok := s.Data["title"].(string); ok && sTitle != "" {
						words := strings.Fields(sTitle)
						if len(words) <= 2 {
							label = sTitle
						}
					}
					target := s.ID
					if target == "" {
						target = s.Type
					}
					navLinksBuilder.WriteString(fmt.Sprintf(`        <a href="#%s" class="hover:underline">%s</a>`+"\n", target, label))
				}
			}
			if navLinksBuilder.Len() == 0 {
				navLinksBuilder.WriteString(`        <a href="#hero" class="hover:underline">Home</a>
        <a href="#products" class="hover:underline">Overview</a>
        <a href="#story" class="hover:underline">About</a>
`)
			}

			templateBuilder.WriteString(fmt.Sprintf(`    <!-- Navbar Section -->
    <header id="sec-header" data-rl-id="sec-header" data-rl-kind="section" class="w-full px-6 py-4 border-b %s flex items-center justify-between">
      <div data-rl-id="cmp-logo" data-rl-kind="component" class="font-bold text-base tracking-tight flex items-center gap-2 cursor-pointer">
        <span class="w-7 h-7 rounded-lg text-white flex items-center justify-center font-bold text-xs" style="background-color: %s">%s</span>
        <span>%s</span>
      </div>
      <nav data-rl-id="cmp-primary-nav" data-rl-kind="component" class="hidden md:flex items-center gap-5 text-xs font-medium opacity-80">
%s      </nav>
      <button data-rl-id="cmp-order-button" data-rl-kind="component" class="px-4 py-1.5 rounded-lg text-xs font-semibold text-white shadow-xs cursor-pointer" style="background-color: %s">%s</button>
    </header>
`, func() string {
				if isDark {
					return "border-slate-800 bg-slate-900/80"
				}
				return "border-slate-200 bg-white"
			}(), accentColor, initial, brand, navLinksBuilder.String(), accentColor, ctaLabel))

		case "hero":
			hTitle := cleanTitle
			if t, ok := sec.Data["title"].(string); ok && t != "" {
				hTitle = t
			} else if h, ok := sec.Data["headline"].(string); ok && h != "" {
				hTitle = h
			}
			hSub := "Solusi terintegrasi yang dirancang untuk efisiensi, skalabilitas, dan kemudahan penggunaan."
			if s, ok := sec.Data["subtitle"].(string); ok && s != "" {
				hSub = s
			} else if d, ok := sec.Data["description"].(string); ok && d != "" {
				hSub = d
			}
			badge := "Platform Overview"
			if b, ok := sec.Data["badge"].(string); ok && b != "" {
				badge = b
			}
			ctaPrimary := "Mulai Sekarang"
			if c, ok := sec.Data["cta_primary"].(string); ok && c != "" {
				ctaPrimary = c
			} else if c, ok := sec.Data["cta_label"].(string); ok && c != "" {
				ctaPrimary = c
			}
			ctaSecondary := "Pelajari Lebih Lanjut"
			if c, ok := sec.Data["cta_secondary"].(string); ok && c != "" {
				ctaSecondary = c
			}

			templateBuilder.WriteString(fmt.Sprintf(`    <!-- Hero Section -->
    <section id="hero" data-rl-id="sec-hero" data-rl-kind="section" class="w-full px-6 py-16 md:py-24 max-w-7xl mx-auto border-b %s">
      <div class="grid grid-cols-1 lg:grid-cols-12 gap-12 items-center">
        <div class="lg:col-span-7 space-y-6 text-left">
          <div data-rl-id="cmp-hero-badge" data-rl-kind="component" class="inline-flex items-center gap-2 px-3 py-1 rounded-full text-xs font-semibold %s">
            <span class="w-2 h-2 rounded-full" style="background-color: %s"></span>
            <span>%s</span>
          </div>
          <h1 data-rl-id="cmp-hero-headline" data-rl-kind="component" class="text-4xl sm:text-5xl font-extrabold tracking-tight leading-tight">%s</h1>
          <p data-rl-id="cmp-hero-sub" data-rl-kind="component" class="text-base opacity-75 max-w-xl leading-relaxed">%s</p>
          <div class="pt-2 flex flex-wrap items-center gap-4">
            <a href="#action" data-rl-id="cmp-hero-cta" data-rl-kind="component" class="px-6 py-3.5 rounded-xl text-xs font-bold text-white shadow-lg transition-transform active:scale-95 cursor-pointer" style="background-color: %s">%s</a>
            <a href="#details" data-rl-id="cmp-hero-secondary-cta" data-rl-kind="component" class="px-6 py-3.5 rounded-xl text-xs font-semibold border %s cursor-pointer">%s</a>
          </div>
        </div>
        <div class="lg:col-span-5">
          <div data-rl-id="cmp-hero-media" data-rl-kind="component" class="relative rounded-3xl overflow-hidden shadow-2xl border %s p-8 flex flex-col justify-between aspect-4/3 %s">
            <div class="space-y-3 text-left">
              <div class="w-10 h-10 rounded-xl flex items-center justify-center font-bold text-white text-sm" style="background-color: %s">✓</div>
              <h3 class="text-lg font-bold">%s</h3>
              <p class="text-xs opacity-75">Tampilan interaktif dan data real-time dalam satu visualisasi komprehensif.</p>
            </div>
            <div class="pt-4 border-t %s flex items-center justify-between text-xs opacity-80">
              <span>Status Sistem</span>
              <span class="font-bold text-emerald-500">● Aktif &amp; Terpantau</span>
            </div>
          </div>
        </div>
      </div>
    </section>
`, func() string {
				if isDark {
					return "border-slate-800/80"
				}
				return "border-slate-200"
			}(), func() string {
				if isDark {
					return "border-slate-800 bg-slate-900/60 text-slate-300"
				}
				return "border-slate-200 bg-slate-100 text-slate-700"
			}(), accentColor, badge, hTitle, hSub, accentColor, ctaPrimary, func() string {
				if isDark {
					return "border-slate-700 bg-slate-800 text-slate-200"
				}
				return "border-slate-300 bg-white text-slate-700 shadow-sm"
			}(), ctaSecondary, func() string {
				if isDark {
					return "border-slate-800"
				}
				return "border-slate-200 shadow-md"
			}(), func() string {
				if isDark {
					return "bg-slate-900/60"
				}
				return "bg-white"
			}(), accentColor, cleanTitle, func() string {
				if isDark {
					return "border-slate-800"
				}
				return "border-slate-100"
			}()))

		case "product_grid", "features", "card_grid", "grid":
			pTitle := "Fitur & Kapabilitas Utama"
			if t, ok := sec.Data["title"].(string); ok && t != "" {
				pTitle = t
			} else if h, ok := sec.Data["headline"].(string); ok && h != "" {
				pTitle = h
			}
			pSub := "Dirancang secara modular untuk memberikan efisiensi operasional dan akurasi tinggi."
			if s, ok := sec.Data["subtitle"].(string); ok && s != "" {
				pSub = s
			} else if d, ok := sec.Data["description"].(string); ok && d != "" {
				pSub = d
			}

			templateBuilder.WriteString(fmt.Sprintf(`    <!-- Feature & Content Grid Section -->
    <section id="products" data-rl-id="sec-products" data-rl-kind="section" class="w-full px-6 py-16 max-w-7xl mx-auto border-b %s">
      <div class="flex flex-col md:flex-row md:items-end justify-between mb-10 gap-4 text-left">
        <div>
          <span class="text-xs font-bold uppercase tracking-widest" style="color: %s">Kategori Utama</span>
          <h2 class="text-2xl sm:text-3xl font-bold tracking-tight mt-1">%s</h2>
          <p class="text-sm opacity-70 mt-1 max-w-xl">%s</p>
        </div>
      </div>
      <div class="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-6">
        <div data-rl-id="cmp-card-1" data-rl-kind="component" class="p-6 rounded-2xl border %s flex flex-col justify-between text-left hover:shadow-lg transition-shadow">
          <div class="space-y-3">
            <span class="w-8 h-8 rounded-lg flex items-center justify-center text-xs font-bold text-white" style="background-color: %s">01</span>
            <h3 class="font-bold text-base">Modul Analisis Data</h3>
            <p class="text-xs opacity-70 leading-relaxed">Pengolahan informasi secara real-time dengan visualisasi metrik yang akurat dan terstruktur.</p>
          </div>
          <div class="mt-6 pt-3 border-t %s flex items-center justify-between text-xs font-semibold">
            <span class="opacity-60">Status: Siap</span>
            <span style="color: %s">Lihat Rincian →</span>
          </div>
        </div>
        <div data-rl-id="cmp-card-2" data-rl-kind="component" class="p-6 rounded-2xl border %s flex flex-col justify-between text-left hover:shadow-lg transition-shadow">
          <div class="space-y-3">
            <span class="w-8 h-8 rounded-lg flex items-center justify-center text-xs font-bold text-white" style="background-color: %s">02</span>
            <h3 class="font-bold text-base">Integrasi & Alur Kerja</h3>
            <p class="text-xs opacity-70 leading-relaxed">Sinkronisasi otomatis antar komponen dengan protokol API yang aman dan terstandarisasi.</p>
          </div>
          <div class="mt-6 pt-3 border-t %s flex items-center justify-between text-xs font-semibold">
            <span class="opacity-60">Status: Terhubung</span>
            <span style="color: %s">Lihat Rincian →</span>
          </div>
        </div>
        <div data-rl-id="cmp-card-3" data-rl-kind="component" class="p-6 rounded-2xl border %s flex flex-col justify-between text-left hover:shadow-lg transition-shadow">
          <div class="space-y-3">
            <span class="w-8 h-8 rounded-lg flex items-center justify-center text-xs font-bold text-white" style="background-color: %s">03</span>
            <h3 class="font-bold text-base">Keamanan & Kontrol Akses</h3>
            <p class="text-xs opacity-70 leading-relaxed">Manajemen otorisasi granular memastikan keamanan data dan kepatuhan standar industri.</p>
          </div>
          <div class="mt-6 pt-3 border-t %s flex items-center justify-between text-xs font-semibold">
            <span class="opacity-60">Status: Terproteksi</span>
            <span style="color: %s">Lihat Rincian →</span>
          </div>
        </div>
      </div>
    </section>
`, func() string {
				if isDark {
					return "border-slate-800/80"
				}
				return "border-slate-200"
			}(), accentColor, pTitle, pSub, func() string {
				if isDark {
					return "border-slate-800 bg-slate-900/50"
				}
				return "border-slate-200 bg-white shadow-xs"
			}(), accentColor, func() string {
				if isDark {
					return "border-slate-800"
				}
				return "border-slate-100"
			}(), accentColor, func() string {
				if isDark {
					return "border-slate-800 bg-slate-900/50"
				}
				return "border-slate-200 bg-white shadow-xs"
			}(), accentColor, func() string {
				if isDark {
					return "border-slate-800"
				}
				return "border-slate-100"
			}(), accentColor, func() string {
				if isDark {
					return "border-slate-800 bg-slate-900/50"
				}
				return "border-slate-200 bg-white shadow-xs"
			}(), accentColor, func() string {
				if isDark {
					return "border-slate-800"
				}
				return "border-slate-100"
			}(), accentColor))

		case "spotlight", "brand_story", "about":
			sTitle := "Arsitektur & Komitmen Kualitas"
			if t, ok := sec.Data["title"].(string); ok && t != "" {
				sTitle = t
			}
			sDesc := "Platform ini dibangun dengan prinsip keandalan, kejelasan informasi, dan performa tinggi untuk mendukung operasional yang mulus."
			if d, ok := sec.Data["description"].(string); ok && d != "" {
				sDesc = d
			} else if s, ok := sec.Data["subtitle"].(string); ok && s != "" {
				sDesc = s
			}

			templateBuilder.WriteString(fmt.Sprintf(`    <!-- Spotlight / Architecture Section -->
    <section id="story" data-rl-id="sec-spotlight" data-rl-kind="section" class="w-full px-6 py-16 max-w-7xl mx-auto border-b %s">
      <div class="grid grid-cols-1 lg:grid-cols-2 gap-12 items-center text-left">
        <div class="space-y-6">
          <span class="text-xs font-bold uppercase tracking-widest" style="color: %s">Karakteristik Sistem</span>
          <h2 data-rl-id="cmp-story-headline" data-rl-kind="component" class="text-3xl font-extrabold tracking-tight">%s</h2>
          <p data-rl-id="cmp-story-desc" data-rl-kind="component" class="text-sm opacity-75 leading-relaxed">%s</p>
          <div class="grid grid-cols-2 gap-6 pt-4">
            <div>
              <div class="text-2xl font-black" style="color: %s">99.9%%</div>
              <div class="text-xs font-semibold opacity-70 mt-1">Ketersediaan &amp; Keandalan</div>
            </div>
            <div>
              <div class="text-2xl font-black" style="color: %s">&lt; 50ms</div>
              <div class="text-xs font-semibold opacity-70 mt-1">Respon Cepat &amp; Efisien</div>
            </div>
          </div>
        </div>
        <div data-rl-id="cmp-story-media" data-rl-kind="component" class="p-8 rounded-3xl border %s %s shadow-xl space-y-4">
          <h4 class="font-bold text-sm">Standar Kualitas &amp; Kepatuhan</h4>
          <p class="text-xs opacity-70 leading-relaxed">Seluruh komponen mematuhi standar desain modern dan struktur data yang tervalidasi.</p>
          <div class="space-y-2 pt-2 text-xs font-medium">
            <div class="p-2.5 rounded-lg border %s flex items-center justify-between">
              <span>Kesesuaian Spesifikasi</span>
              <span class="font-bold text-emerald-500">100%% Terverifikasi</span>
            </div>
            <div class="p-2.5 rounded-lg border %s flex items-center justify-between">
              <span>Keamanan Antarmuka</span>
              <span class="font-bold text-emerald-500">Standar Industri</span>
            </div>
          </div>
        </div>
      </div>
    </section>
`, func() string {
				if isDark {
					return "border-slate-800/80"
				}
				return "border-slate-200"
			}(), accentColor, sTitle, sDesc, accentColor, accentColor, func() string {
				if isDark {
					return "border-slate-800 bg-slate-900/40"
				}
				return "border-slate-200 bg-white"
			}(), func() string {
				if isDark {
					return "bg-slate-900/40"
				}
				return "bg-slate-50"
			}(), func() string {
				if isDark {
					return "border-slate-800 bg-slate-950/60"
				}
				return "border-slate-200 bg-white"
			}(), func() string {
				if isDark {
					return "border-slate-800 bg-slate-950/60"
				}
				return "border-slate-200 bg-white"
			}()))

		case "testimonials", "reviews":
			tTitle := "Umpan Balik Pengguna"
			if t, ok := sec.Data["title"].(string); ok && t != "" {
				tTitle = t
			}
			templateBuilder.WriteString(fmt.Sprintf(`    <!-- Testimonials Section -->
    <section id="reviews" data-rl-id="sec-reviews" data-rl-kind="section" class="w-full px-6 py-16 max-w-7xl mx-auto border-b %s">
      <div class="text-center max-w-xl mx-auto mb-12">
        <span class="text-xs font-bold uppercase tracking-widest" style="color: %s">Testimoni</span>
        <h2 class="text-2xl sm:text-3xl font-bold tracking-tight mt-1">%s</h2>
      </div>
      <div class="grid grid-cols-1 md:grid-cols-3 gap-6 text-left">
        <div data-rl-id="cmp-review-card-1" data-rl-kind="component" class="p-6 rounded-2xl border %s flex flex-col justify-between">
          <p class="text-xs opacity-80 leading-relaxed italic">"Antarmuka sangat responsif dan intuitif, mempermudah tim kami dalam menyelesaikan alur kerja harian."</p>
          <div class="mt-6 flex items-center gap-3">
            <div class="w-8 h-8 rounded-full flex items-center justify-center text-xs font-bold text-white" style="background-color: %s">U1</div>
            <div>
              <div class="text-xs font-bold">Pengguna Terverifikasi</div>
              <div class="text-[10px] opacity-60">Operasional</div>
            </div>
          </div>
        </div>
        <div data-rl-id="cmp-review-card-2" data-rl-kind="component" class="p-6 rounded-2xl border %s flex flex-col justify-between">
          <p class="text-xs opacity-80 leading-relaxed italic">"Visualisasi data sangat jelas dan terstruktur dengan rapi tanpa elemen yang mengganggu."</p>
          <div class="mt-6 flex items-center gap-3">
            <div class="w-8 h-8 rounded-full flex items-center justify-center text-xs font-bold text-white" style="background-color: %s">U2</div>
            <div>
              <div class="text-xs font-bold">Analis Data</div>
              <div class="text-[10px] opacity-60">Manajemen</div>
            </div>
          </div>
        </div>
        <div data-rl-id="cmp-review-card-3" data-rl-kind="component" class="p-6 rounded-2xl border %s flex flex-col justify-between">
          <p class="text-xs opacity-80 leading-relaxed italic">"Arsitektur yang solid dan performa tinggi sangat membantu efisiensi operasional organisasi."</p>
          <div class="mt-6 flex items-center gap-3">
            <div class="w-8 h-8 rounded-full flex items-center justify-center text-xs font-bold text-white" style="background-color: %s">U3</div>
            <div>
              <div class="text-xs font-bold">Koordinator Tim</div>
              <div class="text-[10px] opacity-60">Produktivitas</div>
            </div>
          </div>
        </div>
      </div>
    </section>
`, func() string {
				if isDark {
					return "border-slate-800/80"
				}
				return "border-slate-200"
			}(), accentColor, tTitle, func() string {
				if isDark {
					return "border-slate-800 bg-slate-900/40"
				}
				return "border-slate-200 bg-white shadow-xs"
			}(), accentColor, func() string {
				if isDark {
					return "border-slate-800 bg-slate-900/40"
				}
				return "border-slate-200 bg-white shadow-xs"
			}(), accentColor, func() string {
				if isDark {
					return "border-slate-800 bg-slate-900/40"
				}
				return "border-slate-200 bg-white shadow-xs"
			}(), accentColor))

		case "footer":
			fBrand := cleanTitle
			if b, ok := sec.Data["brand_name"].(string); ok && b != "" {
				fBrand = b
			} else if b, ok := sec.Data["brand"].(string); ok && b != "" {
				fBrand = b
			}
			fDesc := "Solusi terpadu dengan desain intuitif dan arsitektur handal."
			if d, ok := sec.Data["description"].(string); ok && d != "" {
				fDesc = d
			}
			templateBuilder.WriteString(fmt.Sprintf(`    <!-- Footer Section -->
    <footer id="sec-footer" data-rl-id="sec-footer" data-rl-kind="section" class="w-full px-6 py-12 max-w-7xl mx-auto text-left">
      <div class="grid grid-cols-1 md:grid-cols-4 gap-8 mb-10">
        <div class="space-y-3">
          <h3 class="font-bold text-lg">%s</h3>
          <p class="text-xs opacity-60 leading-relaxed">%s</p>
        </div>
        <div class="space-y-2 text-xs">
          <h4 class="font-bold opacity-80">Navigasi</h4>
          <a href="#hero" class="block opacity-60 hover:opacity-100 cursor-pointer">Beranda</a>
          <a href="#products" class="block opacity-60 hover:opacity-100 cursor-pointer">Fitur</a>
          <a href="#story" class="block opacity-60 hover:opacity-100 cursor-pointer">Tentang</a>
        </div>
        <div class="space-y-2 text-xs">
          <h4 class="font-bold opacity-80">Informasi</h4>
          <span class="block opacity-60">Dokumentasi</span>
          <span class="block opacity-60">Panduan Pengguna</span>
          <span class="block opacity-60">Status Layanan</span>
        </div>
        <div class="space-y-2 text-xs">
          <h4 class="font-bold opacity-80">Bantuan</h4>
          <span class="block opacity-60">Pusat Dukungan</span>
          <span class="block opacity-60">Kontak Tim</span>
        </div>
      </div>
      <div class="pt-6 border-t %s flex items-center justify-between text-[11px] opacity-50">
        <div>© 2026 %s. All rights reserved.</div>
        <div class="flex gap-4">
          <a href="#sec-header" class="hover:underline">Kembali ke Atas ↑</a>
          <span>Privasi</span>
          <span>Ketentuan</span>
        </div>
      </div>
    </footer>
`, fBrand, fDesc, func() string {
				if isDark {
					return "border-slate-800"
				}
				return "border-slate-200"
			}(), fBrand))

		case "form", "auth_card", "login_card":
			fTitle := "Masuk ke Akun Anda"
			if t, ok := sec.Data["title"].(string); ok && t != "" {
				fTitle = t
			}
			fSub := "Masuk dengan kredensial Anda."
			if s, ok := sec.Data["subtitle"].(string); ok && s != "" {
				fSub = s
			}
			btnLabel := "Lanjutkan Masuk"
			if l, ok := sec.Data["submit_label"].(string); ok && l != "" {
				btnLabel = l
			}

			// Social buttons (ONLY if explicitly non-empty)
			var socialBuilder strings.Builder
			if sb, ok := sec.Data["social_buttons"].([]interface{}); ok && len(sb) > 0 {
				socialBuilder.WriteString("        <div class=\"grid grid-cols-2 gap-2 mb-4\">\n")
				for _, p := range sb {
					pStr := fmt.Sprintf("%v", p)
					socialBuilder.WriteString(fmt.Sprintf("          <button type=\"button\" class=\"py-2 px-3 rounded-xl border text-xs font-semibold %s\">%s</button>\n", func() string {
						if isDark {
							return "border-slate-700 bg-slate-800 text-slate-200"
						}
						return "border-slate-300 bg-slate-50 text-slate-700"
					}(), pStr))
				}
				socialBuilder.WriteString("        </div>\n")
			} else if sbStr, ok := sec.Data["social_buttons"].([]string); ok && len(sbStr) > 0 {
				socialBuilder.WriteString("        <div class=\"grid grid-cols-2 gap-2 mb-4\">\n")
				for _, pStr := range sbStr {
					socialBuilder.WriteString(fmt.Sprintf("          <button type=\"button\" class=\"py-2 px-3 rounded-xl border text-xs font-semibold %s\">%s</button>\n", func() string {
						if isDark {
							return "border-slate-700 bg-slate-800 text-slate-200"
						}
						return "border-slate-300 bg-slate-50 text-slate-700"
					}(), pStr))
				}
				socialBuilder.WriteString("        </div>\n")
			}

			templateBuilder.WriteString(fmt.Sprintf(`    <!-- Authentication / Form Section -->
    <section class="w-full px-6 py-12 flex items-center justify-center">
      <div class="w-full max-w-md p-8 rounded-2xl border %s shadow-xl text-left">
        <h2 class="text-xl font-bold tracking-tight mb-1">%s</h2>
        <p class="text-xs opacity-70 mb-6 leading-relaxed">%s</p>
%s        <form class="space-y-4" onsubmit="return false;">
          <div>
            <label class="block text-xs font-medium opacity-80 mb-1">Email</label>
            <input type="email" placeholder="nama@email.com" class="w-full px-3.5 py-2.5 rounded-xl border text-xs outline-none %s" />
          </div>
          <div>
            <label class="block text-xs font-medium opacity-80 mb-1">Kata Sandi</label>
            <input type="password" placeholder="••••••••" class="w-full px-3.5 py-2.5 rounded-xl border text-xs outline-none %s" />
          </div>
          <button type="submit" class="w-full py-2.5 rounded-xl text-xs font-bold text-white shadow-sm mt-2" style="background-color: %s">%s</button>
        </form>
      </div>
    </section>
`, func() string {
				if isDark {
					return "border-slate-800 bg-slate-900"
				}
				return "border-slate-200 bg-white"
			}(), fTitle, fSub, socialBuilder.String(), func() string {
				if isDark {
					return "border-slate-700 bg-slate-950 text-white"
				}
				return "border-slate-300 bg-slate-50 text-slate-900"
			}(), func() string {
				if isDark {
					return "border-slate-700 bg-slate-950 text-white"
				}
				return "border-slate-300 bg-slate-50 text-slate-900"
			}(), accentColor, btnLabel))

		default:
			templateBuilder.WriteString(fmt.Sprintf(`    <!-- %s Section -->
    <section class="w-full px-6 py-6 border-b %s">
      <div class="text-xs font-semibold uppercase tracking-wider opacity-60">%s</div>
    </section>
`, sec.Type, func() string {
				if isDark {
					return "border-slate-800"
				}
				return "border-slate-200"
			}(), sec.Type))
		}
	}

	templateBuilder.WriteString("  </div>\n</template>\n")

	vueCode := templateBuilder.String()
	htmlCode := strings.Replace(vueCode, "<template>\n", "", 1)
	htmlCode = strings.Replace(htmlCode, "</template>\n", "", 1)

	return map[string]string{
		"vue":  vueCode,
		"html": htmlCode,
	}
}
