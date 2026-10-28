package controller

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/Bobby-P-dev/go-diagram.git/internal/modules/identity/dto"
	"github.com/Bobby-P-dev/go-diagram.git/internal/modules/identity/service"
	"github.com/Bobby-P-dev/go-diagram.git/internal/platform/httpresponse"
)

type AdminService interface {
	GetAllWithStats(ctx context.Context) ([]service.Summary, error)
	FindByID(ctx context.Context, id string) (*service.CredentialRecord, error)
	Create(ctx context.Context, input service.CreateInput) (*service.CredentialRecord, error)
	Update(ctx context.Context, input service.UpdateInput) (*service.CredentialRecord, error)
	Delete(ctx context.Context, id string) error
}

type AdminController struct {
	credentials AdminService
}

func NewAdminController(credentials AdminService) *AdminController {
	return &AdminController{credentials: credentials}
}

func (c *AdminController) GetCredentials(w http.ResponseWriter, r *http.Request) {
	list, err := c.credentials.GetAllWithStats(r.Context())
	if err != nil {
		httpresponse.RespondError(w, http.StatusInternalServerError, "Gagal mengambil kredensial", err.Error())
		return
	}
	httpresponse.RespondJSON(w, http.StatusOK, "Daftar kredensial berhasil dimuat", summaryDTOs(list))
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
	for _, layout := range layouts {
		if parsed, err := time.Parse(layout, s); err == nil {
			return &parsed, nil
		}
	}
	return nil, errors.New("invalid time format")
}

func (c *AdminController) CreateCredential(w http.ResponseWriter, r *http.Request) {
	var req dto.CreateCredentialRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpresponse.RespondError(w, http.StatusBadRequest, "Invalid Request", "Format data JSON tidak valid")
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

	created, err := c.credentials.Create(r.Context(), service.CreateInput{
		Name: req.Name, Role: role, ExpiresAt: expiresAt,
		CanGenerateDiagram: canDiagram, CanGenerateUI: canUI,
	})
	if err != nil {
		httpresponse.RespondError(w, http.StatusInternalServerError, "Gagal membuat kredensial", err.Error())
		return
	}

	httpresponse.RespondJSON(w, http.StatusCreated, "Kredensial baru berhasil dibuat", credentialSummaryDTO(created))
}

func (c *AdminController) UpdateCredential(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if strings.TrimSpace(id) == "" {
		httpresponse.RespondError(w, http.StatusBadRequest, "Missing ID", "ID kredensial diperlukan")
		return
	}

	var req dto.UpdateCredentialRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpresponse.RespondError(w, http.StatusBadRequest, "Invalid Request", "Format data JSON tidak valid")
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
		curr, err := c.credentials.FindByID(r.Context(), id)
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

	updated, err := c.credentials.Update(r.Context(), service.UpdateInput{
		ID: id, Name: req.Name, Role: req.Role, IsActive: req.IsActive,
		ExpiresAt: newExpiresAt, UpdateExpiresAt: updateExpiresAt,
		CanGenerateDiagram: req.CanGenerateDiagram, CanGenerateUI: req.CanGenerateUI,
	})
	if err != nil {
		httpresponse.RespondError(w, http.StatusInternalServerError, "Gagal memperbarui kredensial", err.Error())
		return
	}

	httpresponse.RespondJSON(w, http.StatusOK, "Kredensial berhasil diperbarui", credentialSummaryDTO(updated))
}

func (c *AdminController) DeleteCredential(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if strings.TrimSpace(id) == "" {
		httpresponse.RespondError(w, http.StatusBadRequest, "Missing ID", "ID kredensial diperlukan")
		return
	}

	if err := c.credentials.Delete(r.Context(), id); err != nil {
		httpresponse.RespondError(w, http.StatusBadRequest, "Gagal menghapus kredensial", err.Error())
		return
	}

	httpresponse.RespondJSON(w, http.StatusOK, "Kredensial berhasil dihapus", map[string]bool{"success": true})
}

func summaryDTOs(rows []service.Summary) []dto.CredentialSummaryDTO {
	var result []dto.CredentialSummaryDTO
	for _, row := range rows {
		result = append(result, dto.CredentialSummaryDTO(row))
	}
	return result
}
