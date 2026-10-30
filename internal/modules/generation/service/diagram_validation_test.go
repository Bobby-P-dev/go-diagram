package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/Bobby-P-dev/go-diagram.git/internal/modules/generation"
)

func TestDiagramParserStrictJSON(test *testing.T) {
	good := diagramJSON(test, diagramFixture("erd"))
	cases := map[string]string{
		"invalid_json":        `{`,
		"prose":               "Here is the graph: " + good,
		"trailing_json":       good + `{}`,
		"trailing_garbage":    good + `x`,
		"root_array":          `[]`,
		"root_null":           `null`,
		"missing_summary":     strings.Replace(good, `"change_summary":`, `"other":`, 1),
		"spoofed_sources":     strings.Replace(good, `{"title":`, `{"sources":[],"title":`, 1),
		"duplicate_title":     strings.Replace(good, `{"title":`, `{"title":"Injected","title":`, 1),
		"duplicate_nested_id": strings.Replace(good, `"id":"customer"`, `"id":"evil","id":"customer"`, 1),
		"escaped_duplicate":   strings.Replace(good, `"id":"customer"`, `"\u0069d":"evil","id":"customer"`, 1),
		"unknown_nested":      strings.Replace(good, `"label":"Pelanggan"`, `"label":"Pelanggan","html":"<script>"`, 1),
		"case_alias":          strings.Replace(good, `"label":"Pelanggan"`, `"Label":"Pelanggan"`, 1),
		"null_edges":          strings.Replace(good, `"graph_edges":[`, `"graph_edges":null,"unused":[`, 1),
		"null_label":          strings.Replace(good, `"label":"Pelanggan"`, `"label":null`, 1),
		"missing_boolean":     strings.Replace(good, `,"nullable":false`, ``, 1),
		"wrong_boolean_type":  strings.Replace(good, `"nullable":false`, `"nullable":"false"`, 1),
		"invalid_utf8":        string([]byte{255}),
		"nul_label":           strings.Replace(good, `"label":"Pelanggan"`, `"label":"Pelanggan\u0000"`, 1),
		"deep_nesting":        strings.Repeat("[", 40) + `1` + strings.Repeat("]", 40),
	}
	for name, raw := range cases {
		test.Run(name, func(test *testing.T) {
			if _, err := parseDiagram(raw, "erd"); !errors.Is(err, generation.ErrInvalidDiagram) {
				test.Fatalf("expected invalid diagram, got %v", err)
			}
		})
	}
	for _, raw := range []string{good, "```json\n" + good + "\n```", "```\n" + good + "\n```"} {
		if _, err := parseDiagram(raw, "erd"); err != nil {
			test.Fatalf("valid/fenced output rejected: %v", err)
		}
	}
}

func TestDiagramGraphValidation(test *testing.T) {
	cases := []struct {
		name        string
		diagramType string
		change      func(*diagramPayload)
	}{
		{"empty_nodes", "erd", func(payload *diagramPayload) { payload.GraphNodes = []generation.DiagramNode{} }},
		{"null_edges", "erd", func(payload *diagramPayload) { payload.GraphEdges = nil }},
		{"duplicate_nodes", "erd", func(payload *diagramPayload) { payload.GraphNodes[1].ID = payload.GraphNodes[0].ID }},
		{"invalid_id", "erd", func(payload *diagramPayload) { payload.GraphNodes[0].ID = "bad id" }},
		{"dangling_edge", "erd", func(payload *diagramPayload) { payload.GraphEdges[0].Source = "missing" }},
		{"duplicate_edge", "erd", func(payload *diagramPayload) { payload.GraphEdges = append(payload.GraphEdges, payload.GraphEdges[0]) }},
		{"wrong_metadata", "erd", func(payload *diagramPayload) { payload.Metadata.DiagramType = "flowchart" }},
		{"empty_title", "erd", func(payload *diagramPayload) { payload.Title = "  " }},
		{"long_summary", "erd", func(payload *diagramPayload) { payload.ChangeSummary = strings.Repeat("a", 2049) }},
		{"multibyte_summary", "erd", func(payload *diagramPayload) { payload.ChangeSummary = strings.Repeat("界", 700) }},
		{"empty_label", "erd", func(payload *diagramPayload) { payload.GraphNodes[0].Data.Label = " " }},
		{"empty_fields", "erd", func(payload *diagramPayload) { payload.GraphNodes[0].Data.Fields = []generation.DiagramField{} }},
		{"duplicate_field", "erd", func(payload *diagramPayload) {
			payload.GraphNodes[0].Data.Fields = append(payload.GraphNodes[0].Data.Fields, payload.GraphNodes[0].Data.Fields[0])
		}},
		{"nullable_primary", "erd", func(payload *diagramPayload) { payload.GraphNodes[0].Data.Fields[0].Nullable = true }},
		{"missing_datatype", "erd", func(payload *diagramPayload) { payload.GraphNodes[0].Data.Fields[0].DataType = " " }},
		{"bad_cardinality", "erd", func(payload *diagramPayload) { payload.GraphEdges[0].Cardinality = "many" }},
		{"sequence_on_erd", "erd", func(payload *diagramPayload) { payload.GraphEdges[0].SequenceNumber = 1 }},
		{"bad_node_type", "flowchart", func(payload *diagramPayload) { payload.GraphNodes[0].Type = "database" }},
		{"unlabelled_decision", "flowchart", func(payload *diagramPayload) {
			payload.GraphNodes[0].Type = "decision"
			payload.GraphEdges[0].Label = ""
		}},
		{"fields_on_flow", "flowchart", func(payload *diagramPayload) {
			payload.GraphNodes[0].Data.Fields = []generation.DiagramField{{Name: "id", DataType: "uuid"}}
		}},
		{"cardinality_on_flow", "flowchart", func(payload *diagramPayload) { payload.GraphEdges[0].Cardinality = "1:N" }},
		{"empty_sequence", "sequence", func(payload *diagramPayload) { payload.GraphEdges = []generation.DiagramEdge{} }},
		{"wrong_participant", "sequence", func(payload *diagramPayload) { payload.GraphNodes[0].Type = "process" }},
		{"unordered_message", "sequence", func(payload *diagramPayload) { payload.GraphEdges[0].SequenceNumber = 2 }},
		{"empty_message", "sequence", func(payload *diagramPayload) { payload.GraphEdges[0].Label = " " }},
		{"bad_message_type", "sequence", func(payload *diagramPayload) { payload.GraphEdges[0].MessageType = "request" }},
	}
	for _, scenario := range cases {
		test.Run(scenario.name, func(test *testing.T) {
			payload := diagramFixture(scenario.diagramType)
			scenario.change(&payload)
			if _, err := parseDiagram(diagramJSON(test, payload), scenario.diagramType); !errors.Is(err, generation.ErrInvalidDiagram) {
				test.Fatalf("expected invalid diagram: %v", err)
			}
		})
	}
}

func TestDiagramGraphBoundariesAndMeaningfulLoops(test *testing.T) {
	for _, diagramType := range []string{"erd", "flowchart", "sequence"} {
		payload := diagramFixture(diagramType)
		payload.GraphEdges[0].Target = payload.GraphEdges[0].Source
		if _, err := parseDiagram(diagramJSON(test, payload), diagramType); err != nil {
			test.Fatalf("self relationship should be allowed: %v", err)
		}
		if diagramType != "sequence" {
			payload.GraphEdges = []generation.DiagramEdge{}
			if _, err := parseDiagram(diagramJSON(test, payload), diagramType); err != nil {
				test.Fatalf("disconnected graph should be allowed: %v", err)
			}
		}
	}
	payload := diagramFixture("flowchart")
	payload.GraphNodes = make([]generation.DiagramNode, MaxDiagramNodes+1)
	if _, err := parseDiagram(diagramJSON(test, payload), "flowchart"); err == nil {
		test.Fatal("node limit not enforced")
	}
	payload = diagramFixture("flowchart")
	payload.GraphEdges = make([]generation.DiagramEdge, MaxDiagramEdges+1)
	if _, err := parseDiagram(diagramJSON(test, payload), "flowchart"); err == nil {
		test.Fatal("edge limit not enforced")
	}
	payload = diagramFixture("erd")
	payload.GraphNodes[0].Data.Fields = make([]generation.DiagramField, MaxDiagramFields+1)
	if _, err := parseDiagram(diagramJSON(test, payload), "erd"); err == nil {
		test.Fatal("field limit not enforced")
	}
	for _, diagramType := range []string{"erd", "flowchart", "sequence"} {
		var schema map[string]any
		if err := json.Unmarshal([]byte(diagramOutputSchema(diagramType)), &schema); err != nil || schema["additionalProperties"] != false {
			test.Fatalf("invalid JSON Schema for %s: %v", diagramType, err)
		}
	}
}

func TestDiagramExactLimitsAndGraphByteBudget(test *testing.T) {
	payload := diagramFixture("flowchart")
	payload.GraphNodes = make([]generation.DiagramNode, MaxDiagramNodes)
	for index := range payload.GraphNodes {
		payload.GraphNodes[index] = generation.DiagramNode{ID: fmt.Sprintf("node-%d", index), Type: "process", Data: generation.DiagramNodeData{Label: "Proses"}}
	}
	payload.GraphEdges = make([]generation.DiagramEdge, MaxDiagramEdges)
	for index := range payload.GraphEdges {
		payload.GraphEdges[index] = generation.DiagramEdge{ID: fmt.Sprintf("edge-%d", index), Source: "node-0", Target: "node-1"}
	}
	if _, err := parseDiagram(diagramJSON(test, payload), "flowchart"); err != nil {
		test.Fatalf("exact node/edge limits should be accepted: %v", err)
	}
	for index := range payload.GraphNodes {
		payload.GraphNodes[index].Data.Description = strings.Repeat("a", 2000)
	}
	if _, err := parseDiagram(diagramJSON(test, payload), "flowchart"); !errors.Is(err, generation.ErrInvalidDiagram) || !strings.Contains(err.Error(), "size limit") {
		test.Fatalf("total graph byte budget not enforced: %v", err)
	}
	payload = diagramFixture("erd")
	payload.GraphNodes[0].Data.Fields = make([]generation.DiagramField, MaxDiagramFields)
	for index := range payload.GraphNodes[0].Data.Fields {
		payload.GraphNodes[0].Data.Fields[index] = generation.DiagramField{Name: fmt.Sprintf("field_%d", index), DataType: "text"}
	}
	if _, err := parseDiagram(diagramJSON(test, payload), "erd"); err != nil {
		test.Fatalf("exact field limit should be accepted: %v", err)
	}
}

func FuzzParseDiagram(fuzz *testing.F) {
	fuzz.Add(`{}`)
	fuzz.Add(`{"title":null}`)
	fuzz.Add(`{"graph_nodes":[{"id":"x","id":"y"}]}`)
	fuzz.Fuzz(func(test *testing.T, raw string) {
		if len(raw) > MaxDiagramResponseBytes {
			test.Skip()
		}
		for _, diagramType := range []string{"erd", "flowchart", "sequence"} {
			_, _ = parseDiagram(raw, diagramType)
		}
	})
}
