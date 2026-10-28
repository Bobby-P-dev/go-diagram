package llm

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

func TestMuseCompatibleCompletion(t *testing.T) {
	expected := `{"entities":[{"name":"User"},{"name":"Order"}],"relations":[]}`
	tests := []struct {
		name       string
		streaming  bool
		output     string
		statusCode int
		textMode   bool
	}{
		{name: "SSE", streaming: true, output: expected},
		{name: "standard JSON", output: expected},
		{name: "text mode for UI generation", streaming: true, output: expected, textMode: true},
		{name: "markdown cleanup", streaming: true, output: "```json\n" + expected + "\n```"},
		{name: "authentication failure", statusCode: http.StatusUnauthorized},
		{name: "invalid request", statusCode: http.StatusBadRequest},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var requestCount atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
				requestCount.Add(1)
				if request.Method != http.MethodPost || request.URL.Path != "/v1/chat/completions" {
					t.Errorf("unexpected request: %s %s", request.Method, request.URL.Path)
				}
				if request.Header.Get("Authorization") != "Bearer mock-meta-key" {
					t.Error("missing configured bearer credential")
				}
				var body ChatRequestOpenAI
				if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
					t.Errorf("decode request: %v", err)
					http.Error(writer, "invalid request", http.StatusBadRequest)
					return
				}
				if body.Model != "muse-spark-1.3" || !body.Stream || body.MaxTokens != 12000 {
					t.Errorf("incorrect model, streaming, or token budget: %+v", body)
				}
				if test.textMode && body.ResponseFormat != nil {
					t.Error("text mode must not force JSON mode")
				} else if !test.textMode && (body.ResponseFormat == nil || body.ResponseFormat.Type != "json_object") {
					t.Error("expected JSON object mode")
				}
				if len(body.Messages) != 4 || body.Messages[0].Role != "system" || body.Messages[3].Content != "Return the ERD as JSON" {
					t.Errorf("incorrect prompt/history mapping: %+v", body.Messages)
				}
				if test.statusCode != 0 {
					writer.Header().Set("Content-Type", "application/json")
					writer.WriteHeader(test.statusCode)
					fmt.Fprint(writer, `{"error":{"message":"mock rejection","type":"invalid_request_error","code":"mock"}}`)
					return
				}
				if test.streaming {
					writer.Header().Set("Content-Type", "text/event-stream")
					fmt.Fprint(writer, ": heartbeat\n\n")
					for _, part := range []string{test.output[:len(test.output)/2], test.output[len(test.output)/2:]} {
						encoded, _ := json.Marshal(part)
						fmt.Fprintf(writer, "data: {\"choices\":[{\"delta\":{\"content\":%s}}]}\n\n", encoded)
					}
					fmt.Fprint(writer, "data: {\"choices\":[{\"delta\":{},\"finish_reason\":\"stop\"}],\"usage\":{\"completion_tokens\":40}}\n\ndata: [DONE]\n\n")
					return
				}
				writer.Header().Set("Content-Type", "application/json")
				encoded, _ := json.Marshal(test.output)
				fmt.Fprintf(writer, `{"choices":[{"message":{"role":"assistant","content":%s},"finish_reason":"stop"}]}`, encoded)
			}))
			defer server.Close()
			client := NewClient(Config{
				AIProvider: "openai", OpenAIAPIKey: "mock-meta-key", OpenAIModel: "muse-spark-1.3",
				OpenAIMBaseURL: server.URL + "/v1", AIMaxTokens: 12000,
			}, server.Client())
			var chunks strings.Builder
			complete := client.CallLLMStream
			if test.textMode {
				complete = client.CallLLMTextStream
			}
			output, err := complete("Return JSON only", []Message{
				{Role: "user", Content: "Design a store"},
				{Role: "assistant", Content: "{}"},
			}, "Return the ERD as JSON", func(chunk string, tokens int) { chunks.WriteString(chunk) })
			if requestCount.Load() != 1 {
				t.Fatalf("expected one request, got %d", requestCount.Load())
			}
			if test.statusCode != 0 {
				if err == nil || !strings.Contains(err.Error(), "mock rejection") {
					t.Fatalf("expected provider rejection, got %v", err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if test.streaming && chunks.String() != test.output {
				t.Fatal("stream callback lost content")
			}
			if cleaned := client.SanitizeJSON(output); cleaned != expected || !json.Valid([]byte(cleaned)) {
				t.Fatalf("unexpected JSON: %s", cleaned)
			}
		})
	}
}
