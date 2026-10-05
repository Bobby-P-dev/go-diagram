package controllers

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/Bobby-P-dev/go-diagram.git/src/dtos"
	"github.com/Bobby-P-dev/go-diagram.git/src/middlewares"
	"github.com/Bobby-P-dev/go-diagram.git/src/models"
	"github.com/Bobby-P-dev/go-diagram.git/src/utils"
)

type AuthController struct {
	credModel models.CredentialModelInterface
}

func NewAuthController(credModel models.CredentialModelInterface) *AuthController {
	return &AuthController{credModel: credModel}
}

func (c *AuthController) Verify(w http.ResponseWriter, r *http.Request) {
	var req dtos.VerifyCredentialRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.RespondError(w, http.StatusBadRequest, "Invalid Request", "Format data JSON tidak valid")
		return
	}
	defer r.Body.Close()

	key := strings.TrimSpace(req.CredentialKey)
	if key == "" {
		utils.RespondError(w, http.StatusBadRequest, "Kredensial Diperlukan", "Harap masukkan kode kredensial Anda.")
		return
	}

	user, err := c.credModel.FindByKey(key)
	if err != nil {
		utils.RespondError(w, http.StatusUnauthorized, "Kredensial Tidak Ditemukan", "Kode kredensial tidak terdaftar dalam sistem.")
		return
	}

	if !user.IsActive {
		utils.RespondError(w, http.StatusForbidden, "Kredensial Dinonaktifkan", "Akses akun ini telah dinonaktifkan oleh administrator.")
		return
	}

	if user.ExpiresAt != nil && time.Now().After(*user.ExpiresAt) {
		utils.RespondError(w, http.StatusForbidden, "Masa Aktif Kedaluwarsa", "Masa aktif kredensial Anda telah berakhir. Hubungi administrator untuk memperpanjang akses.")
		return
	}

	// Update last login
	_ = c.credModel.UpdateLastLogin(user.ID)

	var expiresAtStr *string
	humanText, isExpired := models.FormatExpiresInHuman(user.ExpiresAt)
	if user.ExpiresAt != nil {
		s := user.ExpiresAt.Format(time.RFC3339)
		expiresAtStr = &s
	}

	userDTO := dtos.AuthUserDTO{
		ID:                 user.ID,
		CredentialKey:      user.CredentialKey,
		Name:               user.Name,
		Role:               user.Role,
		IsActive:           user.IsActive,
		CanGenerateDiagram: user.CanGenerateDiagram,
		CanGenerateUI:      user.CanGenerateUI,
		ExpiresAt:          expiresAtStr,
		IsExpired:          isExpired,
		ExpiresInHuman:     humanText,
	}

	resp := dtos.LoginResponse{
		Token: user.CredentialKey,
		User:  userDTO,
	}

	utils.RespondJSON(w, http.StatusOK, "Autentikasi berhasil", resp)
}

func (c *AuthController) Me(w http.ResponseWriter, r *http.Request) {
	user := middlewares.GetUserFromContext(r.Context())
	if user == nil {
		utils.RespondError(w, http.StatusUnauthorized, "Unauthorized", "Sesi tidak ditemukan atau telah berakhir.")
		return
	}

	var expiresAtStr *string
	humanText, isExpired := models.FormatExpiresInHuman(user.ExpiresAt)
	if user.ExpiresAt != nil {
		s := user.ExpiresAt.Format(time.RFC3339)
		expiresAtStr = &s
	}

	userDTO := dtos.AuthUserDTO{
		ID:                 user.ID,
		CredentialKey:      user.CredentialKey,
		Name:               user.Name,
		Role:               user.Role,
		IsActive:           user.IsActive,
		CanGenerateDiagram: user.CanGenerateDiagram,
		CanGenerateUI:      user.CanGenerateUI,
		ExpiresAt:          expiresAtStr,
		IsExpired:          isExpired,
		ExpiresInHuman:     humanText,
	}

	utils.RespondJSON(w, http.StatusOK, "Data profil berhasil diambil", userDTO)
}

func (c *AuthController) Logout(w http.ResponseWriter, r *http.Request) {
	utils.RespondJSON(w, http.StatusOK, "Berhasil logout", map[string]bool{"success": true})
}
