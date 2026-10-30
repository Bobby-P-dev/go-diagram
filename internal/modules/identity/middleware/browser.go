package middleware

import (
	"crypto/subtle"
	"net/http"
	"strings"

	"github.com/Bobby-P-dev/go-diagram.git/internal/modules/identity/authcontext"
	"github.com/Bobby-P-dev/go-diagram.git/internal/modules/identity/service"
	"github.com/Bobby-P-dev/go-diagram.git/internal/platform/httpresponse"
)

const BrowserCookie = "vw_session"

func BrowserMiddleware(sessions *service.BrowserService, credentials CredentialFinder, origins string, legacyEnabled bool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		legacy := AuthMiddleware(credentials)(next)
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if strings.HasPrefix(r.URL.Path, "/api/auth/") {
				w.Header().Set("Cache-Control", "no-store")
			}
			unsafe := r.Method != http.MethodGet && r.Method != http.MethodHead && r.Method != http.MethodOptions
			if unsafe && !AllowedBrowserOrigin(r, origins) {
				browserError(w, r, http.StatusForbidden, "CSRF_REJECTED", "Asal permintaan tidak diizinkan.")
				return
			}
			if r.URL.Path == "/api/auth/login" && r.Method == http.MethodPost {
				next.ServeHTTP(w, r)
				return
			}
			if r.URL.Path == "/api/auth/verify" && !legacyEnabled {
				http.NotFound(w, r)
				return
			}
			if legacyEnabled && r.Header.Get("Authorization") != "" {
				legacy.ServeHTTP(w, r)
				return
			}
			if isPublicPath(r.URL.Path, r.Method) {
				next.ServeHTTP(w, r)
				return
			}
			cookie, err := r.Cookie(BrowserCookie)
			if err != nil {
				browserError(w, r, http.StatusUnauthorized, "UNAUTHENTICATED", "Silakan masuk untuk melanjutkan.")
				return
			}
			user, err := sessions.Authenticate(r.Context(), cookie.Value)
			if err != nil {
				browserError(w, r, http.StatusUnauthorized, "UNAUTHENTICATED", "Sesi tidak valid atau telah berakhir.")
				return
			}
			if unsafe {
				expected := service.CSRFToken(cookie.Value)
				if subtle.ConstantTimeCompare([]byte(expected), []byte(r.Header.Get("X-CSRF-Token"))) != 1 {
					browserError(w, r, http.StatusForbidden, "CSRF_REJECTED", "Token keamanan tidak valid. Muat ulang halaman.")
					return
				}
			}
			next.ServeHTTP(w, r.WithContext(authcontext.WithUser(r.Context(), user)))
		})
	}
}

func AllowedBrowserOrigin(r *http.Request, origins string) bool {
	if r.Header.Get("Sec-Fetch-Site") == "cross-site" {
		return false
	}
	origin := r.Header.Get("Origin")
	if origin == "" {
		return true
	}
	for _, allowed := range strings.Split(origins, ",") {
		if origin == strings.TrimSpace(allowed) {
			return true
		}
	}
	return false
}

func browserError(w http.ResponseWriter, r *http.Request, status int, code, message string) {
	if strings.HasPrefix(r.URL.Path, "/api/v2/") {
		httpresponse.RespondV2Error(w, status, code, message)
		return
	}
	httpresponse.RespondError(w, status, message, "")
}
