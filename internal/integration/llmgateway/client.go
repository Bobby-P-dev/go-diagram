package llmgateway

import (
	"context"

	"github.com/Bobby-P-dev/go-diagram.git/internal/modules/generation"
	generationservice "github.com/Bobby-P-dev/go-diagram.git/internal/modules/generation/service"
	"github.com/Bobby-P-dev/go-diagram.git/internal/platform/llm"
)

type Gateway struct {
	client *llm.Client
}

var _ generationservice.LLMClient = (*Gateway)(nil)

func New(client *llm.Client) *Gateway {
	return &Gateway{client: client}
}

func (gateway *Gateway) CallLLMStreamContext(
	ctx context.Context,
	systemPrompt string,
	history []generationservice.LLMMessage,
	prompt string,
	onChunk generation.ProgressSink,
) (string, error) {
	if gateway == nil || gateway.client == nil {
		return "", generation.ErrEngineUnavailable
	}
	llmMessages := make([]llm.Message, len(history))
	for index, message := range history {
		llmMessages[index] = llm.Message{Role: message.Role, Content: message.Content}
	}
	var callback llm.LLMStreamCallback
	if onChunk != nil {
		callback = func(delta string, tokenCount int) {
			onChunk(delta, tokenCount)
		}
	}
	return gateway.client.CallLLMStreamContext(ctx, systemPrompt, llmMessages, prompt, callback)
}
