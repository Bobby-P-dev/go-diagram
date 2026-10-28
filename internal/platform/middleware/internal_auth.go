package middleware

import (
	"crypto/sha256"
	"crypto/subtle"
	"net/http"
	"strings"

	"github.com/Bobby-P-dev/go-diagram.git/internal/platform/httpresponse"
)

func ValidInternalKey(expectedKey string, r *http.Request) bool {
	if r == nil {
		return false
	}
	expected := strings.TrimSpace(expectedKey)
	provided := strings.TrimSpace(r.Header.Get("X-Internal-Service-Key"))
	if expected == "" || provided == "" {
		return false
	}
	expectedHash := sha256.Sum256([]byte(expected))
	providedHash := sha256.Sum256([]byte(provided))
	return subtle.ConstantTimeCompare(expectedHash[:], providedHash[:]) == 1
}

func RequireInternalKey(expectedKey string, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !ValidInternalKey(expectedKey, r) {
			httpresponse.RespondError(w, http.StatusUnauthorized, "Unauthorized", "Invalid or missing internal service key")
			return
		}
		next(w, r)
	}
}
