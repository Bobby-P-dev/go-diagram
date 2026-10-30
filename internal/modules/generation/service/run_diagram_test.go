package service

import (
	"context"
	"errors"
	"testing"

	"github.com/Bobby-P-dev/go-diagram.git/internal/modules/generation"
)

type diagramRunEngine func(context.Context, generation.DiagramInput, generation.ProgressSink) (*generation.DiagramCandidate, error)

func (engine diagramRunEngine) GenerateDiagram(ctx context.Context, input generation.DiagramInput, sink generation.ProgressSink) (*generation.DiagramCandidate, error) {
	return engine(ctx, input, sink)
}

func TestRunCoordinatorDiagramDispatch(test *testing.T) {
	for _, kind := range []string{"erd", "flowchart", "sequence"} {
		test.Run(kind, func(test *testing.T) {
			input := diagramInputFixture(kind)
			payload := diagramFixture(kind)
			base := &generation.DiagramGraph{Nodes: payload.GraphNodes, Edges: payload.GraphEdges, Metadata: payload.Metadata}
			run := generation.Run{ID: "diagram-run", UserID: "user", ProjectID: input.Manifest.Project.ID, ThreadID: input.Manifest.ThreadID, OutputType: kind, OutputTitle: input.Title, Prompt: input.Prompt, Manifest: input.Manifest, BaseGraph: base, Status: "running"}
			generated, committed := false, false
			engine := diagramRunEngine(func(_ context.Context, received generation.DiagramInput, _ generation.ProgressSink) (*generation.DiagramCandidate, error) {
				generated = true
				if received.Type != kind || received.BaseGraph != base || received.Manifest != input.Manifest || received.Prompt != input.Prompt {
					test.Fatal("worker lost frozen input")
				}
				return &generation.DiagramCandidate{Type: kind, Title: payload.Title, GraphNodes: payload.GraphNodes, GraphEdges: payload.GraphEdges, Metadata: payload.Metadata, ChangeSummary: payload.ChangeSummary, AssistantMessage: payload.AssistantMessage, Format: "graph", SchemaVersion: 1}, nil
			})
			repository := &fakeRunRepo{
				claimFn: func(context.Context, generation.Scope, string) (*generation.Run, bool, error) { return &run, true, nil },
				commitFn: func(_ context.Context, _ generation.Scope, _ string, candidate *generation.ArtifactCandidate) (*generation.Run, error) {
					committed = true
					if candidate.Format != "graph" || candidate.Graph == nil || candidate.DocumentContent != "" || candidate.Type != kind {
						test.Fatalf("invalid commit candidate: %+v", candidate)
					}
					return &generation.Run{Status: "succeeded"}, nil
				},
			}
			NewRunCoordinator(repository, &testResolver{}, nil, engine).execute(context.Background(), run)
			if !generated || !committed {
				test.Fatal("diagram was not generated and committed")
			}
		})
	}
}

func TestRunCoordinatorDiagramErrors(test *testing.T) {
	for _, scenario := range []struct {
		name string
		err  error
		code string
	}{
		{"repair", generation.ErrDiagramRepairFailed, "REPAIR_FAILED"},
		{"oversize", generation.ErrDiagramTooLarge, "DIAGRAM_TOO_LARGE"},
		{"unavailable", generation.ErrEngineUnavailable, "ENGINE_UNAVAILABLE"},
		{"provider", errors.New("provider failed"), "GENERATION_FAILED"},
	} {
		test.Run(scenario.name, func(test *testing.T) {
			run := generation.Run{ID: "run", OutputType: "erd"}
			code := ""
			repository := &fakeRunRepo{
				claimFn: func(context.Context, generation.Scope, string) (*generation.Run, bool, error) { return &run, true, nil },
				commitFn: func(context.Context, generation.Scope, string, *generation.ArtifactCandidate) (*generation.Run, error) {
					test.Fatal("invalid output committed")
					return nil, nil
				},
				finishFn: func(_ context.Context, _ generation.Scope, _, _, actual, _ string) error { code = actual; return nil },
			}
			engine := diagramRunEngine(func(context.Context, generation.DiagramInput, generation.ProgressSink) (*generation.DiagramCandidate, error) {
				return nil, scenario.err
			})
			NewRunCoordinator(repository, &testResolver{}, nil, engine).execute(context.Background(), run)
			if code != scenario.code {
				test.Fatalf("got %s, want %s", code, scenario.code)
			}
		})
	}
	coordinator := NewRunCoordinator(&fakeRunRepo{}, &testResolver{}, nil, nil)
	for _, kind := range []string{"prd", "erd"} {
		if _, err := coordinator.generate(context.Background(), &generation.Run{OutputType: kind}); !errors.Is(err, generation.ErrEngineUnavailable) {
			test.Fatal(err)
		}
	}
}

func TestRunCoordinatorAcceptsDiagramTypes(test *testing.T) {
	scope := generation.Scope{UserID: "11111111-1111-4111-8111-111111111111", ProjectID: "22222222-2222-4222-8222-222222222222", ThreadID: "33333333-3333-4333-8333-333333333333"}
	coordinator := NewRunCoordinator(&fakeRunRepo{}, &testResolver{}, nil, nil)
	for _, kind := range []string{"brainstorm", "prd", "erd", "flowchart", "sequence"} {
		if _, err := coordinator.Start(context.Background(), scope, generation.StartInput{TriggerMessageID: "44444444-4444-4444-8444-444444444444", OutputType: kind}, kind); err != nil {
			test.Fatalf("%s: %v", kind, err)
		}
	}
	for _, kind := range []string{"ui_design", "wireframe", "sequence_diagram", "unknown"} {
		if _, err := coordinator.Start(context.Background(), scope, generation.StartInput{TriggerMessageID: "44444444-4444-4444-8444-444444444444", OutputType: kind}, kind); !errors.Is(err, generation.ErrUnsupportedType) {
			test.Fatalf("accepted unsupported type %s: %v", kind, err)
		}
	}
}
