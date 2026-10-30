package repository

import (
	"errors"
	"testing"
	"time"

	"github.com/Bobby-P-dev/go-diagram.git/internal/modules/artifact"
)

func TestCursorEncodingAndDecoding(test *testing.T) {
	scope := "user-1/proj-1/artifacts"
	now := time.Date(2026, 10, 28, 12, 0, 0, 0, time.UTC)
	validID := "11111111-1111-1111-1111-111111111111"

	test.Run("empty cursor returns zero time and empty id", func(test *testing.T) {
		tm, id, err := decodeTimeIDCursor(scope, "")
		if err != nil || !tm.IsZero() || id != "" {
			test.Fatalf("expected empty cursor result, got time=%v, id=%s, err=%v", tm, id, err)
		}
	})

	test.Run("valid time id cursor roundtrip", func(test *testing.T) {
		encoded := encodeTimeIDCursor(scope, now, validID)
		tm, id, err := decodeTimeIDCursor(scope, encoded)
		if err != nil {
			test.Fatalf("unexpected error: %v", err)
		}
		if !tm.Equal(now) || id != validID {
			test.Fatalf("mismatch: got time=%v, id=%s; want time=%v, id=%s", tm, id, now, validID)
		}
	})

	test.Run("scope mismatch fails", func(test *testing.T) {
		encoded := encodeTimeIDCursor("different-scope", now, validID)
		_, _, err := decodeTimeIDCursor(scope, encoded)
		if err == nil {
			test.Fatal("expected scope mismatch error, got nil")
		}
	})

	test.Run("version cursor roundtrip", func(test *testing.T) {
		verScope := "user-1/proj-1/art-1/revisions"
		encoded := encodeVerCursor(verScope, 5)
		value, err := decodeVerCursor(verScope, encoded)
		if err != nil {
			test.Fatalf("unexpected error: %v", err)
		}
		if value != 5 {
			test.Fatalf("expected 5, got %d", value)
		}
	})

	test.Run("version cursor invalid", func(test *testing.T) {
		verScope := "user-1/proj-1/art-1/revisions"
		encoded := encodeVerCursor(verScope, 0)
		_, err := decodeVerCursor(verScope, encoded)
		if !errors.Is(err, artifact.ErrInvalidInput) {
			test.Fatalf("expected ErrInvalidInput, got %v", err)
		}
	})
}
