package generation

import "testing"

func TestArtifactCandidateValidation(test *testing.T) {
	valid := ArtifactCandidate{Type: "prd", Title: "Draft", DocumentContent: "# Requirements", ChangeSummary: "Created", AssistantMessage: "Ready", Format: "markdown", SchemaVersion: 1}
	if err := valid.Validate(); err != nil {
		test.Fatal(err)
	}
	var absent *ArtifactCandidate
	if err := absent.Validate(); err == nil {
		test.Fatal("nil candidate accepted")
	}
	for _, mutate := range []func(*ArtifactCandidate){
		func(candidate *ArtifactCandidate) { candidate.Type = "wireframe" },
		func(candidate *ArtifactCandidate) { candidate.Format = "graph" },
		func(candidate *ArtifactCandidate) { candidate.SchemaVersion = 2 },
		func(candidate *ArtifactCandidate) { candidate.Title = " " },
		func(candidate *ArtifactCandidate) { candidate.AssistantMessage = "" },
		func(candidate *ArtifactCandidate) { candidate.DocumentContent = "" },
		func(candidate *ArtifactCandidate) { candidate.Graph = &DiagramGraph{} },
		func(candidate *ArtifactCandidate) { candidate.Type = "erd"; candidate.Format = "graph" },
	} {
		candidate := valid
		mutate(&candidate)
		if err := candidate.Validate(); err == nil {
			test.Fatalf("invalid candidate accepted: %+v", candidate)
		}
	}
	graph := ArtifactCandidate{Type: "flowchart", Title: "Flow", ChangeSummary: "Created", AssistantMessage: "Ready", Format: "graph", SchemaVersion: 1,
		Graph: &DiagramGraph{Nodes: []DiagramNode{{ID: "start", Type: "start", Data: DiagramNodeData{Label: "Mulai"}}}, Edges: []DiagramEdge{}, Metadata: DiagramMetadata{DiagramType: "flowchart"}},
	}
	if err := graph.Validate(); err != nil {
		test.Fatal(err)
	}
	graph.DocumentContent = "mixed content"
	if err := graph.Validate(); err == nil {
		test.Fatal("mixed graph/document accepted")
	}
}
