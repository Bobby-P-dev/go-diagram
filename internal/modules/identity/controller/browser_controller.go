package controller

import (
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net"
	"net/http"
	"time"

	"github.com/Bobby-P-dev/go-diagram.git/internal/modules/identity/authcontext"
	"github.com/Bobby-P-dev/go-diagram.git/internal/modules/identity/middleware"
	"github.com/Bobby-P-dev/go-diagram.git/internal/modules/identity/service"
	"github.com/Bobby-P-dev/go-diagram.git/internal/platform/httpresponse"
)

type BrowserController struct {
	sessions *service.BrowserService
	secure   bool
}

func NewBrowserController(sessions *service.BrowserService, secure bool) *BrowserController {
	return &BrowserController{sessions: sessions, secure: secure}
}

func (controller *BrowserController) Login(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || mediaType != "application/json" {
		httpresponse.RespondError(w, http.StatusUnsupportedMediaType, "Gunakan application/json.", "")
		return
	}
	var input struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	r.Body = http.MaxBytesReader(w, r.Body, 4096)
	defer r.Body.Close()
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err = decoder.Decode(&input); err != nil {
		httpresponse.RespondError(w, http.StatusBadRequest, "Format login tidak valid.", "")
		return
	}
	if err = decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		httpresponse.RespondError(w, http.StatusBadRequest, "Format login tidak valid.", "")
		return
	}
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		ip = r.RemoteAddr
	}
	previous := ""
	if cookie, cookieErr := r.Cookie(middleware.BrowserCookie); cookieErr == nil {
		previous = cookie.Value
	}
	token, user, err := controller.sessions.Login(r.Context(), input.Email, input.Password, ip, previous)
	input.Password = ""
	if err != nil {
		status := http.StatusInternalServerError
		message := "Login belum dapat diproses. Coba lagi."
		if errors.Is(err, service.ErrLogin) {
			status = http.StatusUnauthorized
			message = "Email atau kata sandi salah."
		}
		if errors.Is(err, service.ErrLoginLimit) {
			status = http.StatusTooManyRequests
			message = "Terlalu banyak percobaan login. Coba lagi dalam 15 menit."
			w.Header().Set("Retry-After", "900")
		}
		httpresponse.RespondError(w, status, message, "")
		return
	}
	controller.setCookie(w, token, 12*60*60)
	controller.respondSession(w, r, token, user)
}

func (controller *BrowserController) Session(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie(middleware.BrowserCookie)
	user := authcontext.UserFromContext(r.Context())
	if err != nil || user == nil {
		httpresponse.RespondError(w, http.StatusUnauthorized, "Sesi tidak ditemukan.", "")
		return
	}
	controller.respondSession(w, r, cookie.Value, user)
}

func (controller *BrowserController) respondSession(w http.ResponseWriter, r *http.Request, token string, user *service.CredentialRecord) {
	w.Header().Set("Cache-Control", "no-store")
	email, err := controller.sessions.Email(r.Context(), user.ID)
	if err != nil {
		httpresponse.RespondError(w, http.StatusInternalServerError, "Profil belum dapat dimuat.", "")
		return
	}
	profile := struct {
		ID                 string `json:"id"`
		Email              string `json:"email"`
		Name               string `json:"name"`
		Role               string `json:"role"`
		IsActive           bool   `json:"is_active"`
		CanGenerateDiagram bool   `json:"can_generate_diagram"`
		CanGenerateUI      bool   `json:"can_generate_ui"`
	}{user.ID, email, user.Name, user.Role, user.IsActive, user.CanGenerateDiagram, user.CanGenerateUI}
	httpresponse.RespondJSON(w, http.StatusOK, "Autentikasi berhasil", map[string]interface{}{"user": profile, "csrf_token": service.CSRFToken(token)})
}

func (controller *BrowserController) Logout(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	cookie, err := r.Cookie(middleware.BrowserCookie)
	if err == nil {
		if err = controller.sessions.Logout(r.Context(), cookie.Value); err != nil {
			httpresponse.RespondError(w, http.StatusInternalServerError, "Logout gagal. Silakan coba lagi.", "")
			return
		}
	}
	controller.setCookie(w, "", -1)
	httpresponse.RespondJSON(w, http.StatusOK, "Berhasil keluar", map[string]bool{"success": true})
}

func (controller *BrowserController) setCookie(w http.ResponseWriter, token string, maxAge int) {
	expires := time.Now().Add(12 * time.Hour)
	if maxAge < 0 {
		expires = time.Unix(1, 0)
	}
	http.SetCookie(w, &http.Cookie{Name: middleware.BrowserCookie, Value: token, Path: "/", HttpOnly: true, Secure: controller.secure, SameSite: http.SameSiteLaxMode, MaxAge: maxAge, Expires: expires})
}
