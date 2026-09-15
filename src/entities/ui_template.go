package entities

import (
	"encoding/json"
	"time"
)

type UITemplate struct {
	ID          string          `json:"id"`
	Name        string          `json:"name"`
	Category    string          `json:"category"`
	Device      string          `json:"device"`
	Description string          `json:"description"`
	Theme       json.RawMessage `json:"theme"`
	Sections    json.RawMessage `json:"sections"`
	CodeExport  json.RawMessage `json:"code_export"`
	IsFeatured  bool            `json:"is_featured"`
	CreatedAt   time.Time       `json:"created_at"`
}
