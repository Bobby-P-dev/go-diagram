package llm

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
)

type LLMStreamCallback func(delta string, totalTokens int)

func (s *Client) callOpenAI(
	systemPrompt string,
	historyMessages []Message,
	newPrompt string,
	responseFormat *ResponseFormatOpenAI,
) (string, error) {
	return s.callOpenAIWithCallback(systemPrompt, historyMessages, newPrompt, responseFormat, nil)
}

func (s *Client) callOpenAIWithCallback(
	systemPrompt string,
	historyMessages []Message,
	newPrompt string,
	responseFormat *ResponseFormatOpenAI,
	onChunk LLMStreamCallback,
) (string, error) {
	return s.callOpenAIWithContext(context.Background(), systemPrompt, historyMessages, newPrompt, responseFormat, onChunk)
}

func (s *Client) callOpenAIWithContext(ctx context.Context, systemPrompt string, historyMessages []Message, newPrompt string, responseFormat *ResponseFormatOpenAI, onChunk LLMStreamCallback) (string, error) {
	// Di OpenAI, System Prompt dimasukkan sebagai message pertama dengan role "system"
	var openAIMessages []MessageOpenAI
	openAIMessages = append(openAIMessages, MessageOpenAI{
		Role:    "system",
		Content: systemPrompt,
	})

	for _, msg := range historyMessages {
		role := msg.Role
		if role != "user" && role != "assistant" && role != "system" {
			role = "user"
		}
		openAIMessages = append(openAIMessages, MessageOpenAI{
			Role:    role,
			Content: msg.Content,
		})
	}

	if strings.TrimSpace(newPrompt) != "" {
		openAIMessages = append(openAIMessages, MessageOpenAI{
			Role:    "user",
			Content: strings.TrimSpace(newPrompt),
		})
	}

	type chatCompletionChunk struct {
		Choices []struct {
			Delta struct {
				Content          string `json:"content"`
				Role             string `json:"role"`
				ReasoningContent string `json:"reasoning_content"`
			} `json:"delta"`
			FinishReason string `json:"finish_reason"`
		} `json:"choices"`
		Error *struct {
			Message string `json:"message"`
			Type    string `json:"type"`
			Code    any    `json:"code"`
		} `json:"error"`
		Usage *struct {
			PromptTokens     int `json:"prompt_tokens"`
			CompletionTokens int `json:"completion_tokens"`
			TotalTokens      int `json:"total_tokens"`
		} `json:"usage"`
	}

	reqBody := ChatRequestOpenAI{
		Model:          s.model,
		Messages:       openAIMessages,
		Stream:         true,
		MaxTokens:      s.maxTokens,
		ResponseFormat: responseFormat,
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal OpenAI request body: %w", err)
	}

	maxAttempts := 3
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		if err := ctx.Err(); err != nil {
			return "", err
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.baseURL, bytes.NewReader(bodyBytes))
		if err != nil {
			return "", fmt.Errorf("failed to create HTTP request for OpenAI: %w", err)
		}

		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "text/event-stream, application/json")
		req.Header.Set("User-Agent", "curl/8.5.0")
		if s.apiKey != "" {
			req.Header.Set("Authorization", "Bearer "+s.apiKey)
		}

		resp, err := s.client.Do(req)
		if err != nil {
			// Network/transport error: retryable unless we ran out of attempts.
			if attempt == maxAttempts {
				return "", fmt.Errorf("failed to call OpenAI API after %d attempts: %w", maxAttempts, err)
			}
			if err := waitForRetry(ctx, attempt); err != nil {
				return "", err
			}
			continue
		}

		respStatusCode := resp.StatusCode
		if retryableHTTPStatus(respStatusCode) {
			respBytes, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			trimmed := strings.TrimSpace(string(respBytes))
			log.Printf("[AIService] Upstream HTTP %d (retryable) on attempt %d/%d: %s", respStatusCode, attempt, maxAttempts, trimmed)
			if attempt < maxAttempts {
				if err := waitForRetry(ctx, attempt); err != nil {
					return "", err
				}
				continue
			}
		}

		if respStatusCode != http.StatusOK {
			respBytes, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			trimmed := strings.TrimSpace(string(respBytes))
			if respStatusCode == http.StatusGatewayTimeout || respStatusCode == http.StatusBadGateway || respStatusCode == 524 || strings.HasPrefix(trimmed, "<") {
				return "", fmt.Errorf("AI router gateway timeout (HTTP %d from %s): model '%s' took too long or proxy timed out: %s", respStatusCode, s.baseURL, s.model, trimmed)
			}
			var errResp ChatResponseOpenAI
			if err := json.Unmarshal(respBytes, &errResp); err == nil && errResp.Error != nil && errResp.Error.Message != "" {
				return "", fmt.Errorf("OpenAI API error (HTTP %d): %s", respStatusCode, errResp.Error.Message)
			}
			return "", fmt.Errorf("OpenAI API returned status %d: %s", respStatusCode, trimmed)
		}

		// Handle SSE Streaming (keeps connection alive, avoids Cloudflare 524 timeout)
		contentType := resp.Header.Get("Content-Type")
		if strings.Contains(contentType, "text/event-stream") {
			scanner := bufio.NewScanner(resp.Body)
			buf := make([]byte, 1024*1024)
			scanner.Buffer(buf, 10*1024*1024)

			var sb strings.Builder
			var lastFinishReason string
			var completionTokens int
			var streamErr error
			for scanner.Scan() {
				line := strings.TrimSpace(scanner.Text())
				if line == "" || strings.HasPrefix(line, ":") {
					continue
				}
				if line == "data: [DONE]" {
					break
				}
				if strings.HasPrefix(line, "data: ") {
					dataPayload := strings.TrimPrefix(line, "data: ")
					var chunk chatCompletionChunk
					if err := json.Unmarshal([]byte(dataPayload), &chunk); err == nil {
						if chunk.Error != nil && chunk.Error.Message != "" {
							streamErr = fmt.Errorf("upstream stream error: %s", chunk.Error.Message)
							break
						}
						if len(chunk.Choices) > 0 {
							delta := chunk.Choices[0].Delta.Content
							sb.WriteString(delta)
							if onChunk != nil && delta != "" {
								onChunk(delta, completionTokens)
							}
							if chunk.Choices[0].FinishReason != "" {
								lastFinishReason = chunk.Choices[0].FinishReason
							}
						}
						if chunk.Usage != nil {
							completionTokens = chunk.Usage.CompletionTokens
						}
					}
				}
			}
			scanErr := scanner.Err()
			resp.Body.Close()

			rawText := sb.String()
			log.Printf("[AIService] Streamed response completed (attempt %d/%d): length=%d chars, completion_tokens=%d, finish_reason=%q, scanErr=%v, streamErr=%v",
				attempt, maxAttempts, len(rawText), completionTokens, lastFinishReason, scanErr, streamErr)
			if lastFinishReason == "length" || lastFinishReason == "content_filter" || lastFinishReason == "tool_calls" {
				return "", ErrIncompleteCompletion
			}
			isProviderUnavailable := isAggregatorErrorText(rawText)
			if streamErr != nil || scanErr != nil || strings.TrimSpace(rawText) == "" || isProviderUnavailable {
				if attempt < maxAttempts {
					log.Printf("[AIService] Upstream failure/interruption (isProviderUnavailable=%v, len=%d) on attempt %d/%d, retrying...",
						isProviderUnavailable, len(rawText), attempt, maxAttempts)
					if err := waitForRetry(ctx, attempt); err != nil {
						return "", err
					}
					continue
				}
				if isProviderUnavailable {
					return "", fmt.Errorf("AI router upstream error: %s", strings.TrimSpace(rawText))
				}
				if streamErr != nil {
					return "", streamErr
				}
				if scanErr != nil {
					return "", fmt.Errorf("stream connection interrupted: %w", scanErr)
				}
				return "", fmt.Errorf("empty stream content received from OpenAI API after %d attempts", maxAttempts)
			}
			return rawText, nil
		}

		// Fallback for standard non-streaming response
		respBytes, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			if attempt == maxAttempts {
				return "", fmt.Errorf("failed to read response body: %w", err)
			}
			if err := waitForRetry(ctx, attempt); err != nil {
				return "", err
			}
			continue
		}

		var openAIResp ChatResponseOpenAI
		if err := json.Unmarshal(respBytes, &openAIResp); err != nil {
			if attempt < maxAttempts {
				if err := waitForRetry(ctx, attempt); err != nil {
					return "", err
				}
				continue
			}
			return "", fmt.Errorf("failed to unmarshal OpenAI response: %w: %s", err, string(respBytes))
		}
		if len(openAIResp.Choices) == 0 {
			if attempt < maxAttempts {
				if err := waitForRetry(ctx, attempt); err != nil {
					return "", err
				}
				continue
			}
			return "", fmt.Errorf("empty choices in OpenAI response: %s", string(respBytes))
		}

		choice := openAIResp.Choices[0]
		log.Printf("[AIService] Token usage: prompt=%d, completion=%d, total=%d, finish_reason=%q (requested_max=%d)",
			openAIResp.Usage.PromptTokens, openAIResp.Usage.CompletionTokens, openAIResp.Usage.TotalTokens, choice.FinishReason, s.maxTokens)
		if choice.FinishReason == "length" || choice.FinishReason == "content_filter" || choice.FinishReason == "tool_calls" {
			return "", ErrIncompleteCompletion
		}

		rawText := choice.Message.Content
		isProviderUnavailable := isAggregatorErrorText(rawText)
		if strings.TrimSpace(rawText) == "" || isProviderUnavailable {
			if attempt < maxAttempts {
				log.Printf("[AIService] Upstream provider unavailable in non-stream response (isProviderUnavailable=%v) on attempt %d/%d, retrying...",
					isProviderUnavailable, attempt, maxAttempts)
				if err := waitForRetry(ctx, attempt); err != nil {
					return "", err
				}
				continue
			}
			if isProviderUnavailable {
				return "", fmt.Errorf("AI router upstream error: %s", strings.TrimSpace(rawText))
			}
			return "", fmt.Errorf("no content in OpenAI response: %s", string(respBytes))
		}
		return rawText, nil
	}

	return "", fmt.Errorf("failed to call OpenAI API after %d attempts", maxAttempts)
}

func (s *Client) callAnthropic(
	systemPrompt string,
	historyMessages []Message,
	newPrompt string,
) (string, error) {
	return s.callAnthropicWithContext(context.Background(), systemPrompt, historyMessages, newPrompt)
}

func (s *Client) callAnthropicWithContext(ctx context.Context, systemPrompt string, historyMessages []Message, newPrompt string) (string, error) {
	rawMessages := make([]map[string]string, 0, len(historyMessages)+1)
	for _, msg := range historyMessages {
		role := msg.Role
		if role != "user" && role != "assistant" {
			role = "user"
		}
		rawMessages = append(rawMessages, map[string]string{
			"role":    role,
			"content": msg.Content,
		})
	}

	if strings.TrimSpace(newPrompt) != "" {
		rawMessages = append(rawMessages, map[string]string{
			"role":    "user",
			"content": strings.TrimSpace(newPrompt),
		})
	}

	var consolidatedMessages []map[string]string
	for _, m := range rawMessages {
		if len(consolidatedMessages) > 0 && consolidatedMessages[len(consolidatedMessages)-1]["role"] == m["role"] {
			consolidatedMessages[len(consolidatedMessages)-1]["content"] += "\n" + m["content"]
		} else {
			consolidatedMessages = append(consolidatedMessages, map[string]string{
				"role":    m["role"],
				"content": m["content"],
			})
		}
	}

	if len(consolidatedMessages) == 0 {
		consolidatedMessages = append(consolidatedMessages, map[string]string{
			"role":    "user",
			"content": "Generate diagram",
		})
	} else if consolidatedMessages[0]["role"] != "user" {
		consolidatedMessages = append([]map[string]string{{"role": "user", "content": "Start conversation"}}, consolidatedMessages...)
	}

	reqBody := anthropicRequest{
		Model:     s.model,
		MaxTokens: s.maxTokens,
		System:    systemPrompt,
		Messages:  consolidatedMessages,
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal Anthropic request body: %w", err)
	}

	var respBytes []byte
	var respStatusCode int
	maxAttempts := 3

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		if err := ctx.Err(); err != nil {
			return "", err
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.baseURL, bytes.NewReader(bodyBytes))
		if err != nil {
			return "", fmt.Errorf("failed to create HTTP request: %w", err)
		}

		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("x-api-key", s.apiKey)
		req.Header.Set("anthropic-version", "2023-06-01")

		resp, err := s.client.Do(req)
		if err != nil {
			if attempt == maxAttempts {
				return "", fmt.Errorf("failed to call Anthropic API after %d attempts: %w", maxAttempts, err)
			}
			if err := waitForRetry(ctx, attempt); err != nil {
				return "", err
			}
			continue
		}

		respBytes, err = io.ReadAll(resp.Body)
		resp.Body.Close()

		if err != nil {
			if attempt == maxAttempts {
				return "", fmt.Errorf("failed to read response body: %w", err)
			}
			if err := waitForRetry(ctx, attempt); err != nil {
				return "", err
			}
			continue
		}

		respStatusCode = resp.StatusCode
		if retryableHTTPStatus(respStatusCode) || (respStatusCode == http.StatusOK && strings.Contains(string(respBytes), "No available accounts")) {
			if attempt < maxAttempts {
				if err := waitForRetry(ctx, attempt); err != nil {
					return "", err
				}
				continue
			}
		}

		break
	}

	if respStatusCode != http.StatusOK {
		return "", fmt.Errorf("Anthropic API returned status %d: %s", respStatusCode, string(respBytes))
	}

	var anthropicResp anthropicResponse
	if err := json.Unmarshal(respBytes, &anthropicResp); err != nil {
		return "", fmt.Errorf("failed to unmarshal Anthropic response: %w", err)
	}

	if anthropicResp.Error != nil {
		return "", fmt.Errorf("Anthropic API error: %s - %s", anthropicResp.Error.Type, anthropicResp.Error.Message)
	}
	if anthropicResp.StopReason == "max_tokens" || anthropicResp.StopReason == "refusal" || anthropicResp.StopReason == "tool_use" {
		return "", ErrIncompleteCompletion
	}

	if len(anthropicResp.Content) == 0 {
		return "", fmt.Errorf("empty response from Anthropic API: %s", string(respBytes))
	}

	var rawText string
	for _, c := range anthropicResp.Content {
		if c.Type == "text" {
			rawText += c.Text
		}
	}

	if rawText == "" {
		for _, c := range anthropicResp.Content {
			if c.Text != "" {
				rawText += c.Text
			}
		}
	}

	if rawText == "" {
		return "", fmt.Errorf("no text content found in Anthropic response: %s", string(respBytes))
	}

	return rawText, nil
}

func (s *Client) CallLLM(systemPrompt string, historyMessages []Message, newPrompt string) (string, error) {
	return s.CallLLMContext(context.Background(), systemPrompt, historyMessages, newPrompt)
}

func (s *Client) CallLLMContext(ctx context.Context, systemPrompt string, historyMessages []Message, newPrompt string) (string, error) {
	if s.provider == "openai" {
		return s.callOpenAIWithContext(ctx, systemPrompt, historyMessages, newPrompt, &ResponseFormatOpenAI{Type: "json_object"}, nil)
	}
	return s.callAnthropicWithContext(ctx, systemPrompt, historyMessages, newPrompt)
}

func (s *Client) CallLLMStream(
	systemPrompt string,
	historyMessages []Message,
	newPrompt string,
	onChunk LLMStreamCallback,
) (string, error) {
	return s.CallLLMStreamContext(context.Background(), systemPrompt, historyMessages, newPrompt, onChunk)
}

func (s *Client) CallLLMStreamContext(
	ctx context.Context,
	systemPrompt string,
	historyMessages []Message,
	newPrompt string,
	onChunk LLMStreamCallback,
) (string, error) {
	if s.provider == "openai" {
		return s.callOpenAIWithContext(ctx, systemPrompt, historyMessages, newPrompt, &ResponseFormatOpenAI{Type: "json_object"}, onChunk)
	}
	return s.callAnthropicWithContext(ctx, systemPrompt, historyMessages, newPrompt)
}

func (s *Client) CallLLMText(systemPrompt string, historyMessages []Message, newPrompt string) (string, error) {
	return s.CallLLMTextContext(context.Background(), systemPrompt, historyMessages, newPrompt)
}

func (s *Client) CallLLMTextContext(ctx context.Context, systemPrompt string, historyMessages []Message, newPrompt string) (string, error) {
	if s.provider == "openai" {
		return s.callOpenAIWithContext(ctx, systemPrompt, historyMessages, newPrompt, nil, nil)
	}
	return s.callAnthropicWithContext(ctx, systemPrompt, historyMessages, newPrompt)
}

func (s *Client) CallLLMTextStream(
	systemPrompt string,
	historyMessages []Message,
	newPrompt string,
	onChunk LLMStreamCallback,
) (string, error) {
	return s.CallLLMTextStreamContext(context.Background(), systemPrompt, historyMessages, newPrompt, onChunk)
}

func (s *Client) CallLLMTextStreamContext(
	ctx context.Context,
	systemPrompt string,
	historyMessages []Message,
	newPrompt string,
	onChunk LLMStreamCallback,
) (string, error) {
	if s.provider == "openai" {
		return s.callOpenAIWithContext(ctx, systemPrompt, historyMessages, newPrompt, nil, onChunk)
	}
	return s.callAnthropicWithContext(ctx, systemPrompt, historyMessages, newPrompt)
}

func (s *Client) SanitizeJSON(raw string) string {
	return sanitizeJSONResponse(raw)
}
