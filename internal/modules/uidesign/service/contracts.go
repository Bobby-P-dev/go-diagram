package service

import (
	"context"

	"github.com/Bobby-P-dev/go-diagram.git/internal/modules/uidesign"
	dto "github.com/Bobby-P-dev/go-diagram.git/internal/modules/uidesign/dto"
)

type JobBindingRepository interface {
	Create(ctx context.Context, binding uidesign.AsyncJobBinding) error
	GetByID(ctx context.Context, jobID string) (*uidesign.AsyncJobBinding, error)
}

type JobStatusStore interface {
	SetStatus(ctx context.Context, jobID, status string) error
	GetStatus(ctx context.Context, jobID string) (string, error)
	GetJobProject(ctx context.Context, jobID string) (string, error)
}

type JobQueue interface {
	Enqueue(ctx context.Context, payload []byte) error
}

type SidecarDispatcher interface {
	Dispatch(ctx context.Context, payload []byte) error
}

type JobDispatcherInterface interface {
	DispatchUIDesignJob(ctx context.Context, req dto.CreateUIDesignRequest, userID string) (*dto.AsyncJobInitResponse, error)
	GetJobStatus(ctx context.Context, jobID, userID string) (*dto.JobStatusResponse, error)
	ProcessJobCallback(ctx context.Context, req dto.JobCallbackRequest) error
}

type LLMStreamCallback func(delta string, totalTokens int)

type LLMClient interface {
	CallLLM(systemPrompt, prompt string) (string, error)
	CallLLMText(systemPrompt, prompt string) (string, error)
	CallLLMStream(systemPrompt, prompt string, onChunk LLMStreamCallback) (string, error)
	CallLLMTextStream(systemPrompt, prompt string, onChunk LLMStreamCallback) (string, error)
	SanitizeJSON(raw string) string
}

type UserIDReader func(context.Context) string
