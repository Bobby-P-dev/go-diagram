package middleware

import (
	"net/http"
	"strings"
)

func CORSMiddleware(next http.Handler, allowedOriginsHeader string) http.Handler {
	allowedOrigins := strings.Split(allowedOriginsHeader, ",")

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")

		if origin != "" {
			for _, allowed := range allowedOrigins {
				trimmed := strings.TrimSpace(allowed)
				if trimmed == origin {
					w.Header().Set("Access-Control-Allow-Origin", origin)
					break
				}
			}
		}

		w.Header().Add("Vary", "Origin")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, PATCH, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-CSRF-Token, X-Requested-With, X-Internal-Service-Key")
		if strings.HasPrefix(r.URL.Path, "/api/v2/") {
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-CSRF-Token, Idempotency-Key, X-Requested-With")
			w.Header().Set("Access-Control-Expose-Headers", "X-Request-ID")
		}
		w.Header().Set("Access-Control-Allow-Credentials", "true")
		w.Header().Set("Access-Control-Max-Age", "86400")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}
