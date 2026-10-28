package llm

type MessageOpenAI struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}
type ResponseFormatOpenAI struct {
	Type string `json:"type"`
}

type ChatRequestOpenAI struct {
	Model          string                `json:"model"`
	Messages       []MessageOpenAI       `json:"messages"`
	Stream         bool                  `json:"stream"`
	MaxTokens      int                   `json:"max_tokens,omitempty"`
	ResponseFormat *ResponseFormatOpenAI `json:"response_format,omitempty"`
}

type OpenAIError struct {
	Message string `json:"message"`
	Type    string `json:"type"`
	Code    string `json:"code"`
}

type ChatResponseOpenAI struct {
	Choices []struct {
		FinishReason string `json:"finish_reason"`
		Message      struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
	Error *OpenAIError `json:"error,omitempty"`
}

type anthropicRequest struct {
	Model     string              `json:"model"`
	MaxTokens int                 `json:"max_tokens"`
	System    string              `json:"system"`
	Messages  []map[string]string `json:"messages"`
}

type anthropicResponse struct {
	Content []anthropicContent `json:"content"`
	Error   *anthropicError    `json:"error,omitempty"`
}

type anthropicContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type anthropicError struct {
	Type    string `json:"type"`
	Message string `json:"message"`
}
