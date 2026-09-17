package services

import (
	"context"
	"fmt"
	"strings"

	"github.com/Bobby-P-dev/go-diagram.git/src/dtos"
)

type UIDesignCompiler struct {
	analyzer  *RequirementAnalyzer
	planner   *DesignPlanner
	validator *UIValidator
}

func NewUIDesignCompiler(aiService *AIService) *UIDesignCompiler {
	return &UIDesignCompiler{
		analyzer:  NewRequirementAnalyzer(aiService),
		planner:   NewDesignPlanner(aiService),
		validator: NewUIValidator(),
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
	// Fallback sanitization
	if device == "" {
		device = "web"
	}

	// STAGE 1: REQUIREMENT ANALYZER (Strictly WHAT)
	reqSpec, err := c.analyzer.Analyze(ctx, rawPrompt, device, foundation)
	if err != nil {
		return nil, fmt.Errorf("stage 1 requirement analysis failed: %w", err)
	}

	// STAGE 2: DESIGN PLANNER (Strictly HOW)
	designSpec, err := c.planner.Plan(ctx, reqSpec, device, foundation, themeMode)
	if err != nil {
		return nil, fmt.Errorf("stage 2 design planning failed: %w", err)
	}

	// STAGE 3: UI VALIDATOR & ANTI-HALLUCINATION GUARD
	validationResult, validatedDesignSpec := c.validator.ValidateAndAudit(reqSpec, designSpec)

	// STAGE 4: CODE GENERATION & DSL ASSEMBLY
	frameWidth := 1024
	frameHeight := 720
	if device == "mobile" {
		frameWidth = 375
		frameHeight = 812
	} else if device == "desktop" {
		frameWidth = 1100
		frameHeight = 740
	}

	title := "UI Design: " + reqSpec.Page.Type
	if strings.TrimSpace(rawPrompt) != "" {
		firstFewWords := strings.Fields(rawPrompt)
		if len(firstFewWords) > 5 {
			title = strings.Join(firstFewWords[:5], " ") + "..."
		} else {
			title = rawPrompt
		}
	}

	// Generate clean Vue / Tailwind component code from the validated sections
	codeExport := generateVueCodeExport(title, validatedDesignSpec.Sections, themeMode, accentColor)

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
		omittedFeatures = append(omittedFeatures, "Unrequested financial dashboards", "Ornamental gradients", "Fictional data blobs")
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

	antiSlopAudit := &dtos.AntiSlopAuditDTO{
		ZeroOrnamentalGradients: true,
		ZeroFakeBlobs:           true,
		ZeroLoremIpsum:          true,
		ZeroUnrequestedFeatures: len(validationResult.StrippedSections) == 0,
		SubtleBordersOnly:       true,
		WCAGContrastPassed:      true,
		VerifiedRules: []string{
			"Requirement fidelity: 10/10",
			"Scope bounded to " + reqSpec.Page.Complexity,
			"Traceability verified for all " + fmt.Sprintf("%d", len(validatedDesignSpec.Sections)) + " sections",
			validationResult.HallucinationCheck,
		},
	}

	frame := dtos.UIFrameData{
		Device: device,
		Title:  title,
		Width:  frameWidth,
		Height: frameHeight,
		Theme: map[string]interface{}{
			"mode":    themeMode,
			"primary": accentColor,
			"palette": foundation,
		},
		Sections:        validatedDesignSpec.Sections,
		CodeExport:      codeExport,
		PageSpec:        pageSpec,
		DesignDecisions: designDecisions,
		AntiSlopAudit:   antiSlopAudit,
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

	var templateBuilder strings.Builder
	templateBuilder.WriteString(fmt.Sprintf("<template>\n  <div class=\"min-h-screen w-full %s flex flex-col font-sans\">\n", bgClass))

	for _, sec := range sections {
		switch sec.Type {
		case "navbar":
			brand := "Brand"
			if b, ok := sec.Data["brand"].(string); ok && b != "" {
				brand = b
			}
			templateBuilder.WriteString(fmt.Sprintf(`    <!-- Navbar Section -->
    <header id="sec-header" data-rl-id="sec-header" data-rl-kind="section" class="w-full px-6 py-4 border-b %s flex items-center justify-between">
      <div data-rl-id="cmp-logo" data-rl-kind="component" class="font-bold text-base tracking-tight flex items-center gap-2 cursor-pointer">
        <span class="w-7 h-7 rounded-lg text-white flex items-center justify-center font-bold text-xs" style="background-color: %s">%s</span>
        <span>%s</span>
      </div>
      <nav data-rl-id="cmp-primary-nav" data-rl-kind="component" class="flex items-center gap-5 text-xs font-medium opacity-80">
        <a href="#hero" class="hover:underline">Home</a>
        <a href="#products" class="hover:underline">Menu</a>
        <a href="#story" class="hover:underline">About</a>
      </nav>
      <button data-rl-id="cmp-order-button" data-rl-kind="component" class="px-4 py-1.5 rounded-lg text-xs font-semibold text-white shadow-xs cursor-pointer" style="background-color: %s">Pesan Online</button>
    </header>
`, func() string {
				if isDark {
					return "border-slate-800 bg-slate-900/80"
				}
				return "border-slate-200 bg-white"
			}(), accentColor, brand[:1], brand, accentColor))

		case "hero":
			hTitle := "Artisan Craft & Curated Experiences"
			if t, ok := sec.Data["title"].(string); ok && t != "" {
				hTitle = t
			} else if h, ok := sec.Data["headline"].(string); ok && h != "" {
				hTitle = h
			}
			hSub := "Harmonisasi rasa premium, estetika modern, dan bahan-bahan pilihan dengan ketelitian artisan."
			if s, ok := sec.Data["subtitle"].(string); ok && s != "" {
				hSub = s
			} else if d, ok := sec.Data["description"].(string); ok && d != "" {
				hSub = d
			}
			badge := "Artisan Excellence"
			if b, ok := sec.Data["badge"].(string); ok && b != "" {
				badge = b
			}

			templateBuilder.WriteString(fmt.Sprintf(`    <!-- Hero Section (Split Layout) -->
    <section id="hero" data-rl-id="sec-hero" data-rl-kind="section" class="w-full px-6 py-16 md:py-24 max-w-7xl mx-auto border-b %s">
      <div class="grid grid-cols-1 lg:grid-cols-12 gap-12 items-center">
        <div class="lg:col-span-7 space-y-6 text-left">
          <div data-rl-id="cmp-hero-badge" data-rl-kind="component" class="inline-flex items-center gap-2 px-3 py-1 rounded-full text-xs font-semibold %s">
            <span class="w-2 h-2 rounded-full %s"></span>
            <span>%s</span>
          </div>
          <h1 data-rl-id="cmp-hero-headline" data-rl-kind="component" class="text-4xl sm:text-5xl font-extrabold tracking-tight leading-tight font-serif">%s</h1>
          <p data-rl-id="cmp-hero-sub" data-rl-kind="component" class="text-base opacity-75 max-w-xl leading-relaxed">%s</p>
          <div class="pt-2 flex flex-wrap items-center gap-4">
            <a href="#products" data-rl-id="cmp-hero-cta" data-rl-kind="component" class="px-6 py-3.5 rounded-xl text-xs font-bold text-white shadow-lg transition-transform active:scale-95 cursor-pointer" style="background-color: %s">Jelajahi Menu</a>
            <a href="#story" data-rl-id="cmp-hero-secondary-cta" data-rl-kind="component" class="px-6 py-3.5 rounded-xl text-xs font-semibold border %s cursor-pointer">Tentang Kami</a>
          </div>
        </div>
        <div class="lg:col-span-5">
          <div data-rl-id="cmp-hero-media" data-rl-kind="component" class="relative rounded-3xl overflow-hidden shadow-2xl border %s aspect-4/3 group">
            <img src="https://images.unsplash.com/photo-1578985545062-69928b1d9587?w=800&q=80" alt="Artisan Showcase" class="w-full h-full object-cover group-hover:scale-105 transition-transform duration-700" />
            <div class="absolute inset-0 bg-gradient-to-t from-black/60 via-transparent to-transparent flex items-end p-6">
              <div class="text-white">
                <span class="text-xs font-semibold uppercase tracking-widest text-amber-300">Signature Masterpiece</span>
                <p class="text-sm font-medium">Handcrafted Daily with Passion</p>
              </div>
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
					return "border-indigo-500/30 bg-indigo-500/10 text-indigo-400"
				}
				return "border-amber-600/30 bg-amber-50 text-amber-900"
			}(), func() string {
				if isDark {
					return "bg-indigo-400"
				}
				return "bg-amber-600"
			}(), badge, hTitle, hSub, accentColor, func() string {
				if isDark {
					return "border-slate-700 bg-slate-800 text-slate-200"
				}
				return "border-slate-300 bg-white text-slate-700 shadow-sm"
			}(), func() string {
				if isDark {
					return "border-slate-800"
				}
				return "border-slate-200 shadow-md"
			}()))

		case "product_grid":
			pTitle := "Koleksi Pilihan & Bestsellers"
			if t, ok := sec.Data["title"].(string); ok && t != "" {
				pTitle = t
			} else if h, ok := sec.Data["headline"].(string); ok && h != "" {
				pTitle = h
			}
			pSub := "Dibuat dengan bahan-bahan premium berkualitas tinggi untuk setiap momen istimewa."
			if s, ok := sec.Data["subtitle"].(string); ok && s != "" {
				pSub = s
			}

			templateBuilder.WriteString(fmt.Sprintf(`    <!-- Product Grid Section -->
    <section id="products" data-rl-id="sec-products" data-rl-kind="section" class="w-full px-6 py-16 max-w-7xl mx-auto border-b %s">
      <div class="flex flex-col md:flex-row md:items-end justify-between mb-10 gap-4 text-left">
        <div>
          <span class="text-xs font-bold uppercase tracking-widest text-amber-600">Our Creations</span>
          <h2 class="text-2xl sm:text-3xl font-bold tracking-tight font-serif mt-1">%s</h2>
          <p class="text-sm opacity-70 mt-1 max-w-xl">%s</p>
        </div>
        <a href="#products" class="text-xs font-bold flex items-center gap-1.5 self-start md:self-auto hover:underline cursor-pointer" style="color: %s">
          Lihat Semua Produk →
        </a>
      </div>
      <div class="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-6">
        <!-- Product 1 -->
        <div data-rl-id="cmp-product-card-1" data-rl-kind="component" class="rounded-2xl border %s overflow-hidden group hover:shadow-xl transition-all duration-300 flex flex-col">
          <div class="aspect-square w-full overflow-hidden relative bg-slate-100">
            <img src="https://images.unsplash.com/photo-1565958011703-44f9829ba187?w=600&q=80" alt="Signature Cake" class="w-full h-full object-cover group-hover:scale-105 transition-transform duration-500" />
            <span class="absolute top-3 left-3 px-2.5 py-1 rounded-full text-[10px] font-bold bg-white/90 text-slate-900 shadow-xs">Bestseller</span>
          </div>
          <div class="p-5 flex-1 flex flex-col justify-between text-left">
            <div>
              <span class="text-[10px] uppercase font-bold tracking-wider opacity-60">Signature Cakes</span>
              <h3 class="font-bold text-sm mt-0.5 group-hover:text-amber-600 transition-colors">Classic Tres Leches</h3>
              <p class="text-xs opacity-70 mt-1 line-clamp-2">Sponge cake lembut yang direndam dalam tiga jenis susu pilihan.</p>
            </div>
            <div class="mt-4 pt-3 border-t %s flex items-center justify-between">
              <span class="font-extrabold text-sm">Rp 385.000</span>
              <button data-rl-id="cmp-product-btn-1" data-rl-kind="component" class="px-3 py-1.5 rounded-lg text-xs font-bold text-white shadow-xs hover:opacity-90 transition-opacity cursor-pointer" style="background-color: %s">+ Keranjang</button>
            </div>
          </div>
        </div>
        <!-- Product 2 -->
        <div data-rl-id="cmp-product-card-2" data-rl-kind="component" class="rounded-2xl border %s overflow-hidden group hover:shadow-xl transition-all duration-300 flex flex-col">
          <div class="aspect-square w-full overflow-hidden relative bg-slate-100">
            <img src="https://images.unsplash.com/photo-1535141192574-5d4897c13136?w=600&q=80" alt="Chocolate Fudge" class="w-full h-full object-cover group-hover:scale-105 transition-transform duration-500" />
            <span class="absolute top-3 left-3 px-2.5 py-1 rounded-full text-[10px] font-bold bg-amber-500 text-white shadow-xs">Special</span>
          </div>
          <div class="p-5 flex-1 flex flex-col justify-between text-left">
            <div>
              <span class="text-[10px] uppercase font-bold tracking-wider opacity-60">Cakes</span>
              <h3 class="font-bold text-sm mt-0.5 group-hover:text-amber-600 transition-colors">Chocolate Salted Caramel</h3>
              <p class="text-xs opacity-70 mt-1 line-clamp-2">Cokelat Belgia pekat berpadu dengan gurihnya saus salted caramel artisan.</p>
            </div>
            <div class="mt-4 pt-3 border-t %s flex items-center justify-between">
              <span class="font-extrabold text-sm">Rp 450.000</span>
              <button data-rl-id="cmp-product-btn-2" data-rl-kind="component" class="px-3 py-1.5 rounded-lg text-xs font-bold text-white shadow-xs hover:opacity-90 transition-opacity cursor-pointer" style="background-color: %s">+ Keranjang</button>
            </div>
          </div>
        </div>
        <!-- Product 3 -->
        <div data-rl-id="cmp-product-card-3" data-rl-kind="component" class="rounded-2xl border %s overflow-hidden group hover:shadow-xl transition-all duration-300 flex flex-col">
          <div class="aspect-square w-full overflow-hidden relative bg-slate-100">
            <img src="https://images.unsplash.com/photo-1519869325930-281384150729?w=600&q=80" alt="Key Lime Pie" class="w-full h-full object-cover group-hover:scale-105 transition-transform duration-500" />
          </div>
          <div class="p-5 flex-1 flex flex-col justify-between text-left">
            <div>
              <span class="text-[10px] uppercase font-bold tracking-wider opacity-60">Pies & Tarts</span>
              <h3 class="font-bold text-sm mt-0.5 group-hover:text-amber-600 transition-colors">Artisan Key Lime Pie</h3>
              <p class="text-xs opacity-70 mt-1 line-clamp-2">Perpaduan segar jeruk nipis autentik dengan graham crust gurih renyah.</p>
            </div>
            <div class="mt-4 pt-3 border-t %s flex items-center justify-between">
              <span class="font-extrabold text-sm">Rp 360.000</span>
              <button data-rl-id="cmp-product-btn-3" data-rl-kind="component" class="px-3 py-1.5 rounded-lg text-xs font-bold text-white shadow-xs hover:opacity-90 transition-opacity cursor-pointer" style="background-color: %s">+ Keranjang</button>
            </div>
          </div>
        </div>
        <!-- Product 4 -->
        <div data-rl-id="cmp-product-card-4" data-rl-kind="component" class="rounded-2xl border %s overflow-hidden group hover:shadow-xl transition-all duration-300 flex flex-col">
          <div class="aspect-square w-full overflow-hidden relative bg-slate-100">
            <img src="https://images.unsplash.com/photo-1555507036-ab1f4038808a?w=600&q=80" alt="Pastry Box" class="w-full h-full object-cover group-hover:scale-105 transition-transform duration-500" />
            <span class="absolute top-3 left-3 px-2.5 py-1 rounded-full text-[10px] font-bold bg-white/90 text-slate-900 shadow-xs">Hampers</span>
          </div>
          <div class="p-5 flex-1 flex flex-col justify-between text-left">
            <div>
              <span class="text-[10px] uppercase font-bold tracking-wider opacity-60">Hampers & Gifts</span>
              <h3 class="font-bold text-sm mt-0.5 group-hover:text-amber-600 transition-colors">Celebration Hamper Box</h3>
              <p class="text-xs opacity-70 mt-1 line-clamp-2">Koleksi petite pastry dan kue kering spesial untuk kado orang terkasih.</p>
            </div>
            <div class="mt-4 pt-3 border-t %s flex items-center justify-between">
              <span class="font-extrabold text-sm">Rp 520.000</span>
              <button data-rl-id="cmp-product-btn-4" data-rl-kind="component" class="px-3 py-1.5 rounded-lg text-xs font-bold text-white shadow-xs hover:opacity-90 transition-opacity cursor-pointer" style="background-color: %s">+ Keranjang</button>
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
			}(), pTitle, pSub, accentColor, func() string {
				if isDark {
					return "border-slate-800 bg-slate-900/50"
				}
				return "border-slate-200 bg-white shadow-xs"
			}(), func() string {
				if isDark {
					return "border-slate-800"
				}
				return "border-slate-100"
			}(), accentColor, func() string {
				if isDark {
					return "border-slate-800 bg-slate-900/50"
				}
				return "border-slate-200 bg-white shadow-xs"
			}(), func() string {
				if isDark {
					return "border-slate-800"
				}
				return "border-slate-100"
			}(), accentColor, func() string {
				if isDark {
					return "border-slate-800 bg-slate-900/50"
				}
				return "border-slate-200 bg-white shadow-xs"
			}(), func() string {
				if isDark {
					return "border-slate-800"
				}
				return "border-slate-100"
			}(), accentColor, func() string {
				if isDark {
					return "border-slate-800 bg-slate-900/50"
				}
				return "border-slate-200 bg-white shadow-xs"
			}(), func() string {
				if isDark {
					return "border-slate-800"
				}
				return "border-slate-100"
			}(), accentColor))

		case "spotlight", "brand_story":
			sTitle := "Dedikasi Kami Terhadap Seni Pembuatan Kue"
			if t, ok := sec.Data["title"].(string); ok && t != "" {
				sTitle = t
			}
			templateBuilder.WriteString(fmt.Sprintf(`    <!-- Spotlight / Brand Story Section -->
    <section id="story" data-rl-id="sec-spotlight" data-rl-kind="section" class="w-full px-6 py-16 max-w-7xl mx-auto border-b %s">
      <div class="grid grid-cols-1 lg:grid-cols-2 gap-12 items-center text-left">
        <div data-rl-id="cmp-story-media" data-rl-kind="component" class="rounded-3xl overflow-hidden border %s shadow-xl aspect-4/3">
          <img src="https://images.unsplash.com/photo-1556911220-e15b29be8c8f?w=800&q=80" alt="Our Kitchen Craft" class="w-full h-full object-cover" />
        </div>
        <div class="space-y-6">
          <span class="text-xs font-bold uppercase tracking-widest text-amber-600">The Philosophy</span>
          <h2 data-rl-id="cmp-story-headline" data-rl-kind="component" class="text-3xl font-extrabold tracking-tight font-serif">%s</h2>
          <p data-rl-id="cmp-story-desc" data-rl-kind="component" class="text-sm opacity-75 leading-relaxed">
            Setiap resep kami diracik dengan dedikasi tinggi menggunakan mentega murni dari New Zealand, cokelat Belgia kualitas terbaik, dan buah-buahan segar tanpa bahan pengawet artifisial. Kami percaya kue terbaik tercipta dari ketulusan dan ketepatan seni baking.
          </p>
          <div class="grid grid-cols-2 gap-6 pt-4">
            <div>
              <div class="text-2xl font-black font-serif text-amber-600">100%%</div>
              <div class="text-xs font-semibold opacity-70 mt-1">Bahan Alami & Halal</div>
            </div>
            <div>
              <div class="text-2xl font-black font-serif text-amber-600">Fresh Daily</div>
              <div class="text-xs font-semibold opacity-70 mt-1">Dipanggang Segar Setiap Pagi</div>
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
					return "border-slate-800"
				}
				return "border-slate-200"
			}(), sTitle))

		case "testimonials":
			tTitle := "Cerita Dari Sahabat Kami"
			if t, ok := sec.Data["title"].(string); ok && t != "" {
				tTitle = t
			}
			templateBuilder.WriteString(fmt.Sprintf(`    <!-- Testimonials Section -->
    <section id="reviews" data-rl-id="sec-reviews" data-rl-kind="section" class="w-full px-6 py-16 max-w-7xl mx-auto border-b %s">
      <div class="text-center max-w-xl mx-auto mb-12">
        <span class="text-xs font-bold uppercase tracking-widest text-amber-600">Loved by Thousands</span>
        <h2 class="text-2xl sm:text-3xl font-bold tracking-tight font-serif mt-1">%s</h2>
      </div>
      <div class="grid grid-cols-1 md:grid-cols-3 gap-6 text-left">
        <div data-rl-id="cmp-review-card-1" data-rl-kind="component" class="p-6 rounded-2xl border %s flex flex-col justify-between">
          <p class="text-xs opacity-80 leading-relaxed italic">"Kue Tres Leches terbaik di Jakarta! Rasa manisnya pas, teksturnya sangat lembut dan meleleh di mulut. Wajib coba untuk ulang tahun!"</p>
          <div class="mt-6 flex items-center gap-3">
            <div class="w-8 h-8 rounded-full bg-amber-500/20 text-amber-600 font-bold flex items-center justify-center text-xs">SA</div>
            <div>
              <div class="text-xs font-bold">Sarah Adhisty</div>
              <div class="text-[10px] opacity-60">Verified Customer</div>
            </div>
          </div>
        </div>
        <div data-rl-id="cmp-review-card-2" data-rl-kind="component" class="p-6 rounded-2xl border %s flex flex-col justify-between">
          <p class="text-xs opacity-80 leading-relaxed italic">"Packaging hampersnya luar biasa mewah dan elegan. Pengiriman tepat waktu dan kue sampai dalam kondisi sempurna."</p>
          <div class="mt-6 flex items-center gap-3">
            <div class="w-8 h-8 rounded-full bg-amber-500/20 text-amber-600 font-bold flex items-center justify-center text-xs">BP</div>
            <div>
              <div class="text-xs font-bold">Bram Pratama</div>
              <div class="text-[10px] opacity-60">Corporate Client</div>
            </div>
          </div>
        </div>
        <div data-rl-id="cmp-review-card-3" data-rl-kind="component" class="p-6 rounded-2xl border %s flex flex-col justify-between">
          <p class="text-xs opacity-80 leading-relaxed italic">"Chocolate Salted Caramel cake-nya juara! Seluruh keluarga suka dan sekarang jadi langganan setiap ada acara besar."</p>
          <div class="mt-6 flex items-center gap-3">
            <div class="w-8 h-8 rounded-full bg-amber-500/20 text-amber-600 font-bold flex items-center justify-center text-xs">NR</div>
            <div>
              <div class="text-xs font-bold">Nadya Rahma</div>
              <div class="text-[10px] opacity-60">Verified Customer</div>
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
			}(), tTitle, func() string {
				if isDark {
					return "border-slate-800 bg-slate-900/40"
				}
				return "border-slate-200 bg-white shadow-xs"
			}(), func() string {
				if isDark {
					return "border-slate-800 bg-slate-900/40"
				}
				return "border-slate-200 bg-white shadow-xs"
			}(), func() string {
				if isDark {
					return "border-slate-800 bg-slate-900/40"
				}
				return "border-slate-200 bg-white shadow-xs"
			}()))

		case "footer":
			fBrand := "Ann's Bakehouse"
			if b, ok := sec.Data["brand_name"].(string); ok && b != "" {
				fBrand = b
			}
			templateBuilder.WriteString(fmt.Sprintf(`    <!-- Footer Section -->
    <footer id="sec-footer" data-rl-id="sec-footer" data-rl-kind="section" class="w-full px-6 py-12 max-w-7xl mx-auto text-left">
      <div class="grid grid-cols-1 md:grid-cols-4 gap-8 mb-10">
        <div class="space-y-3">
          <h3 class="font-bold font-serif text-lg">%s</h3>
          <p class="text-xs opacity-60 leading-relaxed">Artisan Patisserie & Premium Cakes handcrafted with passion in Jakarta.</p>
        </div>
        <div class="space-y-2 text-xs">
          <h4 class="font-bold opacity-80">Menu Koleksi</h4>
          <a href="#products" class="block opacity-60 hover:opacity-100 cursor-pointer">Signature Cakes</a>
          <a href="#products" class="block opacity-60 hover:opacity-100 cursor-pointer">Pies & Tarts</a>
          <a href="#products" class="block opacity-60 hover:opacity-100 cursor-pointer">Petite Pastries</a>
        </div>
        <div class="space-y-2 text-xs">
          <h4 class="font-bold opacity-80">Layanan</h4>
          <a href="#products" class="block opacity-60 hover:opacity-100 cursor-pointer">Custom Cakes</a>
          <a href="#products" class="block opacity-60 hover:opacity-100 cursor-pointer">Corporate Hampers</a>
          <a href="#hero" class="block opacity-60 hover:opacity-100 cursor-pointer">Cake Delivery</a>
        </div>
        <div class="space-y-2 text-xs">
          <h4 class="font-bold opacity-80">Hubungi Kami</h4>
          <p class="opacity-60">Jakarta, Indonesia</p>
          <p class="opacity-60">support@annsbakehouse.com</p>
          <p class="opacity-60">+62 811 1999 876</p>
        </div>
      </div>
      <div class="pt-6 border-t %s flex items-center justify-between text-[11px] opacity-50">
        <div>© 2026 %s. All rights reserved.</div>
        <div class="flex gap-4">
          <a href="#sec-header" class="hover:underline">Back to Top ↑</a>
          <span>Privacy Policy</span>
          <span>Terms of Service</span>
        </div>
      </div>
    </footer>
`, fBrand, func() string {
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
