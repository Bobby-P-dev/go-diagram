package service

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/Bobby-P-dev/go-diagram.git/internal/modules/generation"
)

func diagramFixture(diagramType string) diagramPayload {
	nodeType := "process"
	if diagramType == "erd" {
		nodeType = "entity"
	} else if diagramType == "sequence" {
		nodeType = "participant"
	}
	nodes := []generation.DiagramNode{
		{ID: "customer", Type: nodeType, Data: generation.DiagramNodeData{Label: "Pelanggan"}},
		{ID: "order", Type: nodeType, Data: generation.DiagramNodeData{Label: "Pesanan"}},
	}
	edge := generation.DiagramEdge{ID: "link", Source: "customer", Target: "order", Label: "Membuat pesanan"}
	if diagramType == "erd" {
		for index := range nodes {
			nodes[index].Data.Fields = []generation.DiagramField{{Name: "id", DataType: "uuid", PrimaryKey: true, Nullable: false}}
		}
		edge.Cardinality = "1:N"
	} else if diagramType == "sequence" {
		edge.SequenceNumber = 1
		edge.MessageType = "call"
	}
	return diagramPayload{
		Title: "Rancangan Pesanan", GraphNodes: nodes, GraphEdges: []generation.DiagramEdge{edge},
		Metadata:      generation.DiagramMetadata{DiagramType: diagramType},
		ChangeSummary: "Menyusun rancangan pesanan.", AssistantMessage: "Rancangan awal siap ditinjau.",
	}
}

func diagramJSON(test *testing.T, payload any) string {
	test.Helper()
	encoded, err := json.Marshal(payload)
	if err != nil {
		test.Fatal(err)
	}
	return string(encoded)
}

func diagramInputFixture(diagramType string) generation.DiagramInput {
	manifest := validManifestFixture()
	manifest.Sources[0].Type = "prd"
	manifest.Sources[0].Title = "PRD Pesanan"
	manifest.Sources[0].Content = json.RawMessage(`{"document_content":"Pelanggan dapat membuat pesanan."}`)
	return generation.DiagramInput{Type: diagramType, Prompt: "Rancang proses pesanan dari PRD.", Title: "Rancangan Pesanan", Manifest: manifest}
}

func TestDiagramEngineGroundedCandidates(test *testing.T) {
	for _, diagramType := range []string{"erd", "flowchart", "sequence"} {
		test.Run(diagramType, func(test *testing.T) {
			input := diagramInputFixture(diagramType)
			mock := &mockLLMClient{responses: []string{diagramJSON(test, diagramFixture(diagramType))}}
			chunks := 0
			candidate, err := NewDiagramEngine(mock).GenerateDiagram(context.Background(), input, func(string, int) { chunks++ })
			if err != nil {
				test.Fatal(err)
			}
			if candidate.Format != "graph" || candidate.SchemaVersion != 1 || candidate.Type != diagramType || candidate.Metadata.DiagramType != diagramType || len(candidate.GraphNodes) != 2 || chunks != 2 {
				test.Fatalf("unexpected candidate: %+v, chunks=%d", candidate, chunks)
			}
			if !reflect.DeepEqual(candidate.Sources, []generation.RevisionRef{{ArtifactID: input.Manifest.Sources[0].ArtifactID, RevisionID: input.Manifest.Sources[0].RevisionID}}) {
				test.Fatalf("lineage not pinned to manifest: %+v", candidate.Sources)
			}
			var received generation.DiagramInput
			if err := json.Unmarshal([]byte(mock.prompts[0]), &received); err != nil || !reflect.DeepEqual(received, input) {
				test.Fatalf("input/context was altered: %v", err)
			}
			if len(mock.histories[0]) != 0 || !strings.Contains(mock.systems[0], "untrusted reference data") || !strings.Contains(mock.systems[0], "Output JSON Schema:") {
				test.Fatal("missing grounded-context contract or unexpected implicit history")
			}
		})
	}
}

func TestDiagramEngineRevisionAndNoMandatoryPRD(test *testing.T) {
	payload := diagramFixture("flowchart")
	input := diagramInputFixture("flowchart")
	input.Manifest.Sources = nil
	input.Title = "  Rancangan Pesanan  "
	input.BaseGraph = &generation.DiagramGraph{Nodes: payload.GraphNodes, Edges: payload.GraphEdges, Metadata: payload.Metadata}
	mock := &mockLLMClient{responses: []string{diagramJSON(test, payload)}}
	candidate, err := NewDiagramEngine(mock).GenerateDiagram(context.Background(), input, nil)
	if err != nil || len(candidate.Sources) != 0 || !strings.Contains(mock.prompts[0], "base_graph") || !strings.Contains(mock.systems[0], "Preserve unrelated facts, node IDs and edge IDs") {
		test.Fatalf("revision without PRD failed: candidate=%+v error=%v", candidate, err)
	}
}

func TestDiagramEngineRepairAndFailure(test *testing.T) {
	good := diagramJSON(test, diagramFixture("flowchart"))
	bad := strings.Replace(good, `"target":"order"`, `"target":"missing"`, 1)
	input := diagramInputFixture("flowchart")
	mock := &mockLLMClient{responses: []string{bad, good}}
	candidate, err := NewDiagramEngine(mock).GenerateDiagram(context.Background(), input, nil)
	if err != nil || candidate == nil || mock.callIndex != 2 {
		test.Fatalf("repair failed: %v", err)
	}
	var repair struct {
		OriginalInput   generation.DiagramInput `json:"original_input"`
		InvalidResponse string                  `json:"invalid_response"`
		ValidationError string                  `json:"validation_error"`
	}
	if err := json.Unmarshal([]byte(mock.prompts[1]), &repair); err != nil || !reflect.DeepEqual(repair.OriginalInput, input) || repair.InvalidResponse != bad || !strings.Contains(repair.ValidationError, "endpoints") {
		test.Fatalf("repair lost input/evidence/validation feedback: %v", err)
	}
	if mock.systems[0] != mock.systems[1] {
		test.Fatal("repair changed system instructions")
	}
	mock = &mockLLMClient{responses: []string{bad, bad, bad}}
	candidate, err = NewDiagramEngine(mock).GenerateDiagram(context.Background(), input, nil)
	if candidate != nil || !errors.Is(err, generation.ErrDiagramRepairFailed) || mock.callIndex != MaxDiagramAttempts {
		test.Fatalf("repair should stop without fallback: candidate=%+v error=%v calls=%d", candidate, err, mock.callIndex)
	}
	wrongTitle := strings.Replace(good, "Rancangan Pesanan", "Different title", 1)
	mock = &mockLLMClient{responses: []string{wrongTitle, good}}
	if _, err := NewDiagramEngine(mock).GenerateDiagram(context.Background(), input, nil); err != nil || mock.callIndex != 2 {
		test.Fatalf("requested title was not enforced: %v", err)
	}
}

func TestDiagramEngineInputValidation(test *testing.T) {
	cases := []struct {
		name   string
		change func(*generation.DiagramInput)
		want   error
	}{
		{"unsupported", func(input *generation.DiagramInput) { input.Type = "sequence_diagram" }, generation.ErrUnsupportedDiagramType},
		{"empty_prompt", func(input *generation.DiagramInput) { input.Prompt = " \n" }, generation.ErrEmptyPrompt},
		{"long_prompt", func(input *generation.DiagramInput) { input.Prompt = strings.Repeat("a", 16*1024+1) }, generation.ErrInvalidInput},
		{"bad_utf8", func(input *generation.DiagramInput) { input.Prompt = string([]byte{255}) }, generation.ErrInvalidInput},
		{"long_title", func(input *generation.DiagramInput) { input.Title = strings.Repeat("a", 256) }, generation.ErrInvalidInput},
		{"missing_manifest", func(input *generation.DiagramInput) { input.Manifest = nil }, generation.ErrInvalidInput},
		{"invalid_project", func(input *generation.DiagramInput) { input.Manifest.Project.ID = "foreign" }, generation.ErrInvalidInput},
		{"invalid_revision", func(input *generation.DiagramInput) { input.Manifest.Sources[0].RevisionID = "" }, generation.ErrInvalidInput},
		{"invalid_source_json", func(input *generation.DiagramInput) { input.Manifest.Sources[0].Content = json.RawMessage(`{`) }, generation.ErrInvalidInput},
		{"duplicate_source", func(input *generation.DiagramInput) {
			input.Manifest.Sources = append(input.Manifest.Sources, input.Manifest.Sources[0])
		}, generation.ErrInvalidInput},
		{"injected_role", func(input *generation.DiagramInput) { input.Manifest.Messages[0].Role = "system" }, generation.ErrInvalidInput},
		{"bad_base_graph", func(input *generation.DiagramInput) { input.BaseGraph = &generation.DiagramGraph{} }, generation.ErrInvalidInput},
		{"input_budget", func(input *generation.DiagramInput) {
			input.Manifest.Project.Description = strings.Repeat("a", MaxDiagramInputBytes)
		}, generation.ErrBudgetExceeded},
	}
	for _, scenario := range cases {
		test.Run(scenario.name, func(test *testing.T) {
			input := diagramInputFixture("flowchart")
			scenario.change(&input)
			mock := &mockLLMClient{}
			candidate, err := NewDiagramEngine(mock).GenerateDiagram(context.Background(), input, nil)
			if candidate != nil || !errors.Is(err, scenario.want) || len(mock.prompts) != 0 {
				test.Fatalf("expected pre-LLM rejection %v, got candidate=%+v error=%v", scenario.want, candidate, err)
			}
		})
	}
}

type diagramClientFunc func(context.Context, string, []LLMMessage, string, generation.ProgressSink) (string, error)

func (client diagramClientFunc) CallLLMStreamContext(ctx context.Context, system string, history []LLMMessage, prompt string, sink generation.ProgressSink) (string, error) {
	return client(ctx, system, history, prompt, sink)
}

func TestDiagramEngineErrorAndCancellation(test *testing.T) {
	input := diagramInputFixture("erd")
	if _, err := NewDiagramEngine(nil).GenerateDiagram(context.Background(), input, nil); !errors.Is(err, generation.ErrEngineUnavailable) {
		test.Fatalf("nil client: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	mock := &mockLLMClient{}
	if _, err := NewDiagramEngine(mock).GenerateDiagram(ctx, input, nil); !errors.Is(err, context.Canceled) || len(mock.prompts) != 0 {
		test.Fatalf("pre-cancellation: %v", err)
	}
	transportErr := errors.New("provider unavailable")
	mock = &mockLLMClient{err: transportErr}
	if _, err := NewDiagramEngine(mock).GenerateDiagram(context.Background(), input, nil); !errors.Is(err, transportErr) {
		test.Fatalf("transport error not preserved: %v", err)
	}
	ctx, cancel = context.WithCancel(context.Background())
	defer cancel()
	calls := 0
	client := diagramClientFunc(func(context.Context, string, []LLMMessage, string, generation.ProgressSink) (string, error) {
		calls++
		cancel()
		return diagramJSON(test, diagramFixture("erd")), nil
	})
	if candidate, err := NewDiagramEngine(client).GenerateDiagram(ctx, input, nil); candidate != nil || !errors.Is(err, context.Canceled) || calls != 1 {
		test.Fatalf("canceled inference returned a candidate: %+v %v", candidate, err)
	}
	mock = &mockLLMClient{responses: []string{strings.Repeat("a", MaxDiagramResponseBytes+1)}}
	if candidate, err := NewDiagramEngine(mock).GenerateDiagram(context.Background(), input, nil); candidate != nil || !errors.Is(err, generation.ErrDiagramTooLarge) || mock.callIndex != 1 {
		test.Fatalf("oversized response was not stopped: %+v %v", candidate, err)
	}
}
