package controller

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/Bobby-P-dev/go-diagram.git/internal/modules/identity/authcontext"
	"github.com/Bobby-P-dev/go-diagram.git/internal/modules/identity/dto"
	"github.com/Bobby-P-dev/go-diagram.git/internal/modules/identity/service"
	"github.com/Bobby-P-dev/go-diagram.git/internal/platform/httpresponse"
)

type AuthService interface {
	FindByKey(ctx context.Context, key string) (*service.CredentialRecord, error)
	UpdateLastLogin(ctx context.Context, id string) error
}

type AuthController struct {
	credentials AuthService
	browser     *BrowserController
}

func NewAuthController(credentials AuthService, browser ...*BrowserController) *AuthController {
	controller := &AuthController{credentials: credentials}
	if len(browser) > 0 {
		controller.browser = browser[0]
	}
	return controller
}

func (c *AuthController) Verify(w http.ResponseWriter, r *http.Request) {
	var req dto.VerifyCredentialRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpresponse.RespondError(w, http.StatusBadRequest, "Invalid Request", "Format data JSON tidak valid")
		return
	}
	defer r.Body.Close()

	key := strings.TrimSpace(req.CredentialKey)
	if key == "" {
		httpresponse.RespondError(w, http.StatusBadRequest, "Kredensial Diperlukan", "Harap masukkan kode kredensial Anda.")
		return
	}

	user, err := c.credentials.FindByKey(r.Context(), key)
	if err != nil {
		httpresponse.RespondError(w, http.StatusUnauthorized, "Kredensial Tidak Ditemukan", "Kode kredensial tidak terdaftar dalam sistem.")
		return
	}

	if !user.IsActive {
		httpresponse.RespondError(w, http.StatusForbidden, "Kredensial Dinonaktifkan", "Akses akun ini telah dinonaktifkan oleh administrator.")
		return
	}

	if user.ExpiresAt != nil && time.Now().After(*user.ExpiresAt) {
		httpresponse.RespondError(w, http.StatusForbidden, "Masa Aktif Kedaluwarsa", "Masa aktif kredensial Anda telah berakhir. Hubungi administrator untuk memperpanjang akses.")
		return
	}

	_ = c.credentials.UpdateLastLogin(r.Context(), user.ID)

	resp := dto.LoginResponse{
		Token: key,
		User:  authUserDTO(user),
	}

	httpresponse.RespondJSON(w, http.StatusOK, "Autentikasi berhasil", resp)
}

func (c *AuthController) Me(w http.ResponseWriter, r *http.Request) {
	user := authcontext.UserFromContext(r.Context())
	if user == nil {
		httpresponse.RespondError(w, http.StatusUnauthorized, "Unauthorized", "Sesi tidak ditemukan atau telah berakhir.")
		return
	}

	httpresponse.RespondJSON(w, http.StatusOK, "Data profil berhasil diambil", authUserDTO(user))
}

func (c *AuthController) Logout(w http.ResponseWriter, r *http.Request) {
	if c.browser != nil {
		c.browser.Logout(w, r)
		return
	}
	httpresponse.RespondJSON(w, http.StatusOK, "Berhasil logout", map[string]bool{"success": true})
}
