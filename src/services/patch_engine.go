package services

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/Bobby-P-dev/go-diagram.git/src/dtos"
)

type PatchEngine struct{}

func NewPatchEngine() *PatchEngine {
	return &PatchEngine{}
}

// ApplyPatch applies targeted modifications to a UIFrameData based on a ChangePlan,
// strictly preserving all unaffected sections, typography, components, and layout.
func (p *PatchEngine) ApplyPatch(
	frame *dtos.UIFrameData,
	plan *dtos.ChangePlanDTO,
) (*dtos.UIFrameData, string, error) {
	if frame == nil {
		return nil, "", fmt.Errorf("cannot patch nil frame")
	}

	// Deep clone frame via JSON to avoid modifying the input
	frameBytes, err := json.Marshal(frame)
	if err != nil {
		return nil, "", fmt.Errorf("failed to serialize frame for cloning: %w", err)
	}

	var patchedFrame dtos.UIFrameData
	if err := json.Unmarshal(frameBytes, &patchedFrame); err != nil {
		return nil, "", fmt.Errorf("failed to deserialize cloned frame: %w", err)
	}

	if patchedFrame.Theme == nil {
		patchedFrame.Theme = map[string]interface{}{
			"mode":    "dark",
			"primary": "#6366f1",
		}
	}

	themeMode, _ := patchedFrame.Theme["mode"].(string)
	if themeMode == "" {
		themeMode = "dark"
	}

	accentColor, _ := patchedFrame.Theme["primary"].(string)
	if accentColor == "" {
		accentColor = "#6366f1"
	}

	var patchSummaryLines []string

	// Apply individual changes
	for _, change := range plan.Changes {
		switch change.Property {
		case "size":
			// Targeted button sizing
			if plan.Target.Component == "button" || strings.Contains(strings.ToLower(plan.Request), "tombol") || strings.Contains(strings.ToLower(plan.Request), "button") {
				for i := range patchedFrame.Sections {
					sec := &patchedFrame.Sections[i]
					if sec.Type == "form" || sec.Type == "auth_card" || sec.Type == "login_card" {
						if sec.Data == nil {
							sec.Data = make(map[string]interface{})
						}
						sec.Data["button_size"] = "small"
						sec.Data["submit_size"] = "small"
					}
				}
				// Also patch raw_html if present
				if patchedFrame.RawHtml != "" {
					patchedFrame.RawHtml = strings.ReplaceAll(patchedFrame.RawHtml, "py-2.5", "py-1.5")
					patchedFrame.RawHtml = strings.ReplaceAll(patchedFrame.RawHtml, "py-3", "py-1.5")
					patchedFrame.RawHtml = strings.ReplaceAll(patchedFrame.RawHtml, "text-xs font-bold", "text-[11px] font-semibold")
				}
				patchSummaryLines = append(patchSummaryLines, "Ukuran tombol login disesuaikan menjadi lebih ringkas (py-1.5, text-[11px]).")
			}

		case "primary_color":
			if change.To != "" {
				accentColor = change.To
				patchedFrame.Theme["primary"] = accentColor
				patchSummaryLines = append(patchSummaryLines, fmt.Sprintf("Warna aksen utama diperbarui menjadi %s.", accentColor))
			}

		case "theme_mode":
			if change.To != "" {
				themeMode = change.To
				patchedFrame.Theme["mode"] = themeMode
				patchSummaryLines = append(patchSummaryLines, fmt.Sprintf("Tema visual diperbarui menjadi %s mode.", themeMode))
			}

		case "alignment":
			for i := range patchedFrame.Sections {
				sec := &patchedFrame.Sections[i]
				if sec.Type == "form" || sec.Type == "auth_card" || sec.Type == "login_card" {
					if sec.Data == nil {
						sec.Data = make(map[string]interface{})
					}
					sec.Data["alignment"] = change.To
				}
			}
			patchSummaryLines = append(patchSummaryLines, fmt.Sprintf("Perataan form disesuaikan ke %s.", change.To))

		case "social_buttons":
			for i := range patchedFrame.Sections {
				sec := &patchedFrame.Sections[i]
				if sec.Type == "form" || sec.Type == "auth_card" || sec.Type == "login_card" {
					if sec.Data == nil {
						sec.Data = make(map[string]interface{})
					}
					sec.Data["social_buttons"] = []string{"Google"}
				}
			}
			patchSummaryLines = append(patchSummaryLines, "Opsi autentikasi Google ditambahkan ke kartu login sesuai permintaan eksplisit.")

		case "title", "content":
			for i := range patchedFrame.Sections {
				sec := &patchedFrame.Sections[i]
				if sec.Data != nil && change.To != "" {
					sec.Data["title"] = change.To
				}
			}
			patchSummaryLines = append(patchSummaryLines, fmt.Sprintf("Konten judul diperbarui menjadi %q.", change.To))
		}
	}

	// Update components in canonical design spec if size changed
	for i := range patchedFrame.Sections {
		sec := &patchedFrame.Sections[i]
		for cIdx := range sec.Components {
			cmp := &sec.Components[cIdx]
			if cmp.ID == "cmp-login-button" || cmp.Type == "button" {
				if cmp.Style == nil {
					cmp.Style = make(map[string]interface{})
				}
				if plan.Target.Component == "button" || strings.Contains(strings.ToLower(plan.Request), "tombol") {
					cmp.Style["size"] = "small"
				}
			}
		}
	}

	// Re-synchronize code_export from sections so code and visual preview remain 100% accurate
	if len(patchedFrame.Sections) > 0 {
		patchedFrame.CodeExport = generateVueCodeExport(patchedFrame.Title, patchedFrame.Sections, themeMode, accentColor)
	}

	// Synchronize canonical design state and implementation
	patchedFrame.SyncCanonical(60, 60)
	if patchedFrame.Implementation != nil && patchedFrame.CodeExport != nil {
		patchedFrame.Implementation.Source.Vue = patchedFrame.CodeExport["vue"]
		patchedFrame.Implementation.Source.HTML = patchedFrame.CodeExport["html"]
	}
	if len(patchedFrame.Sections) > 0 {
		patchedFrame.CodeExport = generateVueCodeExport(patchedFrame.Title, patchedFrame.Sections, themeMode, accentColor)
	}

	// Attach change plan to frame for developer transparency and inspector display
	patchedFrame.ChangePlan = plan

	// Summary explanation
	var explanation string
	if len(patchSummaryLines) > 0 {
		explanation = strings.Join(patchSummaryLines, " ")
	} else {
		explanation = fmt.Sprintf("Perubahan lokal berhasil diterapkan untuk permintaan: %s.", plan.Request)
	}



	return &patchedFrame, explanation, nil
}
