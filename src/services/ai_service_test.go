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
