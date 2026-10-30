package llm

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestCompletionContextCancelsInFlightRequests(test *testing.T) {
	for _, provider := range []string{"openai", "anthropic"} {
		test.Run(provider, func(test *testing.T) {
			started := make(chan struct{})
			var calls atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
				calls.Add(1)
				_, _ = io.Copy(io.Discard, request.Body)
				close(started)
				select {
				case <-request.Context().Done():
				case <-time.After(3 * time.Second):
				}
			}))
			defer server.Close()
			client := NewClient(Config{AIProvider: provider, OpenAIMBaseURL: server.URL, AnthropicBaseURL: server.URL}, server.Client())
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			finished := make(chan error, 1)
			go func() {
				_, err := client.CallLLMContext(ctx, "JSON only", nil, "Generate")
				finished <- err
			}()
			select {
			case <-started:
			case <-time.After(3 * time.Second):
				test.Fatal("request did not start")
			}
			cancel()
			select {
			case err := <-finished:
				if !errors.Is(err, context.Canceled) {
					test.Fatalf("expected cancellation, got %v", err)
				}
			case <-time.After(time.Second):
				test.Fatal("cancellation did not interrupt request")
			}
			if calls.Load() != 1 {
				test.Fatalf("retried canceled request: %d calls", calls.Load())
			}
		})
	}
}

func TestRetryWaitHonorsCancellation(test *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	started := time.Now()
	if err := waitForRetry(ctx, 4); !errors.Is(err, context.DeadlineExceeded) {
		test.Fatalf("got %v", err)
	}
	if time.Since(started) > time.Second {
		test.Fatal("retry sleep ignored deadline")
	}
}

func TestCompletionRejectsIncompleteProviderResults(test *testing.T) {
	for _, scenario := range []struct{ name, provider, body, contentType string }{
		{"OpenAI length", "openai", `{"choices":[{"message":{"content":"{}"},"finish_reason":"length"}]}`, "application/json"},
		{"OpenAI filtered", "openai", `{"choices":[{"message":{"content":"{}"},"finish_reason":"content_filter"}]}`, "application/json"},
		{"SSE length", "openai", "data: {\"choices\":[{\"delta\":{\"content\":\"{}\"},\"finish_reason\":\"length\"}]}\n\ndata: [DONE]\n\n", "text/event-stream"},
		{"Anthropic length", "anthropic", `{"content":[{"type":"text","text":"{}"}],"stop_reason":"max_tokens"}`, "application/json"},
	} {
		test.Run(scenario.name, func(test *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
				writer.Header().Set("Content-Type", scenario.contentType)
				fmt.Fprint(writer, scenario.body)
			}))
			defer server.Close()
			client := NewClient(Config{AIProvider: scenario.provider, OpenAIMBaseURL: server.URL, AnthropicBaseURL: server.URL}, server.Client())
			result, err := client.CallLLMContext(context.Background(), "JSON only", nil, "Generate")
			if !errors.Is(err, ErrIncompleteCompletion) || result != "" {
				test.Fatalf("incomplete result accepted: %q %v", result, err)
			}
		})
	}
}
