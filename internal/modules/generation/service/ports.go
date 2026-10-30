package service

import (
	"context"

	"github.com/Bobby-P-dev/go-diagram.git/internal/modules/generation"
)

type LLMMessage struct {
	Role    string
	Content string
}

type LLMClient interface {
	CallLLMStreamContext(ctx context.Context, systemPrompt string, history []LLMMessage, prompt string, onChunk generation.ProgressSink) (string, error)
}
