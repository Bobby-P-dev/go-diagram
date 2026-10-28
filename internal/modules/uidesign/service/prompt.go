package service

import (
	dtos "github.com/Bobby-P-dev/go-diagram.git/internal/modules/uidesign/dto"
	"strings"
)

func ResolveUIDesignThemeMode(req dtos.CreateUIDesignRequest) string {
	return resolveUIDesignThemeMode(req)
}

func BuildUIDesignPrompt(req dtos.CreateUIDesignRequest) string {
	return buildUIDesignPrompt(req)
}

// Preserve automatic styling until the design brief can interpret the user's prompt.
func resolveUIDesignThemeMode(req dtos.CreateUIDesignRequest) string {
	if mode := strings.ToLower(strings.TrimSpace(req.ThemeMode)); mode != "" {
		return mode
	}
	theme := strings.ToLower(strings.TrimSpace(req.Theme))
	if strings.Contains(theme, "light") {
		return "light"
	}
	if strings.Contains(theme, "dark") {
		return "dark"
	}
	return "auto"
}

func inferDesignIntent(prompt string, req *dtos.CreateUIDesignRequest) dtos.CompileOptions {
	pLower := strings.ToLower(prompt)
	opts := dtos.CompileOptions{
		Archetype:      req.Archetype,
		Density:        req.Density,
		ProductContext: req.ProductContext,
		PrimaryUser:    req.PrimaryUser,
		PrimaryTask:    req.PrimaryTask,
	}

	// Infer Archetype if not explicitly provided
	if opts.Archetype == "" {
		if strings.Contains(pLower, "no-code") || strings.Contains(pLower, "builder") || strings.Contains(pLower, "figjam") || strings.Contains(pLower, "canvas") || strings.Contains(pLower, "whiteboard") || strings.Contains(pLower, "miro") {
			opts.Archetype = "canvas_tool"
		} else if strings.Contains(pLower, "erp") || strings.Contains(pLower, "dashboard") || strings.Contains(pLower, "analytics") || strings.Contains(pLower, "procurement") || strings.Contains(pLower, "supply chain") || strings.Contains(pLower, "crm") || strings.Contains(pLower, "monitoring") || strings.Contains(pLower, "admin") {
			opts.Archetype = "dashboard"
		} else if strings.Contains(pLower, "pos") || strings.Contains(pLower, "kasir") || strings.Contains(pLower, "cashier") {
			opts.Archetype = "pos"
		} else if strings.Contains(pLower, "kanban") || strings.Contains(pLower, "scrum") || strings.Contains(pLower, "trello") {
			opts.Archetype = "kanban"
		} else if strings.Contains(pLower, "booking") || strings.Contains(pLower, "reservasi") || strings.Contains(pLower, "jadwal") || strings.Contains(pLower, "appointment") {
			opts.Archetype = "booking"
		} else if strings.Contains(pLower, "login") || strings.Contains(pLower, "auth") || strings.Contains(pLower, "masuk") || strings.Contains(pLower, "register") {
			opts.Archetype = "form"
		} else if strings.Contains(pLower, "landing") || strings.Contains(pLower, "homepage") || strings.Contains(pLower, "beranda") || strings.Contains(pLower, "studio") || strings.Contains(pLower, "agency") || strings.Contains(pLower, "portfolio") {
			opts.Archetype = "saas_landing"
		}
	}

	// Infer ProductContext if not explicitly provided
	if opts.ProductContext == "" {
		if strings.Contains(pLower, "erp") || strings.Contains(pLower, "procurement") || strings.Contains(pLower, "enterprise") || strings.Contains(pLower, "logistik") || strings.Contains(pLower, "inventory") {
			opts.ProductContext = "erp"
		} else if strings.Contains(pLower, "saas") || strings.Contains(pLower, "no-code") || strings.Contains(pLower, "builder") || strings.Contains(pLower, "platform") || strings.Contains(pLower, "devops") || strings.Contains(pLower, "cloud") || strings.Contains(pLower, "ai") {
			opts.ProductContext = "saas"
		} else if strings.Contains(pLower, "ecommerce") || strings.Contains(pLower, "e-commerce") || strings.Contains(pLower, "toko") || strings.Contains(pLower, "shop") || strings.Contains(pLower, "bakery") || strings.Contains(pLower, "kue") || strings.Contains(pLower, "pastry") || strings.Contains(pLower, "resto") || strings.Contains(pLower, "f&b") || strings.Contains(pLower, "coffee") || strings.Contains(pLower, "kopi") || strings.Contains(pLower, "fashion") {
			opts.ProductContext = "ecommerce"
		} else if strings.Contains(pLower, "fintech") || strings.Contains(pLower, "bank") || strings.Contains(pLower, "wallet") || strings.Contains(pLower, "dompet") || strings.Contains(pLower, "pembayaran") || strings.Contains(pLower, "payment") {
			opts.ProductContext = "fintech"
		} else if strings.Contains(pLower, "clinic") || strings.Contains(pLower, "klinik") || strings.Contains(pLower, "dokter") || strings.Contains(pLower, "hospital") || strings.Contains(pLower, "medis") || strings.Contains(pLower, "kesehatan") {
			opts.ProductContext = "healthcare"
		} else if strings.Contains(pLower, "arsitektur") || strings.Contains(pLower, "architecture") || strings.Contains(pLower, "editorial") || strings.Contains(pLower, "journal") || strings.Contains(pLower, "luxury") || strings.Contains(pLower, "studio") {
			opts.ProductContext = "editorial"
		}
	}

	// Infer Density if not explicitly provided
	if opts.Density == "" {
		if opts.Archetype == "dashboard" || opts.ProductContext == "erp" || opts.Archetype == "pos" {
			opts.Density = "compact"
		} else if opts.Archetype == "canvas_tool" || opts.Archetype == "saas_landing" {
			opts.Density = "balanced"
		} else if opts.ProductContext == "editorial" {
			opts.Density = "generous"
		}
	}

	// Preserve "auto" when user did not explicitly lock a foundation preset,
	// allowing the Art Director LLM to craft a 100% bespoke, prompt-driven visual language.
	if strings.TrimSpace(req.Foundation) == "" {
		req.Foundation = "auto"
	}

	return opts
}

func buildUIDesignPrompt(req dtos.CreateUIDesignRequest) string {
	prompt := strings.TrimSpace(req.Prompt)
	if prompt == "" {
		return ""
	}
	if tone := strings.TrimSpace(req.CustomTone); tone != "" {
		prompt += "\nRequested visual tone: " + tone
	}
	if context := strings.TrimSpace(req.ProductContext); context != "" {
		prompt += "\nProduct context: " + context
	}
	if arch := strings.TrimSpace(req.Archetype); arch != "" {
		prompt += "\nDesign archetype: " + arch
	}
	if density := strings.TrimSpace(req.Density); density != "" {
		prompt += "\nSpacing density: " + density
	}
	if user := strings.TrimSpace(req.PrimaryUser); user != "" {
		prompt += "\nPrimary user: " + user
	}
	if task := strings.TrimSpace(req.PrimaryTask); task != "" {
		prompt += "\nPrimary task: " + task
	}
	return prompt
}
