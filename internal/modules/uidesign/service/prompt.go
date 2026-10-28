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
	opts := dtos.CompileOptions{
		Archetype:      req.Archetype,
		Density:        req.Density,
		ProductContext: req.ProductContext,
		PrimaryUser:    req.PrimaryUser,
		PrimaryTask:    req.PrimaryTask,
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
