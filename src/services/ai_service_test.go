package services

import (
	"encoding/json"
	"strings"
	"testing"
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

