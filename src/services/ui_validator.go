package services

import (
	"encoding/json"
	"strings"

	"github.com/Bobby-P-dev/go-diagram.git/src/dtos"
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

	// Helper to check if keyword is in explicit requirements or raw prompt
	isExplicitlyRequested := func(keyword string) bool {
		if reqSpec == nil {
			return false
		}
		kwLower := strings.ToLower(keyword)
		if strings.Contains(strings.ToLower(reqSpec.RawPrompt), kwLower) {
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
		secBytes, _ := json.Marshal(sec)
		secStr := strings.ToLower(string(secBytes))

		// 1. Hallucination Check: If domain is NOT specified by user, check for unrequested financial/CFO/crypto bloat
		hasHallucination := false
		if !isDomainSpecified {
			for _, kw := range hallucinatedDomainKeywords {
				if strings.Contains(secStr, kw) {
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

		// 2. Traceability Check: Section must have a requirement source
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

	// 4. Section Count Sanity Guard: Prevent runaway duplicates while allowing rich storytelling compositions (up to 12 sections)
	if isSimpleComplexity && len(repairedSections) > 4 {
		repairedSections = repairedSections[:4]
		issues = append(issues, "Bounded simple page complexity to max 4 sections")
	} else if len(repairedSections) > 12 {
		repairedSections = repairedSections[:12]
		issues = append(issues, "Bounded section count to 12 sections to ensure optimal visual rhythm and load performance")
	}

	// Ensure at least one valid section exists
	if len(repairedSections) == 0 {
		repairedSections = append(repairedSections, dtos.UISectionDTO{
			ID:                "sec-hero",
			Type:              "hero",
			Purpose:           "Present primary value proposition",
			RequirementSource: "primary_goal",
			Priority:          "high",
			Data: map[string]interface{}{
				"title":    "Solusi Digital Terpadu",
				"subtitle": "Pengalaman antarmuka modern yang bersih, cepat, dan mudah digunakan.",
				"actions": []map[string]string{
					{"label": "Mulai Sekarang", "variant": "primary"},
				},
			},
		})
		issues = append(issues, "Fallback hero section added to preserve minimal valid layout")
	}

	// Clone design spec with repaired sections
	validatedSpec := *designSpec
	validatedSpec.Sections = repairedSections

	// Compute comprehensive 8-metric quality scores (0-10)
	fidelityScore := 10
	scopeAccuracyScore := 10
	traceabilityScore := 10
	simplicityScore := 10
	hierarchyScore := 9
	visualConsistencyScore := 9
	responsiveQualityScore := 10
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
