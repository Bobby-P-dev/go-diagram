package service

import (
	"encoding/json"
	"strings"

	dtos "github.com/Bobby-P-dev/go-diagram.git/internal/modules/uidesign/dto"
)

type UIValidator struct{}

func NewUIValidator() *UIValidator {
	return &UIValidator{}
}

// Prohibited domain keywords when domain is unspecified or generic
var hallucinatedDomainKeywords = []string{
	"treasury",
	"cfo",
	"virtual card",
	"credit limit",
	"wire transfer",
	"crypto wallet",
	"crypto",
	"blockchain",
	"ocr scan",
	"invoice factoring",
	"fictional company",
}

func (v *UIValidator) ValidateAndAudit(
	reqSpec *dtos.RequirementSpecificationDTO,
	designSpec *dtos.DesignSpecificationDTO,
) (*dtos.UIValidationResultDTO, *dtos.DesignSpecificationDTO) {
	if designSpec == nil {
		return &dtos.UIValidationResultDTO{
			Status: "fail",
			Score: dtos.ValidationScoreDTO{
				RequirementFidelity: 1,
				ScopeAccuracy:       1,
				Traceability:        1,
				Simplicity:          1,
				Hierarchy:           1,
				VisualConsistency:   1,
				ResponsiveQuality:   1,
				HallucinationSafety: 1,
				Scope:               1,
				Consistency:         1,
			},
			Issues:             []string{"Design specification is nil"},
			HallucinationCheck: "failed: empty design specification",
		}, designSpec
	}

	issues := make([]string, 0)
	strippedSections := make([]string, 0)
	structuredIssues := make([]dtos.ValidationIssueDTO, 0)
	repairedSections := make([]dtos.UISectionDTO, 0)

	isDomainSpecified := reqSpec != nil && reqSpec.Context.Domain != nil && strings.TrimSpace(*reqSpec.Context.Domain) != ""
	isSimpleComplexity := reqSpec != nil && strings.ToLower(reqSpec.Page.Complexity) == "simple"

	// Helper to check if keyword is in explicit requirements or raw prompt or primary goal
	isExplicitlyRequested := func(keyword string) bool {
		if reqSpec == nil {
			return false
		}
		kwLower := strings.ToLower(keyword)
		if strings.Contains(strings.ToLower(reqSpec.RawPrompt), kwLower) {
			return true
		}
		if strings.Contains(strings.ToLower(reqSpec.Goals.Primary), kwLower) {
			return true
		}
		for _, exp := range reqSpec.Requirements.Explicit {
			if strings.Contains(strings.ToLower(exp), kwLower) {
				return true
			}
		}
		return false
	}

	for _, sec := range designSpec.Sections {
		// Repair decorative auth copy before domain detection so a valid form is
		// not discarded because its subtitle drifted into an unrelated product.
		if !isDomainSpecified && sec.Data != nil && (sec.Type == "form" || sec.Type == "auth_card" || sec.Type == "login_card") {
			if subtitle, ok := sec.Data["subtitle"].(string); ok {
				for _, keyword := range hallucinatedDomainKeywords {
					if strings.Contains(strings.ToLower(subtitle), keyword) && !isExplicitlyRequested(keyword) {
						sec.Data["subtitle"] = "Masuk ke akun Anda."
						issues = append(issues, "Neutralized unrelated domain copy in "+sec.ID)
						structuredIssues = append(structuredIssues, dtos.ValidationIssueDTO{Type: "content_drift", Component: sec.ID + ".subtitle", Action: "neutralize", Reason: "unrequested domain in authentication copy"})
						break
					}
				}
			}
		}
		secBytes, _ := json.Marshal(sec)
		secStr := strings.ToLower(string(secBytes))

		// 1. Traceability Check: Section must have a requirement source
		if strings.TrimSpace(sec.RequirementSource) == "" {
			issues = append(issues, "Section '"+sec.ID+"' has no requirement_source; stripped to prevent requirement drift")
			strippedSections = append(strippedSections, sec.ID+": missing traceability")
			structuredIssues = append(structuredIssues, dtos.ValidationIssueDTO{
				Type:      "traceability_drift",
				Component: sec.ID,
				Action:    "remove",
				Reason:    "missing requirement_source",
			})
			continue
		}

		// 2. Hallucination Check: If domain is NOT specified by user, check for unrequested financial/CFO/crypto bloat
		hasHallucination := false
		if !isDomainSpecified {
			for _, kw := range hallucinatedDomainKeywords {
				if strings.Contains(secStr, kw) && !isExplicitlyRequested(kw) {
					hasHallucination = true
					issues = append(issues, "Detected and stripped unrequested domain feature: '"+kw+"' in section '"+sec.Type+"'")
					strippedSections = append(strippedSections, sec.ID+": "+kw)
					structuredIssues = append(structuredIssues, dtos.ValidationIssueDTO{
						Type:      "domain_drift",
						Component: sec.ID,
						Action:    "remove",
						Reason:    "unrequested domain keyword '" + kw + "' when domain is unspecified",
					})
					break
				}
			}
		}

		if hasHallucination {
			continue // Strip this entire section!
		}

		// 3. Unrequested Marketing Section Guard (Anti-Slop R-18, R-28, R-36, R-38):
		// Testimonials, pricing tables, generic FAQ, or logo clouds must NEVER be added unless explicitly requested in the brief!
		sTypeLower := strings.ToLower(sec.Type)
		sIDLower := strings.ToLower(sec.ID)
		isMarketingSection := sTypeLower == "testimonials" || sTypeLower == "testimonial" || sTypeLower == "reviews" ||
			sTypeLower == "pricing" || sTypeLower == "pricing_table" || sTypeLower == "faq" || sTypeLower == "logo_cloud" ||
			strings.Contains(sIDLower, "testimonial") || strings.Contains(sIDLower, "review") ||
			strings.Contains(sIDLower, "pricing") || strings.Contains(sIDLower, "faq")

		if isMarketingSection {
			isRequested := isExplicitlyRequested("testimonial") || isExplicitlyRequested("review") ||
				isExplicitlyRequested("pricing") || isExplicitlyRequested("harga") || isExplicitlyRequested("paket") ||
				isExplicitlyRequested("faq") || isExplicitlyRequested("tanya") || isExplicitlyRequested("logo")
			if !isRequested {
				issues = append(issues, "Detected and stripped unrequested generic marketing section: '"+sec.Type+"' ('"+sec.ID+"') per Anti-Slop R-18/R-28/R-36 rules")
				strippedSections = append(strippedSections, sec.ID+": unrequested_marketing_section")
				structuredIssues = append(structuredIssues, dtos.ValidationIssueDTO{
					Type:      "unrequested_marketing_drift",
					Component: sec.ID,
					Action:    "remove",
					Reason:    "unrequested marketing section (" + sec.Type + ") violates Anti-Slop pure-context rule",
				})
				continue // Strip this entire unrequested section!
			}
		}

		// 3. Optional Feature Drift Guard & Intra-Section Feature Sanitization
		if sec.Data != nil {
			// For Auth / Form Sections:
			if sec.Type == "form" || sec.Type == "auth_card" || sec.Type == "login_card" {
				// Social buttons audit: only keep if explicitly requested
				hasGoogle := isExplicitlyRequested("google")
				hasGithub := isExplicitlyRequested("github")
				hasSocial := isExplicitlyRequested("social") || isExplicitlyRequested("sso")

				if !hasGoogle && !hasGithub && !hasSocial {
					if sb, ok := sec.Data["social_buttons"]; ok {
						if sbList, isList := sb.([]interface{}); isList && len(sbList) > 0 {
							sec.Data["social_buttons"] = []string{}
							issues = append(issues, "Stripped unrequested social login buttons (Google/GitHub) from "+sec.ID)
							structuredIssues = append(structuredIssues, dtos.ValidationIssueDTO{
								Type:      "optional_feature_drift",
								Component: sec.ID + ".social_buttons",
								Action:    "remove",
								Reason:    "social login is an optional feature without explicit user request",
							})
						} else if sbStrList, isStrList := sb.([]string); isStrList && len(sbStrList) > 0 {
							sec.Data["social_buttons"] = []string{}
							issues = append(issues, "Stripped unrequested social login buttons from "+sec.ID)
							structuredIssues = append(structuredIssues, dtos.ValidationIssueDTO{
								Type:      "optional_feature_drift",
								Component: sec.ID + ".social_buttons",
								Action:    "remove",
								Reason:    "social login is an optional feature without explicit user request",
							})
						}
					}
				} else {
					// Social login was explicitly requested: normalize to typed []string slice
					socialList := make([]string, 0)
					if sb, ok := sec.Data["social_buttons"]; ok {
						if sbList, isList := sb.([]interface{}); isList {
							for _, item := range sbList {
								if s, ok := item.(string); ok && strings.TrimSpace(s) != "" {
									socialList = append(socialList, strings.TrimSpace(s))
								}
							}
						} else if sbStrList, isStrList := sb.([]string); isStrList {
							socialList = append(socialList, sbStrList...)
						}
					}

					// Ensure requested providers exist
					if hasGoogle {
						hasG := false
						for _, s := range socialList {
							if strings.Contains(strings.ToLower(s), "google") {
								hasG = true
								break
							}
						}
						if !hasG {
							socialList = append(socialList, "Google")
						}
					}
					if hasGithub {
						hasGh := false
						for _, s := range socialList {
							if strings.Contains(strings.ToLower(s), "github") {
								hasGh = true
								break
							}
						}
						if !hasGh {
							socialList = append(socialList, "GitHub")
						}
					}
					sec.Data["social_buttons"] = socialList
				}

				// Remember me audit: only keep if explicitly requested
				if !isExplicitlyRequested("remember") && !isExplicitlyRequested("ingat") {
					if rm, ok := sec.Data["show_remember_me"].(bool); ok && rm {
						sec.Data["show_remember_me"] = false
						issues = append(issues, "Disabled unrequested remember-me checkbox in "+sec.ID)
						structuredIssues = append(structuredIssues, dtos.ValidationIssueDTO{
							Type:      "optional_feature_drift",
							Component: sec.ID + ".show_remember_me",
							Action:    "remove",
							Reason:    "remember me is an optional feature without explicit user request",
						})
					}
				}

				// Forgot password audit: only keep if explicitly requested
				if !isExplicitlyRequested("forgot") && !isExplicitlyRequested("lupa") && !isExplicitlyRequested("recovery") {
					if fp, ok := sec.Data["show_forgot_password"].(bool); ok && fp {
						sec.Data["show_forgot_password"] = false
						issues = append(issues, "Disabled unrequested forgot-password link in "+sec.ID)
						structuredIssues = append(structuredIssues, dtos.ValidationIssueDTO{
							Type:      "optional_feature_drift",
							Component: sec.ID + ".show_forgot_password",
							Action:    "remove",
							Reason:    "password recovery is an optional feature without explicit user request",
						})
					}
				}

				// Registration switch audit: only keep if explicitly requested
				if !isExplicitlyRequested("daftar") && !isExplicitlyRequested("register") && !isExplicitlyRequested("sign up") {
					if sa, ok := sec.Data["switch_action"].(string); ok && sa != "" {
						sec.Data["switch_action"] = ""
						sec.Data["switch_text"] = ""
						issues = append(issues, "Disabled unrequested registration switch link in "+sec.ID)
						structuredIssues = append(structuredIssues, dtos.ValidationIssueDTO{
							Type:      "optional_feature_drift",
							Component: sec.ID + ".switch_action",
							Action:    "remove",
							Reason:    "registration is an optional feature without explicit user request",
						})
					}
				}

				// Content Drift Check: Neutralize assumed marketing copy in subtitle
				if sub, ok := sec.Data["subtitle"].(string); ok {
					subLower := strings.ToLower(sub)
					if strings.Contains(subLower, "workflow") || strings.Contains(subLower, "dashboard") || strings.Contains(subLower, "kelola") {
						sec.Data["subtitle"] = "Masuk ke akun Anda."
						issues = append(issues, "Neutralized assumed product copy in form subtitle")
						structuredIssues = append(structuredIssues, dtos.ValidationIssueDTO{
							Type:      "content_drift",
							Component: sec.ID + ".subtitle",
							Action:    "neutralize",
							Reason:    "generic prompt must use neutral copy instead of assuming product workflow",
						})
					}
				}
			}
		}

		repairedSections = append(repairedSections, sec)
	}

	// Missing usable output must be reported, never replaced by an unrelated hero.
	if len(repairedSections) == 0 {
		validatedSpec := *designSpec
		validatedSpec.Sections = repairedSections
		return &dtos.UIValidationResultDTO{Status: "fail", Issues: append(issues, "No valid sections remain"), StrippedSections: strippedSections, StructuredIssues: structuredIssues, HallucinationCheck: "failed: no valid sections"}, &validatedSpec
	}

	// Clone design spec with repaired sections
	validatedSpec := *designSpec
	validatedSpec.Sections = repairedSections

	// Compute comprehensive 8-metric quality scores (0-10)
	fidelityScore := 10
	scopeAccuracyScore := 10
	traceabilityScore := 10
	simplicityScore := 10
	hallucinationSafetyScore := 10

	if len(strippedSections) > 0 {
		scopeAccuracyScore = 8
		fidelityScore = 9
		hallucinationSafetyScore = 9
	}
	if len(structuredIssues) > 0 {
		fidelityScore = 9
	}
	if !isSimpleComplexity {
		simplicityScore = 8
	}

	// 1. Deterministic Hierarchy Score (0-10):
	// Evaluates structural depth, heading hierarchy, and component differentiation
	hierarchyScore := 8
	hasHeadings := false
	hasSubtextOrBody := false
	for _, sec := range repairedSections {
		if sec.Data != nil {
			if title, ok := sec.Data["title"].(string); ok && strings.TrimSpace(title) != "" {
				hasHeadings = true
			}
			if headline, ok := sec.Data["headline"].(string); ok && strings.TrimSpace(headline) != "" {
				hasHeadings = true
			}
			if subtitle, ok := sec.Data["subtitle"].(string); ok && strings.TrimSpace(subtitle) != "" {
				hasSubtextOrBody = true
			}
			if desc, ok := sec.Data["description"].(string); ok && strings.TrimSpace(desc) != "" {
				hasSubtextOrBody = true
			}
			if htmlContent, ok := sec.Data["html"].(string); ok {
				hLower := strings.ToLower(htmlContent)
				if strings.Contains(hLower, "<h1") || strings.Contains(hLower, "<h2") || strings.Contains(hLower, "<h3") ||
					strings.Contains(hLower, "text-2xl") || strings.Contains(hLower, "text-3xl") || strings.Contains(hLower, "text-4xl") ||
					strings.Contains(hLower, "font-bold") || strings.Contains(hLower, "font-extrabold") {
					hasHeadings = true
				}
				if strings.Contains(hLower, "<p") || strings.Contains(hLower, "text-slate-500") || strings.Contains(hLower, "text-slate-400") ||
					strings.Contains(hLower, "text-sm") || strings.Contains(hLower, "text-base") {
					hasSubtextOrBody = true
				}
			}
		}
		for _, cmp := range sec.Components {
			cType := strings.ToLower(cmp.Type)
			if cType == "heading" || cType == "title" {
				hasHeadings = true
			}
			if cType == "subtext" || cType == "paragraph" || cType == "body" || cType == "description" {
				hasSubtextOrBody = true
			}
		}
	}
	if len(repairedSections) >= 2 {
		hierarchyScore++
	}
	if hasHeadings && hasSubtextOrBody {
		hierarchyScore++
	}
	if hierarchyScore > 10 {
		hierarchyScore = 10
	}

	// 2. Deterministic Visual Consistency Score (0-10):
	// Evaluates design tokens, layout consistency, and style definitions
	visualConsistencyScore := 9
	if strings.TrimSpace(designSpec.Visual.Style) != "" || strings.TrimSpace(designSpec.Visual.Theme) != "" {
		visualConsistencyScore = 10
	}
	if strings.TrimSpace(designSpec.Layout.Type) != "" && strings.TrimSpace(designSpec.Layout.Container) != "" {
		if visualConsistencyScore < 10 {
			visualConsistencyScore = 10
		}
	}

	// 3. Deterministic Responsive Quality Score (0-10):
	// Evaluates mobile adaptation specs and responsive layout primitives
	responsiveQualityScore := 8
	hasResponsiveClasses := false
	for _, sec := range repairedSections {
		if sec.Data != nil {
			if htmlContent, ok := sec.Data["html"].(string); ok {
				hLower := strings.ToLower(htmlContent)
				if strings.Contains(hLower, "sm:") || strings.Contains(hLower, "md:") || strings.Contains(hLower, "lg:") ||
					strings.Contains(hLower, "overflow-x-auto") || strings.Contains(hLower, "grid-cols") ||
					strings.Contains(hLower, "flex-col") || strings.Contains(hLower, "max-w-") {
					hasResponsiveClasses = true
					break
				}
			}
		}
	}
	if strings.TrimSpace(designSpec.Responsive.Desktop) != "" || strings.TrimSpace(designSpec.Responsive.Mobile) != "" {
		responsiveQualityScore++
	}
	if hasResponsiveClasses {
		responsiveQualityScore = 10
	}
	if responsiveQualityScore > 10 {
		responsiveQualityScore = 10
	}

	status := "pass"
	hallucinationCheck := "clean: no unauthorized business domains or features detected"
	if len(strippedSections) > 0 || len(structuredIssues) > 0 {
		status = "repaired"
		hallucinationCheck = "repaired: unrequested optional features or domain drift automatically cleansed"
	}

	result := &dtos.UIValidationResultDTO{
		Status: status,
		Score: dtos.ValidationScoreDTO{
			RequirementFidelity: fidelityScore,
			ScopeAccuracy:       scopeAccuracyScore,
			Traceability:        traceabilityScore,
			Simplicity:          simplicityScore,
			Hierarchy:           hierarchyScore,
			VisualConsistency:   visualConsistencyScore,
			ResponsiveQuality:   responsiveQualityScore,
			HallucinationSafety: hallucinationSafetyScore,
			// Backwards compatibility
			Scope:       scopeAccuracyScore,
			Consistency: visualConsistencyScore,
		},
		Issues:             issues,
		StrippedSections:   strippedSections,
		StructuredIssues:   structuredIssues,
		HallucinationCheck: hallucinationCheck,
	}

	return result, &validatedSpec
}
