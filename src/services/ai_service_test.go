package services

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/Bobby-P-dev/go-diagram.git/src/dtos"
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

func TestSanitizeJSONResponse(t *testing.T) {
	// Case 1: Wrapped in markdown block with trailing comma
	raw := "```json\n{\"message\": \"ok\", \"nodes\": [{\"id\": \"node-1\",},], \"edges\": [],}\n```"
	sanitized := sanitizeJSONResponse(raw)
	expected := "{\"message\": \"ok\", \"nodes\": [{\"id\": \"node-1\"}], \"edges\": []}"
	if sanitized != expected {
		t.Errorf("Expected:\n%s\nGot:\n%s", expected, sanitized)
	}

	// Case 2: Extra conversational text around JSON
	raw2 := "Here is your diagram:\n{\"message\": \"done\", \"nodes\": []}\nHope this helps!"
	sanitized2 := sanitizeJSONResponse(raw2)
	expected2 := "{\"message\": \"done\", \"nodes\": []}"
	if sanitized2 != expected2 {
		t.Errorf("Expected:\n%s\nGot:\n%s", expected2, sanitized2)
	}

	// Case 3: UTF-8 BOM
	raw3 := "\xef\xbb\xbf{\"message\": \"bom\"}"
	sanitized3 := sanitizeJSONResponse(raw3)
	expected3 := "{\"message\": \"bom\"}"
	if sanitized3 != expected3 {
		t.Errorf("Expected:\n%s\nGot:\n%s", expected3, sanitized3)
	}

	// Case 4: Embedded CSS braces inside HTML string without closing fence
	raw4 := "```json\n{\"title\": \"demo\", \"raw_html\": \"<style>.card { color: red; }</style><div class=\\\"flex\\\">content</div>\"}\n"
	sanitized4 := sanitizeJSONResponse(raw4)
	var parsed4 map[string]interface{}
	if err := json.Unmarshal([]byte(sanitized4), &parsed4); err != nil {
		t.Fatalf("Case 4 failed to parse as valid JSON: %v. Raw:\n%s", err, sanitized4)
	}
	if parsed4["title"] != "demo" {
		t.Errorf("Case 4 title mismatch, got: %v", parsed4["title"])
	}

	// Case 5: Truncated JSON before root closing brace
	raw5 := "{\"title\": \"demo\", \"items\": [\"a\", \"b\""
	sanitized5 := sanitizeJSONResponse(raw5)
	var parsed5 map[string]interface{}
	if err := json.Unmarshal([]byte(sanitized5), &parsed5); err != nil {
		t.Fatalf("Case 5 failed to repair truncated JSON: %v. Output:\n%s", err, sanitized5)
	}

	// Case 6: Truncated JSON inside open string
	raw6 := "```json\n{\"title\": \"demo\", \"raw_html\": \"<div class=\\\"flex font-bold\\\">"
	sanitized6 := sanitizeJSONResponse(raw6)
	var parsed6 map[string]interface{}
	if err := json.Unmarshal([]byte(sanitized6), &parsed6); err != nil {
		t.Fatalf("Case 6 failed to repair truncated string in JSON: %v. Output:\n%s", err, sanitized6)
	}
	if parsed6["title"] != "demo" {
		t.Errorf("Case 6 title mismatch: %v", parsed6["title"])
	}
}

func TestNewDiagramTypesPromptGuidelines(t *testing.T) {
	types := []string{"sequence", "c4", "network", "cicd"}
	for _, dt := range types {
		if !strings.Contains(baseSystemPrompt, strings.ToUpper(dt)) && !strings.Contains(baseSystemPrompt, dt) {
			t.Errorf("baseSystemPrompt should contain guidelines for %s", dt)
		}
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

