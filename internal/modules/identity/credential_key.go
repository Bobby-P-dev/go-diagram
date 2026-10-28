// Package identity contains credential policy and token helpers.
package identity

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"strings"
	"time"
)

// MasterAdminID is the fixed UUID reserved for the initial system administrator.
const MasterAdminID = "a0000000-0000-0000-0000-000000000001"

// HiddenCredentialPrefix marks legacy rows whose plaintext key was migrated.
const HiddenCredentialPrefix = "hidden"

// HashCredentialKey returns a deterministic SHA-256 digest for a bearer token.
func HashCredentialKey(key string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(key)))
	return hex.EncodeToString(sum[:])
}

// CredentialKeyPrefix returns a short, non-secret display prefix.
func CredentialKeyPrefix(key string) string {
	runes := []rune(strings.TrimSpace(key))
	if len(runes) > 8 {
		runes = runes[:8]
	}
	return string(runes)
}

// MaskCredentialKey returns a display-only value that cannot authenticate.
func MaskCredentialKey(prefix string) string {
	prefix = strings.TrimSpace(prefix)
	if prefix == "" || prefix == HiddenCredentialPrefix {
		return "********"
	}
	return prefix + "********"
}

// GenerateRandomKey creates a cryptographically random bearer token.
func GenerateRandomKey() (string, error) {
	randomBytes := make([]byte, 32)
	if _, err := rand.Read(randomBytes); err != nil {
		return "", fmt.Errorf("generate credential key: %w", err)
	}
	return "RL-" + base64.RawURLEncoding.EncodeToString(randomBytes), nil
}

// FormatExpiresInHuman describes an expiry for API and admin display.
func FormatExpiresInHuman(expiresAt *time.Time) (string, bool) {
	if expiresAt == nil {
		return "Permanen (Tanpa Batas)", false
	}
	now := time.Now()
	if now.After(*expiresAt) {
		return "Kedaluwarsa", true
	}
	diff := expiresAt.Sub(now)
	if diff < time.Hour {
		mins := int(diff.Minutes())
		if mins <= 1 {
			return "Sisa < 1 menit", false
		}
		return fmt.Sprintf("Sisa %d menit", mins), false
	}
	if diff < 24*time.Hour {
		return fmt.Sprintf("Sisa %d jam", int(diff.Hours())), false
	}
	days := int(diff.Hours() / 24)
	return fmt.Sprintf("Sisa %d hari", days), false
}
