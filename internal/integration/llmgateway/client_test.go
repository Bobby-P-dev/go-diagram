package llmgateway

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/Bobby-P-dev/go-diagram.git/internal/modules/generation"
	generationservice "github.com/Bobby-P-dev/go-diagram.git/internal/modules/generation/service"
	"github.com/Bobby-P-dev/go-diagram.git/internal/platform/llm"
)

func TestMetaMuseModelAPIDocumentGeneration(test *testing.T) {
	expectedDocument := `{
		"title": "Decentralized Settlement Engine PRD",
		"document_content": "# Decentralized Settlement Engine\n\n## 1. Overview\nHigh-reliability transaction settlement.\n\n## 2. Requirements\nBatch settlement processing under 500ms latency.",
		"change_summary": "Authored comprehensive settlement specifications",
		"assistant_message": "Document generated successfully for Meta Muse API."
	}`

	var requestCount atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		requestCount.Add(1)
		if request.Method != http.MethodPost || request.URL.Path != "/v1/chat/completions" {
			test.Errorf("unexpected path: %s %s", request.Method, request.URL.Path)
		}
		if request.Header.Get("Authorization") != "Bearer mock-meta-key" {
			test.Errorf("missing or invalid authorization header")
		}

		var body llm.ChatRequestOpenAI
		if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
			test.Errorf("decode request error: %v", err)
			http.Error(writer, "invalid request", http.StatusBadRequest)
			return
		}

		if body.Model != "muse-spark-1.3" || !body.Stream {
			test.Errorf("expected muse-spark-1.3 streaming request, got: %+v", body)
		}

		writer.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(writer, ": heartbeat\n\n")
		parts := []string{expectedDocument[:len(expectedDocument)/2], expectedDocument[len(expectedDocument)/2:]}
		for _, part := range parts {
			encoded, _ := json.Marshal(part)
			fmt.Fprintf(writer, "data: {\"choices\":[{\"delta\":{\"content\":%s}}]}\n\n", encoded)
		}
		fmt.Fprint(writer, "data: {\"choices\":[{\"delta\":{},\"finish_reason\":\"stop\"}]}\n\ndata: [DONE]\n\n")
	}))
	defer server.Close()

	llmClient := llm.NewClient(llm.Config{
		AIProvider:     "openai",
		OpenAIAPIKey:   "mock-meta-key",
		OpenAIModel:    "muse-spark-1.3",
		OpenAIMBaseURL: server.URL + "/v1",
		AIMaxTokens:    8192,
	}, server.Client())

	gateway := New(llmClient)
	engine := generationservice.NewDocumentEngine(gateway)

	manifest := &generation.Manifest{
		SchemaVersion: 1,
		Mode:          "explicit",
		Project: generation.ProjectContext{
			ID:          "11111111-1111-4111-8111-111111111111",
			Title:       "Settlement Core",
			Description: "Core multi-tenant settlement platform",
		},
		ThreadID: "22222222-2222-4222-8222-222222222222",
	}

	var chunks strings.Builder
	candidate, err := engine.GenerateDocument(context.Background(), generation.DocumentInput{
		Type:     "prd",
		Prompt:   "Draft settlement core PRD",
		Manifest: manifest,
	}, func(delta string, tokenCount int) {
		chunks.WriteString(delta)
	})

	if err != nil {
		test.Fatalf("unexpected error from DocumentEngine: %v", err)
	}
	if requestCount.Load() != 1 {
		test.Fatalf("expected 1 request to Meta Model API, got %d", requestCount.Load())
	}
	if candidate.Title != "Decentralized Settlement Engine PRD" {
		test.Fatalf("unexpected title: %s", candidate.Title)
	}
	if candidate.Type != "prd" || candidate.Format != "markdown" || candidate.SchemaVersion != 1 {
		test.Fatalf("unexpected candidate envelope: %+v", candidate)
	}
	if chunks.String() != expectedDocument {
		test.Fatalf("stream callback mismatch")
	}
}
