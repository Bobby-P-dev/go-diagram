package services

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/Bobby-P-dev/go-diagram.git/src/dtos"
)

type RequirementAnalyzer struct {
	aiService *AIService
}

func NewRequirementAnalyzer(aiService *AIService) *RequirementAnalyzer {
	return &RequirementAnalyzer{
		aiService: aiService,
	}
}

const requirementAnalyzerPrompt = `You are a strict, objective AI Requirement Analyzer for an AI Design Compiler.

YOUR ONLY RESPONSIBILITY IS TO UNDERSTAND WHAT THE USER ACTUALLY WANTS.
DO NOT DESIGN THE UI. DO NOT WRITE HTML. DO NOT INVENT PRODUCT FEATURES.

CORE PRINCIPLE: HIGH FREEDOM IN "HOW", LOW FREEDOM IN "WHAT".
AI MAY BE CREATIVE ABOUT: layout, spacing, typography, visual hierarchy, interaction presentation.
AI MUST NOT INVENT: product functionality, business domains, authentication providers, business workflows, statistics, company names, or features.

MANDATORY RULES (RULES 1 - 12):
RULE 1: Never invent a business domain.
If the user says "buat login page minimalis" or "buat homepage sederhana", domain MUST be null.
Do NOT assume finance, ERP, SaaS, healthcare, e-commerce, or crypto unless the user explicitly mentions it.

RULE 2: COMMON UI PATTERN DOES NOT EQUAL USER REQUIREMENT.
Never reason: "Most login pages have Google login, therefore Google login should be added."
Instead: "Google login is a common optional pattern. The user did not request it. Therefore it is OPTIONAL and must NOT be required."

RULE 3: OPTIONAL != REQUIRED.
Optional features MUST NEVER be placed in 'explicit' or 'implied'. They belong ONLY in 'optional'.

RULE 4: Separate visual style from product function and page composition:
- "modern" is a visual direction, NOT SaaS or startup or dashboard.
- "elegan" is an aesthetic treatment (refined typography, generous whitespace, understated luxury), NOT a corporate suite.
- "minimal" / "minimalis" is an aesthetic visual treatment (clean typography, generous whitespace, subtle borders, uncluttered layout), NOT a command to delete or prune page sections. A minimal page still preserves the requested content and tasks; section count follows the brief.

RULE 5: Never invent business functionality (no treasury, no OCR, no virtual cards, no trading, no workflows).
RULE 6: Never invent statistics, metrics, or revenue.
RULE 7: Never invent company names or fictitious brands. If a URL or brand is mentioned in prompt (e.g. "https://annsbakehouse.com/"), extract and respect that brand and domain!
RULE 8: 3-AXIS FREEDOM MODEL:
- functional_freedom is "low": do NOT invent extra business workflows.
- visual_freedom is "high": explore typography, contrast, spacing, color palette.
- composition_freedom is "high": choose section pacing from the page purpose instead of requiring a landing-page sequence.
RULE 9: "creative" / "experimental" gives visual freedom (design_freedom.visual: "high"), but NEVER authorizes inventing unrequested business workflows (functional freedom remains "low").
RULE 10: For unknown products, use neutral content ("Masuk ke akun Anda", NOT "Kelola seluruh workflow visual Anda").
RULE 11: When information is missing, preserve the uncertainty (keep fields null or empty).
RULE 12: PAGE TYPE STRUCTURAL BOUNDS:
- LOGIN:
  * Explicit: Only what user said (e.g. "login page", "minimalis", "elegan").
  * Implied: ONLY credential input (email/username), password input, and submit action.
  * Optional: Google login, GitHub login, remember me, forgot password, registration. DO NOT make these implied!
- HOMEPAGE / LANDING:
  * Infer only the composition needed to communicate the stated purpose; no fixed section count or sequence.
  * Do not fabricate reviews, numerical claims or products merely to populate common section patterns.

OUTPUT JSON SCHEMA:
{
  "raw_prompt": "string (verbatim original prompt)",
  "page": {
    "type": "string (homepage | landing | dashboard | login | checkout | detail | profile | form | generic)",
    "purpose": "string (concise summary of core user purpose)",
    "complexity": "string (moderate | complex | simple)"
  },
  "context": {
    "domain": null, // string or null. Extracted from prompt or URL, else null.
    "target_user": null // string or null.
  },
  "goals": {
    "primary": "string (the single main objective)",
    "secondary": [] // array of secondary goals
  },
  "requirements": {
    "explicit": [], // array of strictly explicit requirements from prompt
    "implied": [],  // array of strictly necessary functional foundations
    "optional": []  // array of optional ideas (Google login, GitHub, Remember me, etc.)
  },
  "design_freedom": {
    "functional": "low", // always "low" unless user explicitly requested complex multi-workflow
    "visual": "high", // "high" for refined aesthetic exploration
    "composition": "high" // "high" for rich storytelling layout
  },
  "constraints": [], // array of constraints
  "visual_preferences": [], // array of strings
  "responsive": true
}

OUTPUT VALID JSON ONLY. NO MARKDOWN, NO COMMENTARY.`

func (a *RequirementAnalyzer) Analyze(ctx context.Context, rawPrompt string, device string, foundation string) (*dtos.RequirementSpecificationDTO, error) {
	trimmedPrompt := strings.TrimSpace(rawPrompt)
	if trimmedPrompt == "" {
		return &dtos.RequirementSpecificationDTO{
			RawPrompt: "",
			Page: dtos.RequirementPageDTO{
				Type:       "generic",
				Purpose:    "Kanvas antarmuka kosong",
				Complexity: "simple",
			},
			Context: dtos.RequirementContextDTO{
				Domain:     nil,
				TargetUser: nil,
			},
			Goals: dtos.RequirementGoalsDTO{
				Primary: "Kanvas UI baru siap dirancang",
			},
			Requirements: dtos.RequirementsListDTO{
				Explicit: []string{},
				Implied:  []string{},
				Optional: []string{},
			},
			DesignFreedom: &dtos.DesignFreedomDTO{
				Functional: "low",
				Visual:     "low",
			},
			Constraints:       []string{"blank_canvas"},
			VisualPreferences: []string{"clean"},
			Responsive:        true,
		}, nil
	}

	userMessage := fmt.Sprintf(`USER PROMPT: %s
TARGET DEVICE: %s
DESIGN FOUNDATION: %s

Analyze this prompt strictly according to Rules 1-12.
Remember: COMMON UI PATTERN DOES NOT EQUAL USER REQUIREMENT.
Output JSON matching the schema.`, trimmedPrompt, device, foundation)

	respText, err := a.aiService.CallLLM(requirementAnalyzerPrompt, nil, userMessage)
	if err != nil {
		return nil, fmt.Errorf("requirement analysis LLM call failed: %w", err)
	}

	cleanJSON := a.aiService.SanitizeJSON(respText)
	var spec dtos.RequirementSpecificationDTO
	if err := json.Unmarshal([]byte(cleanJSON), &spec); err != nil {
		return nil, fmt.Errorf("failed to parse requirement specification JSON: %w: %s", err, cleanJSON)
	}

	// Always preserve raw_prompt
	spec.RawPrompt = trimmedPrompt
	return enforceRequirementRules(&spec), nil
}

// enforceRequirementRules applies deterministic requirement guards that do not
// need another LLM call: design-freedom defaults, minimal/vivid keyword mapping,
// login optional-feature scrubbing, and canonical structured requirement items.
// Shared by the live analyzing stage and the single-call synthesis path.
func enforceRequirementRules(spec *dtos.RequirementSpecificationDTO) *dtos.RequirementSpecificationDTO {
	// Deterministic rule enforcement
	lowerPrompt := strings.ToLower(spec.RawPrompt)

	// Ensure DesignFreedom is initialized
	if spec.DesignFreedom == nil {
		spec.DesignFreedom = &dtos.DesignFreedomDTO{
			Functional: "low",
			Visual:     "medium",
		}
	} else {
		// Functional freedom is always conservatively low unless explicitly asked
		spec.DesignFreedom.Functional = "low"
	}

	// Domain grounding check: If prompt does not mention any explicit business domain,
	// keep domain null to prevent hallucinated domain anchoring on generic prompts.
	domainKeywords := []string{
		"erp", "procurement", "kue", "bakery", "pastry", "makanan", "food", "restoran", "restaurant",
		"tari", "dance", "klinik", "clinic", "hospital", "rumah sakit", "sekolah", "school",
		"universitas", "toko", "shop", "ecommerce", "e-commerce", "buku", "book", "fashion",
		"hotel", "travel", "properti", "real estate", "game", "gaming", "fitness", "gym",
		"crypto", "bank", "fintech", "pos", "kasir", "point of sale", "logistik", "logistic",
	}
	hasDomainKeyword := false
	for _, dk := range domainKeywords {
		if strings.Contains(lowerPrompt, dk) {
			hasDomainKeyword = true
			break
		}
	}
	if !hasDomainKeyword {
		spec.Context.Domain = nil
		prohibited := []string{"treasury", "cfo", "virtual card", "credit limit", "crypto", "blockchain"}
		for _, bad := range prohibited {
			re := regexp.MustCompile(fmt.Sprintf(`(?i)\b%s\b`, regexp.QuoteMeta(bad)))
			spec.Goals.Primary = re.ReplaceAllString(spec.Goals.Primary, "feature")
			for i := range spec.Goals.Secondary {
				spec.Goals.Secondary[i] = re.ReplaceAllString(spec.Goals.Secondary[i], "feature")
			}
			for i := range spec.Requirements.Explicit {
				spec.Requirements.Explicit[i] = re.ReplaceAllString(spec.Requirements.Explicit[i], "feature")
			}
			for i := range spec.Requirements.Implied {
				spec.Requirements.Implied[i] = re.ReplaceAllString(spec.Requirements.Implied[i], "feature")
			}
			for i := range spec.Requirements.Optional {
				spec.Requirements.Optional[i] = re.ReplaceAllString(spec.Requirements.Optional[i], "feature")
			}
		}
	}

	if strings.Contains(lowerPrompt, "sederhana") || strings.Contains(lowerPrompt, "simple") {
		spec.Page.Complexity = "simple"
	}

	// Minimal describes an aesthetic, not missing content. Preserve the model's
	// grounded domain and composition instead of narrowing to a keyword whitelist.
	if strings.Contains(lowerPrompt, "kreatif") || strings.Contains(lowerPrompt, "creative") || strings.Contains(lowerPrompt, "experimental") || strings.Contains(lowerPrompt, "bold") {
		spec.DesignFreedom.Visual = "high"
	}

	// Login & Auth Specific Enforcement:
	isLogin := spec.Page.Type == "login" || strings.Contains(lowerPrompt, "login") || strings.Contains(lowerPrompt, "masuk")
	if isLogin {
		spec.Page.Type = "login"
		// If user didn't explicitly request social logins, ensure they are NOT in explicit or implied
		hasGoogleReq := strings.Contains(lowerPrompt, "google")
		hasGithubReq := strings.Contains(lowerPrompt, "github")
		hasSocialReq := strings.Contains(lowerPrompt, "social") || strings.Contains(lowerPrompt, "sso")

		if !hasGoogleReq && !hasGithubReq && !hasSocialReq {
			cleanExplicit := make([]string, 0)
			for _, exp := range spec.Requirements.Explicit {
				expLower := strings.ToLower(exp)
				if !strings.Contains(expLower, "google") && !strings.Contains(expLower, "github") && !strings.Contains(expLower, "social") {
					cleanExplicit = append(cleanExplicit, exp)
				}
			}
			spec.Requirements.Explicit = cleanExplicit

			cleanImplied := make([]string, 0)
			for _, imp := range spec.Requirements.Implied {
				impLower := strings.ToLower(imp)
				if !strings.Contains(impLower, "google") && !strings.Contains(impLower, "github") && !strings.Contains(impLower, "social") && !strings.Contains(impLower, "remember") && !strings.Contains(impLower, "register") {
					cleanImplied = append(cleanImplied, imp)
				}
			}
			spec.Requirements.Implied = cleanImplied
			spec.Requirements.Optional = append(spec.Requirements.Optional, "Google Login", "GitHub Login", "Remember Me", "Forgot Password", "Registration")
		} else {
			// User explicitly asked for social authentication: ensure explicitly tracked
			if hasGoogleReq {
				foundG := false
				for _, exp := range spec.Requirements.Explicit {
					if strings.Contains(strings.ToLower(exp), "google") {
						foundG = true
						break
					}
				}
				if !foundG {
					spec.Requirements.Explicit = append(spec.Requirements.Explicit, "Google Login")
				}
			}
			if hasGithubReq {
				foundGh := false
				for _, exp := range spec.Requirements.Explicit {
					if strings.Contains(strings.ToLower(exp), "github") {
						foundGh = true
						break
					}
				}
				if !foundGh {
					spec.Requirements.Explicit = append(spec.Requirements.Explicit, "GitHub Login")
				}
			}
		}
	}

	// Populate Canonical Structured Requirements with Stable IDs
	spec.Requirements.StructuredExplicit = make([]dtos.StructuredRequirementItemDTO, 0, len(spec.Requirements.Explicit))
	for idx, exp := range spec.Requirements.Explicit {
		id := fmt.Sprintf("req-explicit-%d", idx+1)
		expLower := strings.ToLower(exp)
		if strings.Contains(expLower, "login") {
			id = "req-page-login"
		} else if strings.Contains(expLower, "minimal") {
			id = "req-style-minimal"
		} else if strings.Contains(expLower, "elegan") || strings.Contains(expLower, "elegant") {
			id = "req-style-elegant"
		} else if strings.Contains(expLower, "modern") {
			id = "req-style-modern"
		} else if strings.Contains(expLower, "google") {
			id = "req-auth-google"
		} else if strings.Contains(expLower, "github") {
			id = "req-auth-github"
		}
		spec.Requirements.StructuredExplicit = append(spec.Requirements.StructuredExplicit, dtos.StructuredRequirementItemDTO{
			ID:          id,
			Description: exp,
			Confidence:  1.0,
		})
	}

	spec.Requirements.StructuredImplied = make([]dtos.StructuredRequirementItemDTO, 0, len(spec.Requirements.Implied))
	for idx, imp := range spec.Requirements.Implied {
		id := fmt.Sprintf("req-implied-%d", idx+1)
		impLower := strings.ToLower(imp)
		if strings.Contains(impLower, "input") || strings.Contains(impLower, "credential") || strings.Contains(impLower, "password") {
			id = "req-auth-inputs"
		}
		spec.Requirements.StructuredImplied = append(spec.Requirements.StructuredImplied, dtos.StructuredRequirementItemDTO{
			ID:          id,
			Description: imp,
			Confidence:  0.98,
		})
	}

	spec.Requirements.StructuredOptional = make([]dtos.StructuredRequirementItemDTO, 0, len(spec.Requirements.Optional))
	for idx, opt := range spec.Requirements.Optional {
		id := fmt.Sprintf("opt-%d", idx+1)
		optLower := strings.ToLower(opt)
		if strings.Contains(optLower, "google") {
			id = "opt-google-login"
		} else if strings.Contains(optLower, "github") {
			id = "opt-github-login"
		} else if strings.Contains(optLower, "forgot") {
			id = "opt-forgot-password"
		} else if strings.Contains(optLower, "remember") {
			id = "opt-remember-me"
		} else if strings.Contains(optLower, "register") {
			id = "opt-registration"
		}
		spec.Requirements.StructuredOptional = append(spec.Requirements.StructuredOptional, dtos.StructuredRequirementItemDTO{
			ID:          id,
			Description: opt,
			Confidence:  0.7,
		})
	}

	spec.Explicit = spec.Requirements.StructuredExplicit
	spec.Implied = spec.Requirements.StructuredImplied
	spec.Optional = spec.Requirements.StructuredOptional
	spec.Excluded = spec.Requirements.StructuredExcluded

	return spec
}
