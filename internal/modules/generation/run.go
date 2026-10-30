package generation

import (
	"context"
	"encoding/json"
	"errors"
	"time"
)

var (
	ErrIdempotencyRequired = errors.New("idempotency key required")
	ErrIdempotencyConflict = errors.New("idempotency key reused with different request")
	ErrRevisionConflict    = errors.New("artifact revision changed")
	ErrRunTerminal         = errors.New("generation run is terminal")
)

type StartInput struct {
	TriggerMessageID         string           `json:"trigger_message_id"`
	OutputType               string           `json:"output_type"`
	OutputTitle              string           `json:"output_title"`
	TargetArtifactID         string           `json:"target_artifact_id,omitempty"`
	TargetExpectedRevisionID string           `json:"target_expected_revision_id,omitempty"`
	Context                  ResolveInput     `json:"context"`
	Mutation                 *DiagramMutation `json:"mutation,omitempty"`
}

type Run struct {
	ID                       string           `json:"id"`
	ProjectID                string           `json:"project_id"`
	ThreadID                 string           `json:"thread_id"`
	UserID                   string           `json:"-"`
	TriggerMessageID         string           `json:"trigger_message_id"`
	OutputType               string           `json:"output_type"`
	OutputTitle              string           `json:"output_title"`
	TargetArtifactID         string           `json:"target_artifact_id,omitempty"`
	TargetExpectedRevisionID string           `json:"target_expected_revision_id,omitempty"`
	Manifest                 *Manifest        `json:"frozen_manifest"`
	Prompt                   string           `json:"-"`
	BaseDocument             *string          `json:"-"`
	BaseGraph                *DiagramGraph    `json:"-"`
	BaseGraphRaw             json.RawMessage  `json:"-"`
	Mutation                 *DiagramMutation `json:"mutation,omitempty"`
	Status                   string           `json:"status"`
	ErrorCode                string           `json:"error_code,omitempty"`
	ErrorMessage             string           `json:"error_message,omitempty"`
	CommittedArtifactID      string           `json:"committed_artifact_id,omitempty"`
	CommittedRevisionID      string           `json:"committed_revision_id,omitempty"`
	CommittedMessageID       string           `json:"committed_message_id,omitempty"`
	CreatedAt                time.Time        `json:"created_at"`
	UpdatedAt                time.Time        `json:"updated_at"`
	CompletedAt              *time.Time       `json:"completed_at,omitempty"`
}

func (run Run) Terminal() bool {
	return run.Status == "succeeded" || run.Status == "failed" || run.Status == "canceled"
}

type RunWrite struct {
	Key         string
	RequestHash string
}

type RunRepository interface {
	FindAccepted(context.Context, Scope, RunWrite) (*Run, error)
	Create(context.Context, Scope, StartInput, *Manifest, RunWrite) (*Run, error)
	Get(context.Context, Scope, string) (*Run, error)
	Claim(context.Context, Scope, string) (*Run, bool, error)
	Cancel(context.Context, Scope, string) (*Run, error)
	Finish(context.Context, Scope, string, string, string, string) error
	Commit(context.Context, Scope, string, *ArtifactCandidate) (*Run, error)
	RecoverInterrupted(context.Context) error
	Queued(context.Context) ([]Run, error)
}

type RunService interface {
	Start(context.Context, Scope, StartInput, string) (*Run, error)
	Get(context.Context, Scope, string) (*Run, error)
	Cancel(context.Context, Scope, string) (*Run, error)
}
