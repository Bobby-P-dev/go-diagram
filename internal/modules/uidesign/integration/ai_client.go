package integration

import (
	uidesignservice "github.com/Bobby-P-dev/go-diagram.git/internal/modules/uidesign/service"
	"github.com/Bobby-P-dev/go-diagram.git/internal/platform/llm"
)

type CompletionClient interface {
	CallLLM(string, []llm.Message, string) (string, error)
	CallLLMText(string, []llm.Message, string) (string, error)
	CallLLMStream(string, []llm.Message, string, llm.LLMStreamCallback) (string, error)
	CallLLMTextStream(string, []llm.Message, string, llm.LLMStreamCallback) (string, error)
	SanitizeJSON(string) string
}

type uiDesignAIClient struct{ service CompletionClient }

var _ uidesignservice.LLMClient = (*uiDesignAIClient)(nil)

func NewUIDesignAIClient(service CompletionClient) uidesignservice.LLMClient {
	if service == nil {
		return nil
	}
	return &uiDesignAIClient{service: service}
}

func (client *uiDesignAIClient) CallLLM(systemPrompt, prompt string) (string, error) {
	return client.service.CallLLM(systemPrompt, nil, prompt)
}
func (client *uiDesignAIClient) CallLLMText(systemPrompt, prompt string) (string, error) {
	return client.service.CallLLMText(systemPrompt, nil, prompt)
}
func (client *uiDesignAIClient) CallLLMStream(systemPrompt, prompt string, callback uidesignservice.LLMStreamCallback) (string, error) {
	return client.service.CallLLMStream(systemPrompt, nil, prompt, llm.LLMStreamCallback(callback))
}
func (client *uiDesignAIClient) CallLLMTextStream(systemPrompt, prompt string, callback uidesignservice.LLMStreamCallback) (string, error) {
	return client.service.CallLLMTextStream(systemPrompt, nil, prompt, llm.LLMStreamCallback(callback))
}
func (client *uiDesignAIClient) SanitizeJSON(raw string) string {
	return client.service.SanitizeJSON(raw)
}
