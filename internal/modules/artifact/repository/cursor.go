package repository

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"time"

	"github.com/Bobby-P-dev/go-diagram.git/internal/modules/artifact"
)

type TimeIDCursor struct {
	Scope string    `json:"scope"`
	Time  time.Time `json:"t"`
	ID    string    `json:"id"`
}

type VerCursor struct {
	Scope   string `json:"scope"`
	Version int    `json:"v"`
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
	if payload.Scope != scope || !artifact.ValidID(payload.ID) || payload.Time.IsZero() || payload.Time.Year() < 1 || payload.Time.Year() > 9999 {
		return time.Time{}, "", fmt.Errorf("invalid cursor fields")
	}
	return payload.Time, payload.ID, nil
}

func encodeVerCursor(scope string, version int) string {
	data, _ := json.Marshal(VerCursor{Scope: scope, Version: version})
	return base64.RawURLEncoding.EncodeToString(data)
}

func decodeVerCursor(scope, cursor string) (int, error) {
	if cursor == "" {
		return 0, nil
	}
	bytes, err := base64.RawURLEncoding.DecodeString(cursor)
	if err != nil {
		return 0, fmt.Errorf("invalid cursor: %w", err)
	}
	var payload VerCursor
	if err := json.Unmarshal(bytes, &payload); err != nil {
		return 0, fmt.Errorf("invalid cursor payload: %w", err)
	}
	if payload.Scope != scope || payload.Version < 1 {
		return 0, artifact.ErrInvalidInput
	}
	return payload.Version, nil
}
