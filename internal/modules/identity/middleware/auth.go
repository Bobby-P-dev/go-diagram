package middleware

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/Bobby-P-dev/go-diagram.git/internal/modules/identity/authcontext"
	"github.com/Bobby-P-dev/go-diagram.git/internal/modules/identity/service"
	"github.com/Bobby-P-dev/go-diagram.git/internal/platform/httpresponse"
)

type CredentialFinder interface {
	FindByKey(ctx context.Context, key string) (*service.CredentialRecord, error)
}

func isPublicPath(path, method string) bool {
	if method == http.MethodOptions || path == "/health" || path == "/" {
		return true
	}
	if path == "/api/auth/verify" && method == http.MethodPost {
		return true
	}
	if strings.HasPrefix(path, "/api/shared/") && method == http.MethodGet {
		return true
	}
	return path == "/internal/v1/jobs/callback"
}

func AuthMiddleware(credentials CredentialFinder) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			token := strings.TrimSpace(strings.TrimPrefix(authHeader, "Bearer "))
			if strings.HasPrefix(r.URL.Path, "/api/v2/") && r.Method != http.MethodOptions {
				parts := strings.Fields(authHeader)
				if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
					httpresponse.RespondV2Error(w, http.StatusUnauthorized, "UNAUTHENTICATED", "Bearer authentication required")
					return
				}
				user, err := credentials.FindByKey(r.Context(), parts[1])
				if err != nil || user == nil || !user.IsActive || (user.ExpiresAt != nil && !time.Now().Before(*user.ExpiresAt)) {
					httpresponse.RespondV2Error(w, http.StatusUnauthorized, "UNAUTHENTICATED", "Invalid or expired credentials")
					return
				}
				next.ServeHTTP(w, r.WithContext(authcontext.WithUser(r.Context(), user)))
				return
			}
			if isPublicPath(r.URL.Path, r.Method) {
				if authHeader != "" {
					user, err := credentials.FindByKey(r.Context(), token)
					if err == nil && user.IsActive && (user.ExpiresAt == nil || time.Now().Before(*user.ExpiresAt)) {
						r = r.WithContext(authcontext.WithUser(r.Context(), user))
					}
				}
				next.ServeHTTP(w, r)
				return
			}
			if authHeader == "" {
				httpresponse.RespondError(w, http.StatusUnauthorized, "Autentikasi Diperlukan", "Silakan masukkan kredensial akses Anda untuk melanjutkan.")
				return
			}
			if token == "" {
				httpresponse.RespondError(w, http.StatusUnauthorized, "Kredensial Tidak Valid", "Format Authorization Bearer token kosong.")
				return
			}
			user, err := credentials.FindByKey(r.Context(), token)
			if err != nil {
				httpresponse.RespondError(w, http.StatusUnauthorized, "Kredensial Tidak Ditemukan", "Kredensial akses yang Anda masukkan tidak valid atau tidak terdaftar.")
				return
			}
			if !user.IsActive {
				httpresponse.RespondError(w, http.StatusForbidden, "Akses Dinonaktifkan", "Kredensial ini telah dinonaktifkan oleh administrator.")
				return
			}
			if user.ExpiresAt != nil && time.Now().After(*user.ExpiresAt) {
				httpresponse.RespondError(w, http.StatusForbidden, "Masa Aktif Kedaluwarsa", "Masa aktif kredensial Anda telah berakhir. Hubungi administrator untuk memperpanjang akses.")
				return
			}
			next.ServeHTTP(w, r.WithContext(authcontext.WithUser(r.Context(), user)))
		})
	}
}

func RequireAdmin(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user := authcontext.UserFromContext(r.Context())
		if user == nil || user.Role != "admin" {
			httpresponse.RespondError(w, http.StatusForbidden, "Akses Ditolak", "Hanya administrator yang diizinkan mengakses menu ini.")
			return
		}
		next(w, r)
	}
}
