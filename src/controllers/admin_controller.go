package controllers

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/Bobby-P-dev/go-diagram.git/src/dtos"
	"github.com/Bobby-P-dev/go-diagram.git/src/models"
	"github.com/Bobby-P-dev/go-diagram.git/src/utils"
)

type AdminController struct {
	credModel models.CredentialModelInterface
}

func NewAdminController(credModel models.CredentialModelInterface) *AdminController {
	return &AdminController{credModel: credModel}
}

func (c *AdminController) GetCredentials(w http.ResponseWriter, r *http.Request) {
	list, err := c.credModel.GetAllWithStats()
	if err != nil {
		utils.RespondError(w, http.StatusInternalServerError, "Gagal mengambil kredensial", err.Error())
		return
	}
	utils.RespondJSON(w, http.StatusOK, "Daftar kredensial berhasil dimuat", list)
}

func parseTimeFlexibly(s string) (*time.Time, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, nil
	}
	layouts := []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02T15:04:05",
		"2006-01-02T15:04",
		"2006-01-02 15:04:05",
		"2006-01-02 15:04",
		"2006-01-02",
	}
	for _, l := range layouts {
		if t, err := time.Parse(l, s); err == nil {
			return &t, nil
		}
	}
	return nil, errors.New("invalid time format")
}

func (c *AdminController) CreateCredential(w http.ResponseWriter, r *http.Request) {
	var req dtos.CreateCredentialRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.RespondError(w, http.StatusBadRequest, "Invalid Request", "Format data JSON tidak valid")
		return
	}
	defer r.Body.Close()

	var expiresAt *time.Time
	if req.ExpiresAt != "" {
		if t, err := parseTimeFlexibly(req.ExpiresAt); err == nil {
			expiresAt = t
		}
	} else if req.DurationMinutes > 0 {
		t := time.Now().Add(time.Duration(req.DurationMinutes) * time.Minute)
		expiresAt = &t
	} else if req.DurationHours > 0 {
		t := time.Now().Add(time.Duration(req.DurationHours) * time.Hour)
		expiresAt = &t
	} else if req.DurationDays > 0 {
		t := time.Now().AddDate(0, 0, req.DurationDays)
		expiresAt = &t
	}

	role := req.Role
	if role != "admin" {
		role = "user"
	}

	canDiagram := true
	if req.CanGenerateDiagram != nil {
		canDiagram = *req.CanGenerateDiagram
	}
	canUI := true
	if req.CanGenerateUI != nil {
		canUI = *req.CanGenerateUI
	}
	if !canDiagram && !canUI {
		canDiagram = true
		canUI = true
	}

	created, err := c.credModel.Create(req.Name, role, expiresAt, canDiagram, canUI)
	if err != nil {
		utils.RespondError(w, http.StatusInternalServerError, "Gagal membuat kredensial", err.Error())
		return
	}

	var expiresAtStr *string
	humanText, isExpired := models.FormatExpiresInHuman(created.ExpiresAt)
	if created.ExpiresAt != nil {
		s := created.ExpiresAt.Format(time.RFC3339)
		expiresAtStr = &s
	}

	resp := dtos.CredentialSummaryDTO{
		ID:                 created.ID,
		CredentialKey:      created.CredentialKey,
		Name:               created.Name,
		Role:               created.Role,
		IsActive:           created.IsActive,
		CanGenerateDiagram: created.CanGenerateDiagram,
		CanGenerateUI:      created.CanGenerateUI,
		ExpiresAt:          expiresAtStr,
		IsExpired:          isExpired,
		ExpiresInHuman:     humanText,
		ProjectCount:       0,
		CreatedAt:          created.CreatedAt.Format(time.RFC3339),
		UpdatedAt:          created.UpdatedAt.Format(time.RFC3339),
	}

	utils.RespondJSON(w, http.StatusCreated, "Kredensial baru berhasil dibuat", resp)
}

func (c *AdminController) UpdateCredential(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if strings.TrimSpace(id) == "" {
		utils.RespondError(w, http.StatusBadRequest, "Missing ID", "ID kredensial diperlukan")
		return
	}

	var req dtos.UpdateCredentialRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.RespondError(w, http.StatusBadRequest, "Invalid Request", "Format data JSON tidak valid")
		return
	}
	defer r.Body.Close()

	var newExpiresAt *time.Time
	updateExpiresAt := false

	if req.SetPermanent || (req.ExpiresAt != nil && (*req.ExpiresAt == "" || *req.ExpiresAt == "never" || *req.ExpiresAt == "null")) {
		updateExpiresAt = true
		newExpiresAt = nil
	} else if req.ExpiresAt != nil && *req.ExpiresAt != "" {
		if t, err := parseTimeFlexibly(*req.ExpiresAt); err == nil {
			updateExpiresAt = true
			newExpiresAt = t
		}
	} else if req.DurationMinutes > 0 {
		updateExpiresAt = true
		t := time.Now().Add(time.Duration(req.DurationMinutes) * time.Minute)
		newExpiresAt = &t
	} else if req.DurationHours > 0 {
		updateExpiresAt = true
		t := time.Now().Add(time.Duration(req.DurationHours) * time.Hour)
		newExpiresAt = &t
	} else if req.DurationDays > 0 {
		updateExpiresAt = true
		t := time.Now().AddDate(0, 0, req.DurationDays)
		newExpiresAt = &t
	} else if req.ExtendMinutes > 0 || req.ExtendHours > 0 || req.ExtendDays > 0 {
		updateExpiresAt = true
		curr, err := c.credModel.FindByID(id)
		if err == nil {
			var base time.Time
			if curr.ExpiresAt != nil && curr.ExpiresAt.After(time.Now()) {
				base = *curr.ExpiresAt
			} else {
				base = time.Now()
			}
			if req.ExtendMinutes > 0 {
				base = base.Add(time.Duration(req.ExtendMinutes) * time.Minute)
			}
			if req.ExtendHours > 0 {
				base = base.Add(time.Duration(req.ExtendHours) * time.Hour)
			}
			if req.ExtendDays > 0 {
				base = base.AddDate(0, 0, req.ExtendDays)
			}
			newExpiresAt = &base
		}
	}

	updated, err := c.credModel.UpdateFull(id, req.Name, req.Role, req.IsActive, newExpiresAt, updateExpiresAt, req.CanGenerateDiagram, req.CanGenerateUI)
	if err != nil {
		utils.RespondError(w, http.StatusInternalServerError, "Gagal memperbarui kredensial", err.Error())
		return
	}

	var expiresAtStr *string
	humanText, isExpired := models.FormatExpiresInHuman(updated.ExpiresAt)
	if updated.ExpiresAt != nil {
		s := updated.ExpiresAt.Format(time.RFC3339)
		expiresAtStr = &s
	}

	resp := dtos.CredentialSummaryDTO{
		ID:                 updated.ID,
		CredentialKey:      updated.CredentialKey,
		Name:               updated.Name,
		Role:               updated.Role,
		IsActive:           updated.IsActive,
		CanGenerateDiagram: updated.CanGenerateDiagram,
		CanGenerateUI:      updated.CanGenerateUI,
		ExpiresAt:          expiresAtStr,
		IsExpired:          isExpired,
		ExpiresInHuman:     humanText,
		CreatedAt:          updated.CreatedAt.Format(time.RFC3339),
		UpdatedAt:          updated.UpdatedAt.Format(time.RFC3339),
	}

	utils.RespondJSON(w, http.StatusOK, "Kredensial berhasil diperbarui", resp)
}

func (c *AdminController) DeleteCredential(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if strings.TrimSpace(id) == "" {
		utils.RespondError(w, http.StatusBadRequest, "Missing ID", "ID kredensial diperlukan")
		return
	}

	if err := c.credModel.Delete(id); err != nil {
		utils.RespondError(w, http.StatusBadRequest, "Gagal menghapus kredensial", err.Error())
		return
	}

	utils.RespondJSON(w, http.StatusOK, "Kredensial berhasil dihapus", map[string]bool{"success": true})
}
