package integration

import (
	"strings"
	"testing"

	dtos "github.com/Bobby-P-dev/go-diagram.git/internal/modules/diagram/dto"
)

func TestSwimlanePromptGuidelines(t *testing.T) {
	// Verify baseSystemPrompt has lane and dashed specifications
	if !strings.Contains(baseSystemPrompt, "\"lane\"") {
		t.Errorf("baseSystemPrompt should mention lane")
	}

	diagramType := "swimlane"
	systemPrompt := baseSystemPrompt
	if diagramType != "" {
		switch strings.ToLower(diagramType) {
		case "swimlane", "bpmn":
			systemPrompt += "\n- MANDATORY: Design a Cross-Functional Swimlane BPMN Flowchart ala Eraser.io."
		default:
			t.Errorf("expected swimlane case to match")
		}
	}

	if !strings.Contains(systemPrompt, "Swimlane BPMN") {
		t.Errorf("systemPrompt should contain Swimlane BPMN guidelines")
	}
}

func TestERDPromptGuidelines(t *testing.T) {
	diagramType := "erd"
	systemPrompt := baseSystemPrompt
	if diagramType != "" {
		switch strings.ToLower(diagramType) {
		case "erd", "database":
			systemPrompt += "\n- MANDATORY: Design relational database tables.\n- STRICT ERD RULE: DO NOT generate any 'lane' property. ERD diagrams MUST be pure relational table structures without swimlanes, lanes, or departmental bands. Keep 'lane' empty or omit it completely."
		}
	}

	if !strings.Contains(systemPrompt, "STRICT ERD RULE") {
		t.Errorf("systemPrompt should contain STRICT ERD RULE")
	}
	if !strings.Contains(systemPrompt, "without swimlanes") {
		t.Errorf("systemPrompt should explicitly forbid swimlanes for ERD")
	}
}

func TestNewDiagramTypesPromptGuidelines(t *testing.T) {
	types := []string{"sequence", "c4", "network", "cicd", "usecase"}
	for _, dt := range types {
		if !strings.Contains(baseSystemPrompt, strings.ToUpper(dt)) && !strings.Contains(baseSystemPrompt, dt) {
			t.Errorf("baseSystemPrompt should contain guidelines for %s", dt)
		}
	}
}

func TestUseCasePromptGuidelines(t *testing.T) {
	if !strings.Contains(baseSystemPrompt, "\"actor\"") {
		t.Errorf("baseSystemPrompt should mention actor")
	}
	if !strings.Contains(baseSystemPrompt, "\"usecase\"") {
		t.Errorf("baseSystemPrompt should mention usecase")
	}
}

func TestValidateAndSanitizeGraph(t *testing.T) {
	nodes := []dtos.GraphNode{
		{ID: "node-1", Type: "input", Data: dtos.NodeData{Label: "Start", Lane: "LANE_A"}},
		{ID: "node-2", Type: "decision", Data: dtos.NodeData{Label: "Check", Lane: "LANE_A"}},
		{ID: "node-1", Type: "input", Data: dtos.NodeData{Label: "Duplicate ID", Lane: "LANE_A"}},
		{ID: "node-3", Type: "output", Data: dtos.NodeData{Label: "End", Lane: "LANE_A"}},
	}

	edges := []dtos.GraphEdge{
		{ID: "e1", Source: "node-1", Target: "node-2", SourceHandle: "bottom", TargetHandle: "top"},
		{ID: "e2", Source: "node-2", Target: "node-3", SourceHandle: "bottom", TargetHandle: "top", Label: "Yes"},
		{ID: "orphan-1", Source: "node-2", Target: "node-nonexistent", SourceHandle: "right", TargetHandle: "top", Label: "No"},
		{ID: "orphan-2", Source: "ghost-node", Target: "node-3"},
		{ID: "", Source: "node-1", Target: "node-3"}, // missing ID
	}

	// Case 1: Flowchart (should strip lane, deduplicate node-1, prune 2 orphan edges, assign edge ID)
	cleanNodes, cleanEdges := validateAndSanitizeGraph(nodes, edges, "flowchart")

	if len(cleanNodes) != 3 {
		t.Fatalf("expected 3 deduplicated nodes, got %d", len(cleanNodes))
	}
	for _, n := range cleanNodes {
		if n.Data.Lane != "" {
			t.Errorf("expected lane to be stripped for flowchart, got: %s", n.Data.Lane)
		}
	}

	if len(cleanEdges) != 3 {
		t.Fatalf("expected 3 valid edges (2 orphans pruned), got %d", len(cleanEdges))
	}
	for _, e := range cleanEdges {
		if e.ID == "" {
			t.Errorf("expected edge to have a generated ID, got empty")
		}
		if e.Source == "ghost-node" || e.Target == "node-nonexistent" {
			t.Errorf("orphan edge was not pruned: %v", e)
		}
	}

	// Verify handle preservation
	if cleanEdges[0].SourceHandle != "bottom" || cleanEdges[0].TargetHandle != "top" {
		t.Errorf("expected handles to be preserved, got source: %s, target: %s", cleanEdges[0].SourceHandle, cleanEdges[0].TargetHandle)
	}

	// Case 2: Swimlane (should preserve lane)
	swimNodes, _ := validateAndSanitizeGraph(nodes, edges, "swimlane")
	if len(swimNodes) != 3 {
		t.Fatalf("expected 3 nodes, got %d", len(swimNodes))
	}
	if swimNodes[0].Data.Lane != "LANE_A" {
		t.Errorf("expected lane to be preserved for swimlane diagram, got: %s", swimNodes[0].Data.Lane)
	}
}
