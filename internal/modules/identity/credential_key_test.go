package identity

import (
	"strings"
	"testing"
	"time"
)

func TestHashCredentialKeyProperties(t *testing.T) {
	firstHash := HashCredentialKey("RL-TEST-KEY-1")
	trimmedHash := HashCredentialKey("  RL-TEST-KEY-1  ")
	if firstHash != trimmedHash {
		t.Fatalf("expected whitespace trimming in hashing")
	}
	if len(firstHash) != 64 {
		t.Fatalf("expected 64 character sha256 hex, got length %d", len(firstHash))
	}
	otherHash := HashCredentialKey("RL-TEST-KEY-2")
	if firstHash == otherHash {
		t.Fatalf("different keys must produce different digests")
	}
}

func TestCredentialKeyPrefixRunes(t *testing.T) {
	if prefix := CredentialKeyPrefix("RL-1234567890"); prefix != "RL-12345" {
		t.Fatalf("expected first 8 runes, got %q", prefix)
	}
	if prefix := CredentialKeyPrefix("RL-SHORT"); prefix != "RL-SHORT" {
		t.Fatalf("expected short prefix preserved, got %q", prefix)
	}
}

func TestMaskCredentialKeyNeverRevealsSecret(t *testing.T) {
	if mask := MaskCredentialKey(""); mask != "********" {
		t.Fatalf("expected default mask for empty prefix, got %q", mask)
	}
	if mask := MaskCredentialKey(HiddenCredentialPrefix); mask != "********" {
		t.Fatalf("expected default mask for hidden prefix, got %q", mask)
	}
	if mask := MaskCredentialKey("RL-ABCDE"); mask != "RL-ABCDE********" {
		t.Fatalf("expected prefix with stars, got %q", mask)
	}
}

func TestGenerateRandomKeyFormat(t *testing.T) {
	key1, err := GenerateRandomKey()
	if err != nil {
		t.Fatalf("unexpected generation error: %v", err)
	}
	key2, err := GenerateRandomKey()
	if err != nil {
		t.Fatalf("unexpected generation error: %v", err)
	}
	if !strings.HasPrefix(key1, "RL-") || !strings.HasPrefix(key2, "RL-") {
		t.Fatalf("expected RL- prefix on generated keys")
	}
	if key1 == key2 {
		t.Fatalf("consecutive generated keys must not collide")
	}
	if len(key1) != 46 {
		t.Fatalf("expected 46 character key, got %d (%q)", len(key1), key1)
	}
}

func TestFormatExpiresInHumanDescriptions(t *testing.T) {
	text, expired := FormatExpiresInHuman(nil)
	if text != "Permanen (Tanpa Batas)" || expired {
		t.Fatalf("expected permanent for nil expiry: %q, %v", text, expired)
	}

	past := time.Now().Add(-5 * time.Minute)
	text, expired = FormatExpiresInHuman(&past)
	if text != "Kedaluwarsa" || !expired {
		t.Fatalf("expected expired for past timestamp: %q, %v", text, expired)
	}

	underOneMin := time.Now().Add(30 * time.Second)
	text, expired = FormatExpiresInHuman(&underOneMin)
	if text != "Sisa < 1 menit" || expired {
		t.Fatalf("expected <1 min: %q, %v", text, expired)
	}

	minutesAhead := time.Now().Add(25 * time.Minute)
	text, expired = FormatExpiresInHuman(&minutesAhead)
	if !strings.Contains(text, "menit") || expired {
		t.Fatalf("expected minutes remaining: %q, %v", text, expired)
	}

	hoursAhead := time.Now().Add(5 * time.Hour)
	text, expired = FormatExpiresInHuman(&hoursAhead)
	if !strings.Contains(text, "jam") || expired {
		t.Fatalf("expected hours remaining: %q, %v", text, expired)
	}

	daysAhead := time.Now().Add(72 * time.Hour)
	text, expired = FormatExpiresInHuman(&daysAhead)
	if !strings.Contains(text, "hari") || expired {
		t.Fatalf("expected days remaining: %q, %v", text, expired)
	}
}
