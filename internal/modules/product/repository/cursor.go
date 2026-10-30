package repository

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"time"

	"github.com/Bobby-P-dev/go-diagram.git/internal/modules/product"
)

type TimeIDCursor struct {
	Scope string    `json:"scope"`
	Time  time.Time `json:"t"`
	ID    string    `json:"id"`
}

type SeqCursor struct {
	Scope    string `json:"scope"`
	Sequence int64  `json:"s"`
}

func encodeTimeIDCursor(scope string, timestamp time.Time, id string) string {
	data, _ := json.Marshal(TimeIDCursor{Scope: scope, Time: timestamp.UTC(), ID: id})
	return base64.RawURLEncoding.EncodeToString(data)
}

func decodeTimeIDCursor(scope, cursor string) (time.Time, string, error) {
	if cursor == "" {
		return time.Time{}, "", nil
	}
	bytes, err := base64.RawURLEncoding.DecodeString(cursor)
	if err != nil {
		return time.Time{}, "", fmt.Errorf("invalid cursor: %w", err)
	}
	var payload TimeIDCursor
	if err := json.Unmarshal(bytes, &payload); err != nil {
		return time.Time{}, "", fmt.Errorf("invalid cursor payload: %w", err)
	}
	if payload.Scope != scope || !product.ValidID(payload.ID) || payload.Time.IsZero() || payload.Time.Year() < 1 || payload.Time.Year() > 9999 {
		return time.Time{}, "", fmt.Errorf("invalid cursor fields")
	}
	return payload.Time, payload.ID, nil
}

func encodeSeqCursor(scope string, seq int64) string {
	data, _ := json.Marshal(SeqCursor{Scope: scope, Sequence: seq})
	return base64.RawURLEncoding.EncodeToString(data)
}

func decodeSeqCursor(scope, cursor string) (int64, error) {
	if cursor == "" {
		return 0, nil
	}
	bytes, err := base64.RawURLEncoding.DecodeString(cursor)
	if err != nil {
		return 0, fmt.Errorf("invalid cursor: %w", err)
	}
	var payload SeqCursor
	if err := json.Unmarshal(bytes, &payload); err != nil {
		return 0, fmt.Errorf("invalid cursor payload: %w", err)
	}
	if payload.Scope != scope || payload.Sequence < 1 {
		return 0, product.ErrInvalidInput
	}
	return payload.Sequence, nil
}
