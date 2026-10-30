package generation

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"
)

const (
	MaxDiagramBytes  = 256 * 1024
	MaxDiagramNodes  = 200
	MaxDiagramEdges  = 500
	MaxDiagramFields = 100
)

var diagramIDPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]{0,79}$`)

func invalidDiagram(reason string) error { return fmt.Errorf("%w: %s", ErrInvalidDiagram, reason) }
func diagramText(value string, maxRunes, maxBytes int, required bool) bool {
	return utf8.ValidString(value) && !strings.ContainsRune(value, '\x00') && len(value) <= maxBytes && utf8.RuneCountInString(value) <= maxRunes && (!required || strings.TrimSpace(value) != "")
}

func ValidateDiagramGraph(graph DiagramGraph, diagramType string) error {
	if graph.Metadata.DiagramType != diagramType {
		return invalidDiagram("metadata.diagram_type must match requested type")
	}
	if graph.Nodes == nil || len(graph.Nodes) < 1 || len(graph.Nodes) > MaxDiagramNodes || graph.Edges == nil || len(graph.Edges) > MaxDiagramEdges {
		return invalidDiagram("graph requires 1..200 nodes and 0..500 edges, not null")
	}
	encoded, err := json.Marshal(graph)
	if err != nil || len(encoded) > MaxDiagramBytes {
		return invalidDiagram("graph exceeds size limit")
	}
	nodes := make(map[string]string, len(graph.Nodes))
	for _, node := range graph.Nodes {
		if !diagramIDPattern.MatchString(node.ID) || nodes[node.ID] != "" || !diagramText(node.Data.Label, 255, 1024, true) || !diagramText(node.Data.Description, 2048, 2048, false) {
			return invalidDiagram("nodes require unique valid IDs, nonempty labels and bounded text")
		}
		switch diagramType {
		case "erd":
			if node.Type != "entity" || len(node.Data.Fields) == 0 || len(node.Data.Fields) > MaxDiagramFields {
				return invalidDiagram("ERD nodes must be entities with 1..100 fields")
			}
			fields := make(map[string]bool, len(node.Data.Fields))
			for _, field := range node.Data.Fields {
				if !diagramText(field.Name, 128, 512, true) || !diagramText(field.DataType, 128, 512, true) || fields[field.Name] || (field.PrimaryKey && field.Nullable) {
					return invalidDiagram("ERD fields need unique names and data types; primary keys cannot be nullable")
				}
				fields[field.Name] = true
			}
		case "flowchart":
			if node.Type != "start" && node.Type != "process" && node.Type != "decision" && node.Type != "end" {
				return invalidDiagram("flowchart node type must be start, process, decision or end")
			}
		case "sequence":
			if node.Type != "participant" {
				return invalidDiagram("sequence nodes must be participants")
			}
		default:
			return invalidDiagram("unsupported graph type")
		}
		if diagramType != "erd" && node.Data.Fields != nil {
			return invalidDiagram("fields belong only to ERD entities")
		}
		nodes[node.ID] = node.Type
	}
	if diagramType == "sequence" && len(graph.Edges) == 0 {
		return invalidDiagram("sequence requires at least one message")
	}
	edges := make(map[string]bool, len(graph.Edges))
	for index, edge := range graph.Edges {
		if !diagramIDPattern.MatchString(edge.ID) || edges[edge.ID] || nodes[edge.Source] == "" || nodes[edge.Target] == "" || !diagramText(edge.Label, 1024, 4096, false) {
			return invalidDiagram("edges require unique valid IDs, existing endpoints and bounded labels")
		}
		edges[edge.ID] = true
		if diagramType == "erd" {
			if edge.Cardinality != "1:1" && edge.Cardinality != "1:N" && edge.Cardinality != "N:1" && edge.Cardinality != "N:M" {
				return invalidDiagram("ERD edges require cardinality 1:1, 1:N, N:1 or N:M")
			}
		} else if edge.Cardinality != "" {
			return invalidDiagram("cardinality belongs only to ERD edges")
		}
		if diagramType == "sequence" {
			if edge.SequenceNumber != index+1 || strings.TrimSpace(edge.Label) == "" || (edge.MessageType != "call" && edge.MessageType != "return" && edge.MessageType != "async") {
				return invalidDiagram("sequence messages need labels, ordered sequence_number 1..N and call/return/async message_type")
			}
		} else if edge.SequenceNumber != 0 || edge.MessageType != "" {
			return invalidDiagram("message fields belong only to sequence edges")
		}
		if diagramType == "flowchart" && nodes[edge.Source] == "decision" && strings.TrimSpace(edge.Label) == "" {
			return invalidDiagram("outgoing decision edges require branch labels")
		}
	}
	return nil
}
