package generation

import (
	"context"
	"encoding/json"
	"errors"
)

var (
	ErrUnauthenticated = errors.New("authentication required")
	ErrInvalidInput    = errors.New("invalid context selection")
	ErrNotFound        = errors.New("context resource not found")
	ErrBudgetExceeded  = errors.New("project context exceeds content budget")
)

const (
	DefaultContentBytes = 48 * 1024
	MaxContentBytes     = 128 * 1024
	MaxSources          = 20
	MaxThreads          = 5
	CandidateLimit      = 50
	MessagesPerThread   = 20
)

type Scope struct {
	UserID    string
	ProjectID string
	ThreadID  string
}

type RevisionRef struct {
	ArtifactID string `json:"artifact_id"`
	RevisionID string `json:"revision_id,omitempty"`
}

type ResolveInput struct {
	Mode                string        `json:"mode"`
	Sources             []RevisionRef `json:"sources"`
	ThreadIDs           []string      `json:"thread_ids"`
	ExcludedArtifactIDs []string      `json:"excluded_artifact_ids"`
	MaxContentBytes     int           `json:"max_content_bytes"`
}

type ProjectContext struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Description string `json:"description"`
}

type Source struct {
	ArtifactID    string          `json:"artifact_id"`
	RevisionID    string          `json:"revision_id"`
	VersionNumber int             `json:"version_number"`
	Title         string          `json:"title"`
	Type          string          `json:"type"`
	Format        string          `json:"format"`
	Content       json.RawMessage `json:"content"`
	ContentBytes  int             `json:"-"`
}

type Message struct {
	ID             string `json:"id"`
	ThreadID       string `json:"thread_id"`
	Role           string `json:"role"`
	Content        string `json:"content"`
	SequenceNumber int64  `json:"sequence_number"`
	ContentBytes   int    `json:"-"`
}

type Omission struct {
	ArtifactID string `json:"artifact_id,omitempty"`
	RevisionID string `json:"revision_id,omitempty"`
	MessageID  string `json:"message_id,omitempty"`
	ThreadID   string `json:"thread_id,omitempty"`
	Reason     string `json:"reason"`
}

type Budget struct {
	MaxContentBytes  int `json:"max_content_bytes"`
	UsedContentBytes int `json:"used_content_bytes"`
}

type Manifest struct {
	SchemaVersion           int            `json:"schema_version"`
	Mode                    string         `json:"mode"`
	Project                 ProjectContext `json:"project"`
	ThreadID                string         `json:"thread_id"`
	Sources                 []Source       `json:"sources"`
	Messages                []Message      `json:"messages"`
	Omitted                 []Omission     `json:"omitted"`
	Budget                  Budget         `json:"budget"`
	CandidateLimitReached   bool           `json:"candidate_limit_reached"`
	HistoryLimitedThreadIDs []string       `json:"history_limited_thread_ids"`
	PreviewOnly             bool           `json:"preview_only"`
}

type Snapshot struct {
	Project                 ProjectContext
	Sources                 []Source
	Messages                []Message
	CandidateLimitReached   bool
	HistoryLimitedThreadIDs []string
}

type ContextReader interface {
	ReadSnapshot(context.Context, Scope, ResolveInput) (*Snapshot, error)
}

type ContextResolver interface {
	Resolve(context.Context, Scope, ResolveInput) (*Manifest, error)
}
