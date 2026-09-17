package services

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/Bobby-P-dev/go-diagram.git/src/dtos"
)

// DuplicateValidator enforces strict uniqueness and semantic distinction across sections and components.
type DuplicateValidator struct{}

func NewDuplicateValidator() *DuplicateValidator {
	return &DuplicateValidator{}
}

// ValidateDuplicateSectionIDs ensures no two sections share the same ID and all component IDs are unique.
func (v *DuplicateValidator) ValidateDuplicateSectionIDs(sections []dtos.UISectionDTO) error {
	seenSections := make(map[string]bool)
	seenComponents := make(map[string]bool)

	for idx, sec := range sections {
		secID := strings.TrimSpace(sec.ID)
		if secID == "" {
			return fmt.Errorf("section at index %d has empty ID", idx)
		}
		if seenSections[secID] {
			return fmt.Errorf("duplicate section ID detected: %q (index %d)", secID, idx)
		}
		seenSections[secID] = true

		for cIdx, cmp := range sec.Components {
			cmpID := strings.TrimSpace(cmp.ID)
			if cmpID == "" {
				continue
			}
			if seenComponents[cmpID] {
				return fmt.Errorf("duplicate component ID %q detected in section %q (cmp index %d)", cmpID, secID, cIdx)
			}
			seenComponents[cmpID] = true
		}
	}
	return nil
}

// ValidateHTMLDuplicateIDs scans raw HTML for duplicate data-rl-id attributes on sections and components.
func (v *DuplicateValidator) ValidateHTMLDuplicateIDs(rawHtml string) error {
	if strings.TrimSpace(rawHtml) == "" {
		return nil
	}

	re := regexp.MustCompile(`data-rl-id=["']([^"']+)["']`)
	matches := re.FindAllStringSubmatch(rawHtml, -1)

	seen := make(map[string]int)
	var duplicates []string

	for _, m := range matches {
		if len(m) > 1 {
			id := m[1]
			seen[id]++
			if seen[id] == 2 {
				duplicates = append(duplicates, id)
			}
		}
	}

	if len(duplicates) > 0 {
		return fmt.Errorf("duplicate data-rl-id attributes detected in HTML: %s", strings.Join(duplicates, ", "))
	}
	return nil
}

// ValidateSemanticDuplicates checks for accidental duplicate sections having identical titles,
// identical purposes, or identical image assets.
func (v *DuplicateValidator) ValidateSemanticDuplicates(sections []dtos.UISectionDTO, op dtos.OperationType) error {
	// If the user explicitly requested an insert, we allow some thematic overlap
	if op == dtos.OpInsertSection {
		return nil
	}

	seenTitles := make(map[string]string)   // normalizedTitle -> secID
	seenPurposes := make(map[string]string) // normalizedPurpose -> secID
	seenImages := make(map[string]string)   // imageURL -> secID

	for _, sec := range sections {
		// 1. Headline / Title check
		title := extractSectionHeadline(sec)
		normTitle := normalizeSemanticString(title)
		if normTitle != "" && len(normTitle) > 5 && !isGenericTitle(normTitle) {
			if existingID, found := seenTitles[normTitle]; found && existingID != sec.ID {
				return fmt.Errorf("semantic duplicate detected: section %q and %q share identical headline %q", existingID, sec.ID, title)
			}
			seenTitles[normTitle] = sec.ID
		}

		// 2. Purpose check
		normPurpose := normalizeSemanticString(sec.Purpose)
		if normPurpose != "" && len(normPurpose) > 10 {
			if existingID, found := seenPurposes[normPurpose]; found && existingID != sec.ID {
				return fmt.Errorf("semantic duplicate detected: section %q and %q share identical purpose %q", existingID, sec.ID, sec.Purpose)
			}
			seenPurposes[normPurpose] = sec.ID
		}

		// 3. Repeated Image asset check (avoid identical photo across multiple content sections)
		imgURL := extractSectionImageURL(sec)
		if imgURL != "" && !strings.Contains(imgURL, "logo") && !strings.Contains(imgURL, "avatar") {
			if existingID, found := seenImages[imgURL]; found && existingID != sec.ID {
				return fmt.Errorf("semantic duplicate detected: section %q and %q share identical image asset %q", existingID, sec.ID, imgURL)
			}
			seenImages[imgURL] = sec.ID
		}
	}
	return nil
}

func extractSectionHeadline(sec dtos.UISectionDTO) string {
	if sec.Data != nil {
		if h, ok := sec.Data["headline"].(string); ok && h != "" {
			return h
		}
		if t, ok := sec.Data["title"].(string); ok && t != "" {
			return t
		}
	}
	return ""
}

func extractSectionImageURL(sec dtos.UISectionDTO) string {
	if sec.Data != nil {
		if img, ok := sec.Data["image_url"].(string); ok && img != "" {
			return img
		}
		if img, ok := sec.Data["image"].(string); ok && img != "" {
			return img
		}
	}
	return ""
}

func normalizeSemanticString(s string) string {
	lower := strings.ToLower(strings.TrimSpace(s))
	// Remove common punctuation
	re := regexp.MustCompile(`[^\w\s]`)
	cleaned := re.ReplaceAllString(lower, "")
	return strings.Join(strings.Fields(cleaned), " ")
}

func isGenericTitle(normTitle string) bool {
	generics := []string{
		"hero", "navbar", "footer", "products", "reviews", "features", "contact",
		"about", "menu", "pricing", "faqs", "gallery",
	}
	for _, g := range generics {
		if normTitle == g {
			return true
		}
	}
	return false
}
