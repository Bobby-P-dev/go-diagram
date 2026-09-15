package services

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/Bobby-P-dev/go-diagram.git/src/dtos"
)

type DesignPlanner struct {
	aiService *AIService
}

func NewDesignPlanner(aiService *AIService) *DesignPlanner {
	return &DesignPlanner{
		aiService: aiService,
	}
}

const designPlannerPrompt = `You are an expert AI Design Planner & Frontend Architecture Expert.

YOUR RESPONSIBILITY:
Answer HOW the validated requirements should be presented.
Translate the Requirement Specification into a structured Design Specification using the Component Registry.

CRITICAL ARCHITECTURE RULES:
1. HIGH FREEDOM IN "HOW", LOW FREEDOM IN "WHAT":
   You have freedom in layout, spacing, visual hierarchy, and component arrangement.
   You have ZERO freedom to invent product functionality, business domains, authentication providers, or features.

2. OPTIONAL != REQUIRED (STRICT POLICY):
   Features in 'requirements.optional' MUST NOT BE INCLUDED unless explicitly listed in 'requirements.explicit'.
   COMMON UI PATTERN DOES NOT EQUAL USER REQUIREMENT.
   Never reason: "Most login pages have Google login, so I will add Google login."
   If the user did not explicitly request Google or GitHub login, do NOT add them.

3. PAGE-SPECIFIC STRUCTURAL CONSTRAINTS:
   - For LOGIN / AUTH PAGE:
     * Layout: "centered_auth" or "single_column".
     * Section type: "form" (or "auth_card").
     * Section data fields: ONLY email/username and password fields, plus primary submit button.
     * "social_buttons": [] (EMPTY array unless explicitly requested in requirements.explicit!).
     * "show_remember_me": false (unless in requirements.explicit).
     * "show_forgot_password": false (unless in requirements.explicit).
     * "switch_action": "" (empty unless registration was explicitly requested).
     * "subtitle": Neutral copy ("Masuk ke akun Anda" or "Silakan masukkan kredensial Anda"). NEVER assume product domain.
   - For HOMEPAGE:
     * Standard sections: "navbar" + "hero" + "feature_grid".
     * DO NOT add "kpi_grid", financial metrics, pricing tables, testimonials, or dashboards unless explicitly requested.

4. DESIGN FREEDOM ADHERENCE:
   - If design_freedom.visual is "low" (simple/minimal): Use restrained, clean spacing, minimum necessary sections (2-3), no visual clutter.
   - If design_freedom.visual is "high" (creative/experimental): Use bold typography and creative card arrangements, but KEEP FUNCTIONAL SCOPE STRICTLY CONSERVATIVE.

5. COMPLEXITY DISCIPLINE:
   - "simple": 2 to 3 sections maximum. Low visual density, zero unnecessary decorative blocks.
   - "moderate": 3 to 4 sections with moderate interactivity.
   - "complex": Only if explicitly required by multi-workflow domain.

6. REQUIREMENT TRACEABILITY:
   Every section MUST have a valid "requirement_source" linking directly to a requirement:
   e.g. "requirement_source": "primary_goal", "requirement_source": "implied:credential_auth", "requirement_source": "explicit:xxx".
   If a section has no source, DO NOT INCLUDE IT.

7. COMPONENT REGISTRY:
   Valid section types:
   - "navbar": header navigation with brand, links, and action button.
   - "hero": headline, subtitle, primary/secondary action buttons.
   - "feature_grid": 2-4 value propositions with icons and descriptions.
   - "data_table": structured data records with columns and rows.
   - "form": login or input form with fields and submit button.
   - "kpi_grid": metric summary cards (only if explicitly requested for dashboard!).
   - "product_grid": catalog items with pricing.
   - "pricing_table": subscription plans (only if explicitly requested).

OUTPUT JSON SCHEMA:
{
  "page": {
    "type": "string",
    "purpose": "string",
    "complexity": "simple | moderate | complex"
  },
  "layout": {
    "type": "single_column | dashboard_split | master_detail | centered_auth",
    "container": "max-w-6xl | max-w-4xl | max-w-md | w-full",
    "alignment": "left | center",
    "spacing": "4-64px calibrated"
  },
  "visual": {
    "style": "clean | enterprise | soft_minimal",
    "density": "compact | balanced | generous",
    "theme": "string"
  },
  "typography": {
    "heading": "text-2xl or text-3xl font-bold tracking-tight",
    "body": "text-sm text-slate-600 leading-relaxed",
    "metadata": "text-xs font-mono text-slate-400"
  },
  "sections": [
    {
      "id": "string (e.g. sec-navbar, sec-auth, sec-hero)",
      "type": "navbar | hero | feature_grid | data_table | form | kpi_grid | pricing_table | product_grid",
      "purpose": "string explaining why this section exists",
      "priority": "high | medium | low",
      "requirement_source": "primary_goal | explicit:xxx | implied:xxx",
      "data": {
        // Structured props for the section component.
        // For form: title, subtitle, fields: [{label, name, type, placeholder}], submit_label, social_buttons: [], show_remember_me: false, show_forgot_password: false
      }
    }
  ],
  "responsive": {
    "desktop": "1024x720 multi-column",
    "tablet": "fluid responsive",
    "mobile": "stacked single column"
  },
  "states": ["default", "hover", "active"]
}

OUTPUT VALID JSON ONLY. NO MARKDOWN, NO COMMENTARY.`

func (p *DesignPlanner) Plan(
	ctx context.Context,
	reqSpec *dtos.RequirementSpecificationDTO,
	device string,
	foundation string,
	themeMode string,
) (*dtos.DesignSpecificationDTO, error) {
	reqSpecBytes, err := json.MarshalIndent(reqSpec, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal requirement spec: %w", err)
	}

	userMessage := fmt.Sprintf(`VALIDATED REQUIREMENT SPECIFICATION:
%s

TARGET DEVICE: %s
FOUNDATION: %s
THEME MODE: %s

Plan the design specification strictly following the rules. Every section must have a requirement_source. Output valid JSON matching the schema.`, string(reqSpecBytes), device, foundation, themeMode)

	respText, err := p.aiService.CallLLM(designPlannerPrompt, nil, userMessage)
	if err != nil {
		return nil, fmt.Errorf("design planner LLM call failed: %w", err)
	}

	cleanJSON := p.aiService.SanitizeJSON(respText)
	var designSpec dtos.DesignSpecificationDTO
	if err := json.Unmarshal([]byte(cleanJSON), &designSpec); err != nil {
		return nil, fmt.Errorf("failed to parse design specification JSON: %w: %s", err, cleanJSON)
	}

	// Populate Canonical Stable Component IDs and Traceability
	for sIdx := range designSpec.Sections {
		sec := &designSpec.Sections[sIdx]
		if sec.ID == "" {
			sec.ID = fmt.Sprintf("sec-%s-%d", sec.Type, sIdx+1)
		}

		// Ensure RequirementSourceObj is set
		if sec.RequirementSourceObj == nil {
			sourceType := "design"
			sourceID := "primary_goal"
			if strings.Contains(sec.RequirementSource, "explicit") {
				sourceType = "explicit"
				sourceID = "req-explicit-1"
			} else if strings.Contains(sec.RequirementSource, "implied") || sec.Type == "form" {
				sourceType = "implied"
				sourceID = "req-auth-inputs"
			}
			sec.RequirementSourceObj = &dtos.RequirementSourceDTO{
				Type: sourceType,
				ID:   sourceID,
			}
		}

		// Synthesize Canonical Components for Section
		if len(sec.Components) == 0 && sec.Data != nil {
			switch sec.Type {
			case "form", "auth_card", "login_card":
				if rawFields, ok := sec.Data["fields"].([]interface{}); ok {
					for fIdx, rf := range rawFields {
						if fMap, ok := rf.(map[string]interface{}); ok {
							name, _ := fMap["name"].(string)
							label, _ := fMap["label"].(string)
							fType, _ := fMap["type"].(string)
							if name == "" {
								name = fmt.Sprintf("field_%d", fIdx+1)
							}
							sec.Components = append(sec.Components, dtos.UIComponentDTO{
								ID:      fmt.Sprintf("cmp-%s-input", name),
								Type:    fType,
								Name:    name,
								Label:   label,
								Purpose: fmt.Sprintf("Collect user %s", label),
								RequirementSource: &dtos.RequirementSourceDTO{
									Type: "implied",
									ID:   "req-auth-inputs",
								},
							})
						}
					}
				}
				// Submit action button
				subLabel, _ := sec.Data["submit_label"].(string)
				if subLabel == "" {
					subLabel = "Submit"
				}
				sec.Components = append(sec.Components, dtos.UIComponentDTO{
					ID:      "cmp-login-button",
					Type:    "button",
					Name:    "submit",
					Label:   subLabel,
					Purpose: "Submit authentication credentials",
					RequirementSource: &dtos.RequirementSourceDTO{
						Type: "implied",
						ID:   "req-auth-inputs",
					},
					Style: map[string]interface{}{"size": "standard", "variant": "primary"},
				})

			case "navbar":
				brand, _ := sec.Data["brand"].(string)
				sec.Components = append(sec.Components,
					dtos.UIComponentDTO{
						ID:      "cmp-brand-logo",
						Type:    "brand",
						Label:   brand,
						Purpose: "Display brand identity and application name",
						RequirementSource: &dtos.RequirementSourceDTO{Type: "design", ID: "branding"},
					},
					dtos.UIComponentDTO{
						ID:      "cmp-nav-links",
						Type:    "navigation",
						Purpose: "Provide primary section navigation",
						RequirementSource: &dtos.RequirementSourceDTO{Type: "design", ID: "navigation"},
					},
					dtos.UIComponentDTO{
						ID:      "cmp-nav-cta",
						Type:    "button",
						Label:   "Get Started",
						Purpose: "Top-level call to action",
						RequirementSource: &dtos.RequirementSourceDTO{Type: "design", ID: "cta"},
					},
				)

			case "hero":
				title, _ := sec.Data["title"].(string)
				sub, _ := sec.Data["subtitle"].(string)
				sec.Components = append(sec.Components,
					dtos.UIComponentDTO{
						ID:      "cmp-hero-title",
						Type:    "heading",
						Label:   title,
						Purpose: "Communicate core value proposition",
						RequirementSource: &dtos.RequirementSourceDTO{Type: "explicit", ID: "req-title"},
					},
					dtos.UIComponentDTO{
						ID:      "cmp-hero-subtitle",
						Type:    "text",
						Label:   sub,
						Purpose: "Elaborate headline with supportive copy",
						RequirementSource: &dtos.RequirementSourceDTO{Type: "explicit", ID: "req-subtitle"},
					},
					dtos.UIComponentDTO{
						ID:      "cmp-hero-cta-primary",
						Type:    "button",
						Label:   "Mulai Sekarang",
						Purpose: "Primary page action target",
						RequirementSource: &dtos.RequirementSourceDTO{Type: "design", ID: "primary_cta"},
					},
				)

			default:
				sec.Components = append(sec.Components, dtos.UIComponentDTO{
					ID:      fmt.Sprintf("cmp-%s-main", sec.Type),
					Type:    sec.Type,
					Purpose: fmt.Sprintf("Render %s section content", sec.Type),
					RequirementSource: &dtos.RequirementSourceDTO{Type: "design", ID: "section_content"},
				})
			}
		}
	}

	// Populate Canonical Structured Design Decisions
	designSpec.DesignDecisions = []dtos.DesignDecisionItemDTO{
		{
			ID:       "dec-001",
			Decision: fmt.Sprintf("Layout: %s", designSpec.Layout.Type),
			Reason:   fmt.Sprintf("Optimal presentation for %s page purpose", designSpec.Page.Purpose),
			Source:   "design_system",
		},
		{
			ID:       "dec-002",
			Decision: fmt.Sprintf("Visual Style: %s with theme mode %s", designSpec.Visual.Style, themeMode),
			Reason:   "Adheres to user visual preference without ornamental clutter",
			Source:   "visual_interpretation",
		},
		{
			ID:       "dec-003",
			Decision: fmt.Sprintf("Bounded Complexity: %s (%d sections)", designSpec.Page.Complexity, len(designSpec.Sections)),
			Reason:   "Eliminates unrequested feature drift and preserves design focus",
			Source:   "anti_slop_guard",
		},
	}

	return &designSpec, nil
}
