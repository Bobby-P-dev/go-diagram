package middlewares

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/Bobby-P-dev/go-diagram.git/src/entities"
	"github.com/Bobby-P-dev/go-diagram.git/src/models"
	"github.com/Bobby-P-dev/go-diagram.git/src/utils"
)

type contextKey string

const UserContextKey contextKey = "auth_user"

func GetUserFromContext(ctx context.Context) *entities.AccessCredential {
	if ctx == nil {
		return nil
	}
	if u, ok := ctx.Value(UserContextKey).(*entities.AccessCredential); ok {
		return u
	}
	return nil
}

func isPublicPath(path, method string) bool {
	if method == http.MethodOptions {
		return true
	}
	if path == "/health" || path == "/" {
		return true
	}
	if path == "/api/auth/verify" && method == http.MethodPost {
		return true
	}
	// Public share view is read-only GET. Forking requires authentication.
	if strings.HasPrefix(path, "/api/shared/") && method == http.MethodGet {
		return true
	}
	if path == "/internal/v1/jobs/callback" {
		return true
	}
	return false
}

func AuthMiddleware(credModel models.CredentialModelInterface) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			path := r.URL.Path

			// Allow public endpoints
			if isPublicPath(path, r.Method) {
				// Even if public, if token is provided, attempt to inject user for convenience (e.g. fork or me)
				authHeader := r.Header.Get("Authorization")
				if authHeader != "" {
					token := strings.TrimSpace(strings.TrimPrefix(authHeader, "Bearer "))
					if user, err := credModel.FindByKey(token); err == nil && user.IsActive {
						if user.ExpiresAt == nil || time.Now().Before(*user.ExpiresAt) {
							ctx := context.WithValue(r.Context(), UserContextKey, user)
							r = r.WithContext(ctx)
						}
					}
				}
				next.ServeHTTP(w, r)
				return
			}

			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				utils.RespondError(w, http.StatusUnauthorized, "Autentikasi Diperlukan", "Silakan masukkan kredensial akses Anda untuk melanjutkan.")
				return
			}

			token := strings.TrimSpace(strings.TrimPrefix(authHeader, "Bearer "))
			if token == "" {
				utils.RespondError(w, http.StatusUnauthorized, "Kredensial Tidak Valid", "Format Authorization Bearer token kosong.")
				return
			}

			user, err := credModel.FindByKey(token)
			if err != nil {
				utils.RespondError(w, http.StatusUnauthorized, "Kredensial Tidak Ditemukan", "Kredensial akses yang Anda masukkan tidak valid atau tidak terdaftar.")
				return
			}

			if !user.IsActive {
				utils.RespondError(w, http.StatusForbidden, "Akses Dinonaktifkan", "Kredensial ini telah dinonaktifkan oleh administrator.")
				return
			}

			if user.ExpiresAt != nil && time.Now().After(*user.ExpiresAt) {
				utils.RespondError(w, http.StatusForbidden, "Masa Aktif Kedaluwarsa", "Masa aktif kredensial Anda telah berakhir. Hubungi administrator untuk memperpanjang akses.")
				return
			}

			// Inject user into request context
			ctx := context.WithValue(r.Context(), UserContextKey, user)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// RequireAdmin verifies that the authenticated user has the 'admin' role
func RequireAdmin(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user := GetUserFromContext(r.Context())
		if user == nil || user.Role != "admin" {
			utils.RespondError(w, http.StatusForbidden, "Akses Ditolak", "Hanya administrator yang diizinkan mengakses menu ini.")
			return
		}
		next(w, r)
	}
}
