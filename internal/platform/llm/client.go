package llm

import (
	"net/http"
	"strings"
	"time"
)

type Config struct {
	AIProvider       string
	OpenAIAPIKey     string
	OpenAIModel      string
	OpenAIMBaseURL   string
	AnthropicAPIKey  string
	AnthropicModel   string
	AnthropicBaseURL string
	AIMaxTokens      int
}

type Message struct {
	Role    string
	Content string
}

type Client struct {
	client    *http.Client
	provider  string
	apiKey    string
	model     string
	baseURL   string
	maxTokens int
}

func NewClient(config Config, client *http.Client) *Client {
	provider := strings.ToLower(strings.TrimSpace(config.AIProvider))
	if provider == "" {
		if config.OpenAIAPIKey != "" {
			provider = "openai"
		} else {
			provider = "anthropic"
		}
	}

	var apiKey, model, baseURL string
	if provider == "openai" {
		apiKey = config.OpenAIAPIKey
		model = config.OpenAIModel
		baseURL = config.OpenAIMBaseURL
		if !strings.HasSuffix(baseURL, "/chat/completions") {
			baseURL = strings.TrimSuffix(baseURL, "/") + "/chat/completions"
		}
	} else {
		provider = "anthropic"
		apiKey = config.AnthropicAPIKey
		model = config.AnthropicModel
		baseURL = config.AnthropicBaseURL
	}

	maxTokens := config.AIMaxTokens
	if maxTokens <= 0 {
		maxTokens = 8192
	}
	// TokenomixAI and similar aggregator proxies return 502 Bad Gateway when max_tokens > 8192
	if maxTokens > 8192 && strings.Contains(strings.ToLower(baseURL), "tokenomixai") {
		maxTokens = 8192
	}

	if client == nil {
		client = &http.Client{Timeout: 300 * time.Second}
	}
	return &Client{
		client:    client,
		provider:  provider,
		apiKey:    apiKey,
		model:     model,
		baseURL:   baseURL,
		maxTokens: maxTokens,
	}
}

func (client *Client) Provider() string { return client.provider }
func (client *Client) Model() string    { return client.model }
