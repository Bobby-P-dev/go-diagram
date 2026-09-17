package services

import (
	"fmt"
	"strings"

	"github.com/Bobby-P-dev/go-diagram.git/src/dtos"
)

// SectionReconciler executes deterministic state reconciliation based on explicit operation types
// and stable IDs, preventing array concatenation and duplicate section accumulation.
type SectionReconciler struct {
	validator *DuplicateValidator
}

func NewSectionReconciler(validator *DuplicateValidator) *SectionReconciler {
	if validator == nil {
		validator = NewDuplicateValidator()
	}
	return &SectionReconciler{validator: validator}
}

type ReconcileRequest struct {
	CurrentSections  []dtos.UISectionDTO
	IncomingSections []dtos.UISectionDTO
	Operation        dtos.OperationType
	TargetID         string
	InsertIndex      int // Optional: index to insert (-1 means auto-place before footer or append)
}

type ReconcileResult struct {
	Sections     []dtos.UISectionDTO
	Operation    dtos.OperationType
	TargetID     string
	BeforeCount  int
	AfterCount   int
	Explanation  string
}

// Reconcile executes the state reconciliation according to the explicit OperationType.
func (r *SectionReconciler) Reconcile(req ReconcileRequest) (*ReconcileResult, error) {
	op := req.Operation
	if op == "" {
		op = dtos.OpFullReplace
	}

	beforeCount := len(req.CurrentSections)

	switch op {
	case dtos.OpFullReplace:
		if len(req.IncomingSections) == 0 {
			return nil, fmt.Errorf("FULL_REPLACE requires at least one incoming section")
		}
		// Validate incoming sections
		if err := r.validator.ValidateDuplicateSectionIDs(req.IncomingSections); err != nil {
			return nil, fmt.Errorf("FULL_REPLACE rejected due to duplicate IDs: %w", err)
		}
		if err := r.validator.ValidateSemanticDuplicates(req.IncomingSections, op); err != nil {
			return nil, fmt.Errorf("FULL_REPLACE rejected due to semantic duplicates: %w", err)
		}

		resultSections := make([]dtos.UISectionDTO, len(req.IncomingSections))
		copy(resultSections, req.IncomingSections)

		return &ReconcileResult{
			Sections:    resultSections,
			Operation:   op,
			BeforeCount: beforeCount,
			AfterCount:  len(resultSections),
			Explanation: fmt.Sprintf("FULL_REPLACE: Halaman dimutasi total menjadi %d seksi kanonikal baru.", len(resultSections)),
		}, nil

	case dtos.OpSectionPatch:
		targetID := strings.TrimSpace(req.TargetID)
		if targetID == "" && len(req.IncomingSections) > 0 {
			targetID = req.IncomingSections[0].ID
		}
		if targetID == "" {
			return nil, fmt.Errorf("SECTION_PATCH requires a non-empty TargetID")
		}
		if len(req.IncomingSections) == 0 {
			return nil, fmt.Errorf("SECTION_PATCH requires incoming patch data")
		}

		incoming := req.IncomingSections[0]
		matchedIdx := -1
		resultSections := make([]dtos.UISectionDTO, len(req.CurrentSections))
		copy(resultSections, req.CurrentSections)

		for i, sec := range resultSections {
			if sec.ID == targetID || sec.Type == targetID {
				matchedIdx = i
				break
			}
		}

		if matchedIdx == -1 {
			return nil, fmt.Errorf("SECTION_PATCH failed: target section %q not found in current page", targetID)
		}

		// Patch fields while strictly preserving stable ID and position
		target := &resultSections[matchedIdx]
		if incoming.Type != "" {
			target.Type = incoming.Type
		}
		if incoming.Priority != "" {
			target.Priority = incoming.Priority
		}
		if incoming.Purpose != "" {
			target.Purpose = incoming.Purpose
		}
		if incoming.RequirementSource != "" {
			target.RequirementSource = incoming.RequirementSource
		}
		if incoming.RequirementSourceObj != nil {
			target.RequirementSourceObj = incoming.RequirementSourceObj
		}
		if incoming.Data != nil {
			if target.Data == nil {
				target.Data = make(map[string]interface{})
			}
			for k, v := range incoming.Data {
				target.Data[k] = v
			}
		}
		if len(incoming.Components) > 0 {
			target.Components = incoming.Components
		}

		if err := r.validator.ValidateDuplicateSectionIDs(resultSections); err != nil {
			return nil, fmt.Errorf("SECTION_PATCH resulted in duplicate IDs: %w", err)
		}

		return &ReconcileResult{
			Sections:    resultSections,
			Operation:   op,
			TargetID:    targetID,
			BeforeCount: beforeCount,
			AfterCount:  len(resultSections),
			Explanation: fmt.Sprintf("SECTION_PATCH: Seksi %q berhasil diperbarui secara lokal. Seluruh %d seksi lain utuh.", targetID, len(resultSections)-1),
		}, nil

	case dtos.OpSectionReplace:
		targetID := strings.TrimSpace(req.TargetID)
		if targetID == "" && len(req.IncomingSections) > 0 {
			targetID = req.IncomingSections[0].ID
		}
		if targetID == "" {
			return nil, fmt.Errorf("SECTION_REPLACE requires a non-empty TargetID")
		}
		if len(req.IncomingSections) == 0 {
			return nil, fmt.Errorf("SECTION_REPLACE requires incoming replacement section")
		}

		replacement := req.IncomingSections[0]
		// Force preservation of stable ID
		replacement.ID = targetID

		matchedIdx := -1
		resultSections := make([]dtos.UISectionDTO, len(req.CurrentSections))
		copy(resultSections, req.CurrentSections)

		for i, sec := range resultSections {
			if sec.ID == targetID || sec.Type == targetID {
				matchedIdx = i
				break
			}
		}

		if matchedIdx == -1 {
			return nil, fmt.Errorf("SECTION_REPLACE failed: target section %q not found", targetID)
		}

		resultSections[matchedIdx] = replacement

		if err := r.validator.ValidateDuplicateSectionIDs(resultSections); err != nil {
			return nil, fmt.Errorf("SECTION_REPLACE resulted in duplicate IDs: %w", err)
		}

		return &ReconcileResult{
			Sections:    resultSections,
			Operation:   op,
			TargetID:    targetID,
			BeforeCount: beforeCount,
			AfterCount:  len(resultSections),
			Explanation: fmt.Sprintf("SECTION_REPLACE: Seksi %q diganti dengan versi baru pada posisi urutan yang sama.", targetID),
		}, nil

	case dtos.OpInsertSection:
		if len(req.IncomingSections) == 0 {
			return nil, fmt.Errorf("INSERT_SECTION requires at least one section to insert")
		}

		// Ensure incoming section IDs do not collide with existing ones
		currentIDs := make(map[string]bool)
		for _, sec := range req.CurrentSections {
			currentIDs[sec.ID] = true
		}

		for _, inc := range req.IncomingSections {
			if currentIDs[inc.ID] {
				return nil, fmt.Errorf("INSERT_SECTION rejected: section ID %q already exists in page", inc.ID)
			}
		}

		insertAt := -1
		if req.TargetID != "" {
			for i, sec := range req.CurrentSections {
				if sec.ID == req.TargetID || sec.Type == req.TargetID {
					insertAt = i + 1
					break
				}
			}
		}

		if insertAt < 0 && req.InsertIndex > 0 && req.InsertIndex <= len(req.CurrentSections) {
			insertAt = req.InsertIndex
		}

		// If still unspecified, insert right before footer (if footer exists), otherwise at the end
		if insertAt < 0 {
			insertAt = len(req.CurrentSections)
			for i, sec := range req.CurrentSections {
				if sec.Type == "footer" || sec.ID == "sec-footer" {
					insertAt = i
					break
				}
			}
		}

		resultSections := make([]dtos.UISectionDTO, 0, len(req.CurrentSections)+len(req.IncomingSections))
		resultSections = append(resultSections, req.CurrentSections[:insertAt]...)
		resultSections = append(resultSections, req.IncomingSections...)
		resultSections = append(resultSections, req.CurrentSections[insertAt:]...)

		if err := r.validator.ValidateDuplicateSectionIDs(resultSections); err != nil {
			return nil, fmt.Errorf("INSERT_SECTION resulted in duplicate IDs: %w", err)
		}

		return &ReconcileResult{
			Sections:    resultSections,
			Operation:   op,
			TargetID:    req.TargetID,
			BeforeCount: beforeCount,
			AfterCount:  len(resultSections),
			Explanation: fmt.Sprintf("INSERT_SECTION: Menambahkan %d seksi baru pada posisi indeks %d.", len(req.IncomingSections), insertAt),
		}, nil

	case dtos.OpDeleteSection:
		targetID := strings.TrimSpace(req.TargetID)
		if targetID == "" {
			return nil, fmt.Errorf("DELETE_SECTION requires a non-empty TargetID")
		}

		if len(req.CurrentSections) <= 1 {
			return nil, fmt.Errorf("DELETE_SECTION rejected: page must retain at least one section")
		}

		matchedIdx := -1
		for i, sec := range req.CurrentSections {
			if sec.ID == targetID || sec.Type == targetID {
				matchedIdx = i
				break
			}
		}

		if matchedIdx == -1 {
			return nil, fmt.Errorf("DELETE_SECTION failed: target section %q not found", targetID)
		}

		resultSections := make([]dtos.UISectionDTO, 0, len(req.CurrentSections)-1)
		resultSections = append(resultSections, req.CurrentSections[:matchedIdx]...)
		resultSections = append(resultSections, req.CurrentSections[matchedIdx+1:]...)

		return &ReconcileResult{
			Sections:    resultSections,
			Operation:   op,
			TargetID:    targetID,
			BeforeCount: beforeCount,
			AfterCount:  len(resultSections),
			Explanation: fmt.Sprintf("DELETE_SECTION: Seksi %q berhasil dihapus dari halaman.", targetID),
		}, nil

	case dtos.OpReorderSections:
		if len(req.IncomingSections) != len(req.CurrentSections) {
			return nil, fmt.Errorf("REORDER_SECTIONS requires incoming sections with matching count")
		}

		secMap := make(map[string]dtos.UISectionDTO)
		for _, sec := range req.CurrentSections {
			secMap[sec.ID] = sec
		}

		resultSections := make([]dtos.UISectionDTO, 0, len(req.IncomingSections))
		for _, inc := range req.IncomingSections {
			orig, found := secMap[inc.ID]
			if !found {
				return nil, fmt.Errorf("REORDER_SECTIONS failed: section %q does not exist in current page", inc.ID)
			}
			resultSections = append(resultSections, orig)
		}

		return &ReconcileResult{
			Sections:    resultSections,
			Operation:   op,
			BeforeCount: beforeCount,
			AfterCount:  len(resultSections),
			Explanation: "REORDER_SECTIONS: Urutan seksi berhasil diperbarui tanpa mengubah konten.",
		}, nil

	case dtos.OpComponentPatch:
		// Sections array unchanged; components metadata updated
		resultSections := make([]dtos.UISectionDTO, len(req.CurrentSections))
		copy(resultSections, req.CurrentSections)

		return &ReconcileResult{
			Sections:    resultSections,
			Operation:   op,
			TargetID:    req.TargetID,
			BeforeCount: beforeCount,
			AfterCount:  len(resultSections),
			Explanation: fmt.Sprintf("COMPONENT_PATCH: Komponen %q dimodifikasi tanpa mengubah struktur seksi.", req.TargetID),
		}, nil

	default:
		return nil, fmt.Errorf("unsupported OperationType: %q", op)
	}
}
