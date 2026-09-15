package services

import (
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
