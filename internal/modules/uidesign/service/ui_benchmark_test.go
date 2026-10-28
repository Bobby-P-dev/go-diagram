package service_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/Bobby-P-dev/go-diagram.git/internal/modules/uidesign"
	dtos "github.com/Bobby-P-dev/go-diagram.git/internal/modules/uidesign/dto"
	uidesignservice "github.com/Bobby-P-dev/go-diagram.git/internal/modules/uidesign/service"
)

type benchmarkScenario struct {
	name          string
	prompt        string
	device        string
	width, height int
	payload       func(testing.TB) string
}

func benchmarkScenarios() []benchmarkScenario {
	return []benchmarkScenario{
		{"bakery_mobile", "Katalog Toko Roti & Kue Tradisional Oma", "mobile", 375, 812, bakeryBenchmarkPayload},
		{"telemetry_web", "SaaS Cloud Telemetry Dashboard with latency metrics and pod health", "web", 1024, 720, telemetryBenchmarkPayload},
		{"minimal_login", "buat halaman login minimalis", "web", 1024, 720, authBenchmarkPayload},
		{"editorial_desktop", "Atelier Studio Arsitektur & Desain Ruang", "desktop", 1100, 740, architectureBenchmarkPayload},
	}
}

type benchmarkLLM struct {
	test    testing.TB
	outputs []string
	methods []string
	prompts []string
	failure error
}

func (client *benchmarkLLM) next(method, prompt string) (string, error) {
	client.methods = append(client.methods, method)
	client.prompts = append(client.prompts, prompt)
	if client.failure != nil {
		return "", client.failure
	}
	index := len(client.methods) - 1
	if index >= len(client.outputs) {
		client.test.Fatalf("unexpected LLM call %d", index+1)
	}
	return client.outputs[index], nil
}

func (client *benchmarkLLM) CallLLM(systemPrompt, prompt string) (string, error) {
	return client.next("json", prompt)
}

func (client *benchmarkLLM) CallLLMStream(systemPrompt, prompt string, callback uidesignservice.LLMStreamCallback) (string, error) {
	output, err := client.next("stream", prompt)
	if err == nil {
		for start := 0; start < len(output); start += 128 {
			callback(output[start:min(start+128, len(output))], (start/128+1)*64)
		}
	}
	return output, err
}

func (client *benchmarkLLM) CallLLMText(systemPrompt, prompt string) (string, error) {
	client.test.Fatal("single-pass pipeline unexpectedly used a text-only stage")
	return "", nil
}

func (client *benchmarkLLM) CallLLMTextStream(systemPrompt, prompt string, callback uidesignservice.LLMStreamCallback) (string, error) {
	return client.CallLLMText(systemPrompt, prompt)
}

func (client *benchmarkLLM) SanitizeJSON(raw string) string { return raw }

type benchmarkGateway struct {
	*inMemoryProjectGateway
	snapshots []uidesign.CreateVersionInput
}

func newBenchmarkGateway() *benchmarkGateway {
	return &benchmarkGateway{inMemoryProjectGateway: newInMemoryProjectGateway()}
}

func (gateway *benchmarkGateway) CreateVersion(ctx context.Context, input uidesign.CreateVersionInput) error {
	if err := gateway.inMemoryProjectGateway.CreateVersion(ctx, input); err != nil {
		return err
	}
	gateway.snapshots = append(gateway.snapshots, input)
	return nil
}

func benchmarkRequest(scenario benchmarkScenario, fixture dtos.UIDesignSynthesisDTO) dtos.CreateUIDesignRequest {
	return dtos.CreateUIDesignRequest{Prompt: scenario.prompt, Device: scenario.device, Foundation: "auto", ThemeMode: fixture.Theme["mode"].(string), AccentColor: fixture.Theme["accent"].(string)}
}

func assertBenchmarkResult(t testing.TB, response *dtos.ProjectResponse, gateway *benchmarkGateway, scenario benchmarkScenario, fixture dtos.UIDesignSynthesisDTO) {
	t.Helper()
	if response == nil {
		t.Fatal("missing project response")
	}
	var nodes []struct {
		ID   string           `json:"id"`
		Type string           `json:"type"`
		Data dtos.UIFrameData `json:"data"`
	}
	if err := json.Unmarshal(response.Nodes, &nodes); err != nil {
		t.Fatal(err)
	}
	if len(nodes) != 1 || nodes[0].Type != "ui_frame" || nodes[0].ID == "" {
		t.Fatalf("expected one identified Vue Flow frame: %+v", nodes)
	}
	frame := nodes[0].Data
	if frame.RawHtml != fixture.RawHTML || frame.CodeExport["html"] != frame.RawHtml || frame.Implementation == nil || frame.Implementation.Source.HTML != frame.RawHtml || frame.Implementation.Source.Vue != frame.CodeExport["vue"] {
		t.Fatal("fixture HTML changed or preview/export/canonical source diverged")
	}
	if frame.Canvas == nil || frame.Canvas.Device != scenario.device || frame.Canvas.Width != scenario.width || frame.Canvas.Height != scenario.height || frame.Width != scenario.width || frame.Height != scenario.height {
		t.Fatal("device viewport contract changed")
	}
	if frame.Title != fixture.Title || len(frame.Sections) != len(fixture.Sections) {
		t.Fatal("fixture title or section count changed")
	}
	for key, value := range fixture.Theme {
		if frame.Theme[key] != value {
			t.Errorf("theme token %s changed: got %v want %v", key, frame.Theme[key], value)
		}
	}
	for index, section := range frame.Sections {
		if section.ID != fixture.Sections[index].ID || section.Type != fixture.Sections[index].Type || section.RequirementSource == "" || !strings.Contains(frame.RawHtml, `data-rl-id="`+section.ID+`"`) {
			t.Errorf("section targeting/traceability changed: %+v", section)
		}
	}
	if frame.DesignState == nil || frame.DesignState.DesignSpec == nil || len(frame.DesignState.DesignSpec.Sections) != len(frame.Sections) {
		t.Fatal("canonical design sections diverged from frame sections")
	}
	if frame.Audit == nil || frame.Audit.Validation == nil || frame.Audit.Validation.Status == "fail" || frame.Audit.VisualReview == nil || frame.Audit.VisualReview.Status != "not_evaluated" {
		t.Fatal("missing validation or unperformed visual review marked evaluated")
	}
	stored := gateway.projects[response.ID]
	if len(gateway.projects) != 1 || stored == nil || stored.UserID != "benchmark-user" || stored.ProjectMode != "ui_design" || !bytes.Equal(stored.CurrentNodes, response.Nodes) || !bytes.Equal(response.CurrentNodes, response.Nodes) {
		t.Fatal("stored project graph differs from response")
	}
	if len(gateway.snapshots) != 1 || gateway.snapshots[0].VersionNumber != 1 || gateway.snapshots[0].ProjectID != response.ID || !bytes.Equal(gateway.snapshots[0].Nodes, response.Nodes) || !bytes.Equal(gateway.snapshots[0].Edges, response.Edges) {
		t.Fatal("version 1 does not snapshot the returned graph")
	}
	messages := gateway.messages[response.ID]
	if len(messages) != 2 || messages[0].Role != "user" || messages[0].Content != scenario.prompt || messages[1].Role != "assistant" || len(response.Messages) != 2 {
		t.Fatal("user and assistant conversation was not persisted")
	}
}

func TestBenchmarkUIDesignScenarios(t *testing.T) {
	for _, scenario := range benchmarkScenarios() {
		for _, streaming := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/stream=%v", scenario.name, streaming), func(t *testing.T) {
				payload := scenario.payload(t)
				var fixture dtos.UIDesignSynthesisDTO
				if err := json.Unmarshal([]byte(payload), &fixture); err != nil {
					t.Fatal(err)
				}
				client := &benchmarkLLM{test: t, outputs: []string{payload}}
				gateway := newBenchmarkGateway()
				service := uidesignservice.NewUIDesignService(gateway, client, func(context.Context) string { return "benchmark-user" })
				request := benchmarkRequest(scenario, fixture)
				var response *dtos.ProjectResponse
				var err error
				if streaming {
					var events, stages []string
					progressCount, previousTokens, previousChars := 0, 0, 0
					var completed *dtos.ProjectResponse
					response, err = service.GenerateUIDesignStream(context.Background(), request, func(event string, data any) {
						events = append(events, event)
						switch event {
						case "status":
							stages = append(stages, data.(map[string]string)["stage"])
						case "progress":
							progress := data.(map[string]interface{})
							tokens, chars := progress["tokens"].(int), progress["chars"].(int)
							if tokens <= previousTokens || chars <= previousChars {
								t.Error("non-increasing streaming progress")
							}
							previousTokens, previousChars = tokens, chars
							progressCount++
						case "complete":
							if completed != nil {
								t.Error("duplicate complete event")
							}
							completed = data.(*dtos.ProjectResponse)
						}
					})
					if progressCount == 0 || completed != response || len(events) == 0 || events[len(events)-1] != "complete" || !reflect.DeepEqual(stages, []string{"analyzing", "compiling"}) || !reflect.DeepEqual(client.methods, []string{"stream"}) {
						t.Fatalf("invalid streaming lifecycle: %v, stages=%v, progress=%d", events, stages, progressCount)
					}
				} else {
					response, err = service.GenerateUIDesign(context.Background(), request)
					if !reflect.DeepEqual(client.methods, []string{"json"}) {
						t.Fatalf("valid generation must use exactly one call: %v", client.methods)
					}
				}
				if err != nil {
					t.Fatal(err)
				}
				if !strings.Contains(client.prompts[0], scenario.prompt) {
					t.Fatal("user prompt lost before LLM call")
				}
				assertBenchmarkResult(t, response, gateway, scenario, fixture)
			})
		}
	}
}

func TestBenchmarkUIDesignRepairAndFailure(t *testing.T) {
	scenario := benchmarkScenarios()[2]
	payload := scenario.payload(t)
	var fixture dtos.UIDesignSynthesisDTO
	if err := json.Unmarshal([]byte(payload), &fixture); err != nil {
		t.Fatal(err)
	}
	for _, failure := range []string{"malformed_json", "missing_html", "provider_error", "canceled", "repair_success"} {
		t.Run(failure, func(t *testing.T) {
			invalid := `{"raw_html":"","sections":[]}`
			if failure == "malformed_json" {
				invalid = `{"raw_html":"unfinished`
			}
			client := &benchmarkLLM{test: t, outputs: []string{invalid, invalid}}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			if failure == "provider_error" {
				client.failure = errors.New("mock provider unavailable")
			}
			if failure == "canceled" {
				cancel()
			}
			if failure == "repair_success" {
				client.outputs[1] = payload
			}
			gateway := newBenchmarkGateway()
			service := uidesignservice.NewUIDesignService(gateway, client, func(context.Context) string { return "benchmark-user" })
			completeCount := 0
			response, err := service.GenerateUIDesignStream(ctx, benchmarkRequest(scenario, fixture), func(event string, data any) {
				if event == "complete" {
					completeCount++
				}
			})
			expectedCalls := 2
			if failure == "canceled" {
				expectedCalls = 0
			}
			if failure == "provider_error" {
				expectedCalls = 1
			}
			if len(client.methods) != expectedCalls {
				t.Fatalf("unexpected repair/cancellation calls: %v", client.methods)
			}
			if failure == "repair_success" {
				if err != nil || completeCount != 1 {
					t.Fatalf("repair did not finish once: %v", err)
				}
				assertBenchmarkResult(t, response, gateway, scenario, fixture)
				return
			}
			if err == nil || response != nil || completeCount != 0 || len(gateway.projects) != 0 || len(gateway.snapshots) != 0 || len(gateway.messages) != 0 {
				t.Fatal("failed generation produced successful or persisted artifacts")
			}
			if failure == "provider_error" && !errors.Is(err, client.failure) || failure == "canceled" && !errors.Is(err, context.Canceled) {
				t.Fatalf("original error lost: %v", err)
			}
		})
	}
}

func BenchmarkUIDesignOfflinePipeline(benchmark *testing.B) {
	for _, scenario := range benchmarkScenarios() {
		benchmark.Run(scenario.name, func(benchmark *testing.B) {
			payload := scenario.payload(benchmark)
			var fixture dtos.UIDesignSynthesisDTO
			if err := json.Unmarshal([]byte(payload), &fixture); err != nil {
				benchmark.Fatal(err)
			}
			request := benchmarkRequest(scenario, fixture)
			benchmark.ReportAllocs()
			benchmark.ResetTimer()
			for iteration := 0; iteration < benchmark.N; iteration++ {
				client := &benchmarkLLM{test: benchmark, outputs: []string{payload}}
				service := uidesignservice.NewUIDesignService(newBenchmarkGateway(), client, func(context.Context) string { return "benchmark-user" })
				if _, err := service.GenerateUIDesign(context.Background(), request); err != nil {
					benchmark.Fatal(err)
				}
			}
		})
	}
}
