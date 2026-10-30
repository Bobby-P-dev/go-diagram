package service

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/Bobby-P-dev/go-diagram.git/internal/modules/generation"
)

func TestDiagramEngineTargetedPatch(test *testing.T) {
	for _, kind := range []string{"erd", "flowchart", "sequence"} {
		test.Run(kind, func(test *testing.T) {
			fixture := diagramFixture(kind)
			input := diagramInputFixture(kind)
			input.BaseGraph = &generation.DiagramGraph{Nodes: fixture.GraphNodes, Edges: fixture.GraphEdges, Metadata: fixture.Metadata}
			input.Mutation = &generation.DiagramMutation{NodeIDs: []string{"customer"}, EdgeIDs: []string{}}
			patch := fixture
			patch.GraphNodes = append([]generation.DiagramNode{}, fixture.GraphNodes[:1]...)
			patch.GraphNodes[0].Data.Label = "Pembeli"
			patch.GraphEdges = []generation.DiagramEdge{}
			mock := &mockLLMClient{responses: []string{diagramJSON(test, fixture), diagramJSON(test, patch)}}
			candidate, err := NewDiagramEngine(mock).GenerateDiagram(context.Background(), input, nil)
			if err != nil {
				test.Fatal(err)
			}
			if candidate.GraphNodes[0].Data.Label != "Pembeli" || candidate.GraphNodes[1].Data.Label != fixture.GraphNodes[1].Data.Label || len(candidate.GraphEdges) != 1 || len(mock.prompts) != 2 {
				test.Fatal("patch merge/repair failed")
			}
			if !strings.Contains(mock.systems[0], "UPDATE ONLY") || !strings.Contains(mock.prompts[1], "mutation") {
				test.Fatal("scope missing in initial/repair request")
			}
			mock = &mockLLMClient{responses: []string{diagramJSON(test, fixture), diagramJSON(test, fixture), diagramJSON(test, fixture)}}
			if _, err := NewDiagramEngine(mock).GenerateDiagram(context.Background(), input, nil); !errors.Is(err, generation.ErrDiagramRepairFailed) || len(mock.prompts) != 3 {
				test.Fatalf("unsafe output did not fail closed: %v", err)
			}
			input.Mutation.NodeIDs = []string{"missing"}
			mock = &mockLLMClient{}
			if _, err := NewDiagramEngine(mock).GenerateDiagram(context.Background(), input, nil); !errors.Is(err, generation.ErrInvalidInput) || len(mock.prompts) != 0 {
				test.Fatal("invalid target reached provider")
			}
		})
	}
}

func TestMutationCoordinatorRejectsInvalidModes(test *testing.T) {
	scope := generation.Scope{UserID: "11111111-1111-4111-8111-111111111111", ProjectID: "22222222-2222-4222-8222-222222222222", ThreadID: "33333333-3333-4333-8333-333333333333"}
	coordinator := NewRunCoordinator(&fakeRunRepo{}, &testResolver{}, nil, nil)
	for _, kind := range []string{"prd", "brainstorm", "erd"} {
		input := generation.StartInput{OutputType: kind, TriggerMessageID: "44444444-4444-4444-8444-444444444444", Mutation: &generation.DiagramMutation{NodeIDs: []string{"selected"}, EdgeIDs: []string{}}}
		if kind != "erd" {
			input.TargetArtifactID = "55555555-5555-4555-8555-555555555555"
			input.TargetExpectedRevisionID = "66666666-6666-4666-8666-666666666666"
		}
		if _, err := coordinator.Start(context.Background(), scope, input, kind); !errors.Is(err, generation.ErrInvalidInput) {
			test.Fatalf("accepted %s invalid mode: %v", kind, err)
		}
	}
}
