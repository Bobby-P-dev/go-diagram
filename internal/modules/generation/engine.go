package generation

import (
	"context"
	"errors"
)

var (
	ErrEngineUnavailable = errors.New("generation engine unavailable")
	ErrUnsupportedType   = errors.New("unsupported generation artifact type")
	ErrEmptyPrompt       = errors.New("prompt cannot be empty")
	ErrRepairFailed      = errors.New("failed to generate valid structured document output")
	ErrInvalidDocument   = errors.New("invalid structured document output")
	ErrDocumentTooLarge  = errors.New("document output exceeds size limit")
)

type DocumentInput struct {
	Type         string    `json:"type"`
	Title        string    `json:"title"`
	Prompt       string    `json:"prompt"`
	BaseDocument *string   `json:"base_document,omitempty"`
	Manifest     *Manifest `json:"manifest"`
}

type DocumentCandidate struct {
	Type             string        `json:"type"`
	Title            string        `json:"title"`
	DocumentContent  string        `json:"document_content"`
	ChangeSummary    string        `json:"change_summary"`
	AssistantMessage string        `json:"assistant_message"`
	Format           string        `json:"format"`
	SchemaVersion    int           `json:"schema_version"`
	Sources          []RevisionRef `json:"sources"`
}

type ProgressSink func(delta string, tokenCount int)

type DocumentEngine interface {
	GenerateDocument(ctx context.Context, input DocumentInput, emit ProgressSink) (*DocumentCandidate, error)
}
