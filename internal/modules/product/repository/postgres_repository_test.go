package repository

import (
	"errors"
	"testing"
	"time"

	"github.com/Bobby-P-dev/go-diagram.git/internal/modules/product"
)

func TestCursorEncodingAndDecoding(test *testing.T) {
	scope := "user-1/projects"
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

	test.Run("invalid base64 fails", func(test *testing.T) {
		_, _, err := decodeTimeIDCursor(scope, "invalid-base64!@#$")
		if err == nil {
			test.Fatal("expected base64 error, got nil")
		}
	})

	test.Run("invalid id format fails", func(test *testing.T) {
		encoded := encodeTimeIDCursor(scope, now, "not-a-valid-uuid")
		_, _, err := decodeTimeIDCursor(scope, encoded)
		if err == nil {
			test.Fatal("expected invalid id error, got nil")
		}
	})

	test.Run("seq cursor roundtrip", func(test *testing.T) {
		seqScope := "user-1/thread-1/messages"
		encoded := encodeSeqCursor(seqScope, 42)
		seq, err := decodeSeqCursor(seqScope, encoded)
		if err != nil {
			test.Fatalf("unexpected error: %v", err)
		}
		if seq != 42 {
			test.Fatalf("expected 42, got %d", seq)
		}
	})

	test.Run("seq cursor scope mismatch or negative fails", func(test *testing.T) {
		seqScope := "user-1/thread-1/messages"
		encoded := encodeSeqCursor("different", 42)
		_, err := decodeSeqCursor(seqScope, encoded)
		if !errors.Is(err, product.ErrInvalidInput) {
			test.Fatalf("expected ErrInvalidInput, got %v", err)
		}

		encodedBad := encodeSeqCursor(seqScope, 0)
		_, err = decodeSeqCursor(seqScope, encodedBad)
		if !errors.Is(err, product.ErrInvalidInput) {
			test.Fatalf("expected ErrInvalidInput, got %v", err)
		}
	})
}
