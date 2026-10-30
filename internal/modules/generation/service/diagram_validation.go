package service

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"unicode/utf8"

	"github.com/Bobby-P-dev/go-diagram.git/internal/modules/generation"
)

func invalidDiagram(reason string) error {
	return fmt.Errorf("%w: %s", generation.ErrInvalidDiagram, reason)
}

func diagramText(value string, maxRunes, maxBytes int, required bool) bool {
	return utf8.ValidString(value) && !strings.ContainsRune(value, '\x00') && len(value) <= maxBytes && utf8.RuneCountInString(value) <= maxRunes && (!required || strings.TrimSpace(value) != "")
}

func parseDiagram(raw, diagramType string) (*diagramPayload, error) {
	payload, err := parseDiagramPayload(raw, diagramType)
	if err != nil {
		return nil, err
	}
	graph := generation.DiagramGraph{Nodes: payload.GraphNodes, Edges: payload.GraphEdges, Metadata: payload.Metadata}
	if err := generation.ValidateDiagramGraph(graph, diagramType); err != nil {
		return nil, err
	}
	return payload, nil
}

func parseDiagramPayload(raw, diagramType string) (*diagramPayload, error) {
	if !utf8.ValidString(raw) {
		return nil, invalidDiagram("output must be UTF-8")
	}
	text := strings.TrimSpace(raw)
	for _, opening := range []string{"```json\n", "```\n"} {
		if strings.HasPrefix(text, opening) && strings.HasSuffix(text, "```") {
			text = strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(text, opening), "```"))
			break
		}
	}
	decoder := json.NewDecoder(strings.NewReader(text))
	decoder.UseNumber()
	value, err := diagramJSONValue(decoder, 0)
	if err != nil {
		return nil, err
	}
	if _, err := decoder.Token(); err != io.EOF {
		return nil, invalidDiagram("output must contain exactly one JSON object")
	}
	if err := validateDiagramShape(value, diagramType); err != nil {
		return nil, err
	}
	decoder = json.NewDecoder(strings.NewReader(text))
	decoder.DisallowUnknownFields()
	var payload diagramPayload
	if err := decoder.Decode(&payload); err != nil {
		return nil, invalidDiagram("output fields must match the typed contract")
	}
	payload.Title = strings.TrimSpace(payload.Title)
	payload.ChangeSummary = strings.TrimSpace(payload.ChangeSummary)
	payload.AssistantMessage = strings.TrimSpace(payload.AssistantMessage)
	if !diagramText(payload.Title, 255, 1024, true) || !diagramText(payload.ChangeSummary, 2048, 2048, true) || !diagramText(payload.AssistantMessage, 4096, 4096, true) {
		return nil, invalidDiagram("title, change_summary and assistant_message must be nonempty bounded strings")
	}
	if payload.Metadata.DiagramType != diagramType {
		return nil, invalidDiagram("metadata must match diagram type")
	}
	return &payload, nil
}

func diagramJSONValue(decoder *json.Decoder, depth int) (any, error) {
	if depth > 32 {
		return nil, invalidDiagram("JSON nesting exceeds limit")
	}
	token, err := decoder.Token()
	if err != nil || token == nil {
		return nil, invalidDiagram("malformed JSON or null value")
	}
	switch token {
	case json.Delim('{'):
		object := map[string]any{}
		for decoder.More() {
			key, err := decoder.Token()
			name, ok := key.(string)
			if err != nil || !ok {
				return nil, invalidDiagram("object keys must be strings")
			}
			if _, exists := object[name]; exists {
				return nil, invalidDiagram("duplicate JSON field")
			}
			value, err := diagramJSONValue(decoder, depth+1)
			if err != nil {
				return nil, err
			}
			object[name] = value
		}
		closing, err := decoder.Token()
		if err != nil || closing != json.Delim('}') {
			return nil, invalidDiagram("malformed object")
		}
		return object, nil
	case json.Delim('['):
		array := []any{}
		for decoder.More() {
			value, err := diagramJSONValue(decoder, depth+1)
			if err != nil {
				return nil, err
			}
			array = append(array, value)
		}
		closing, err := decoder.Token()
		if err != nil || closing != json.Delim(']') {
			return nil, invalidDiagram("malformed array")
		}
		return array, nil
	case json.Delim('}'), json.Delim(']'):
		return nil, invalidDiagram("unexpected closing delimiter")
	default:
		return token, nil
	}
}

func diagramObject(value any, required, optional []string) (map[string]any, error) {
	object, ok := value.(map[string]any)
	if !ok {
		return nil, invalidDiagram("expected an object")
	}
	allowed := map[string]bool{}
	for _, key := range required {
		if _, ok := object[key]; !ok {
			return nil, invalidDiagram("missing required field: " + key)
		}
		allowed[key] = true
	}
	for _, key := range optional {
		allowed[key] = true
	}
	for key := range object {
		if !allowed[key] {
			return nil, invalidDiagram("unknown output field")
		}
	}
	return object, nil
}

func validateDiagramShape(value any, diagramType string) error {
	root, err := diagramObject(value, []string{"title", "graph_nodes", "graph_edges", "metadata", "change_summary", "assistant_message"}, nil)
	if err != nil {
		return err
	}
	if _, err := diagramObject(root["metadata"], []string{"diagram_type"}, nil); err != nil {
		return err
	}
	nodes, nodesOK := root["graph_nodes"].([]any)
	edges, edgesOK := root["graph_edges"].([]any)
	if !nodesOK || !edgesOK {
		return invalidDiagram("graph_nodes and graph_edges must be arrays")
	}
	for _, value := range nodes {
		node, err := diagramObject(value, []string{"id", "type", "data"}, nil)
		if err != nil {
			return err
		}
		required, optional := []string{"label"}, []string{"description"}
		if diagramType == "erd" {
			required = append(required, "fields")
		}
		data, err := diagramObject(node["data"], required, optional)
		if err != nil {
			return err
		}
		if diagramType == "erd" {
			fields, ok := data["fields"].([]any)
			if !ok {
				return invalidDiagram("entity fields must be an array")
			}
			for _, field := range fields {
				if _, err := diagramObject(field, []string{"name", "data_type", "primary_key", "nullable"}, nil); err != nil {
					return err
				}
			}
		}
	}
	for _, value := range edges {
		required, optional := []string{"id", "source", "target"}, []string{"label"}
		if diagramType == "erd" {
			required = append(required, "cardinality")
		}
		if diagramType == "sequence" {
			required = append(required, "label", "sequence_number", "message_type")
			optional = nil
		}
		if _, err := diagramObject(value, required, optional); err != nil {
			return err
		}
	}
	return nil
}
