package generation

import (
	"bytes"
	"encoding/json"
)

type DiagramSnapshot struct {
	Nodes    []json.RawMessage `json:"nodes"`
	Edges    []json.RawMessage `json:"edges"`
	Metadata json.RawMessage   `json:"metadata"`
}

func PreserveDiagramMutation(snapshot json.RawMessage, result DiagramGraph, mutation *DiagramMutation) (*DiagramSnapshot, error) {
	var base DiagramGraph
	var raw DiagramSnapshot
	if len(snapshot) > MaxDiagramBytes || json.Unmarshal(snapshot, &base) != nil || json.Unmarshal(snapshot, &raw) != nil {
		return nil, ErrInvalidInput
	}
	if err := ValidateDiagramGraph(base, result.Metadata.DiagramType); err != nil {
		return nil, err
	}
	if err := ValidateDiagramMutationResult(base, result, mutation); err != nil {
		return nil, err
	}
	for index, node := range base.Nodes {
		before, _ := json.Marshal(node)
		after, _ := json.Marshal(result.Nodes[index])
		raw.Nodes[index] = overlayKnown(raw.Nodes[index], before, after)
	}
	for index, edge := range base.Edges {
		before, _ := json.Marshal(edge)
		after, _ := json.Marshal(result.Edges[index])
		raw.Edges[index] = overlayKnown(raw.Edges[index], before, after)
	}
	encoded, err := json.Marshal(raw)
	if err != nil || len(encoded) > MaxDiagramBytes {
		return nil, ErrDiagramTooLarge
	}
	return &raw, nil
}

func overlayKnown(raw, before, after json.RawMessage) json.RawMessage {
	if bytes.Equal(before, after) {
		return raw
	}
	if len(after) > 0 && after[0] == '{' {
		original, oldFields, newFields := map[string]json.RawMessage{}, map[string]json.RawMessage{}, map[string]json.RawMessage{}
		_ = json.Unmarshal(raw, &original)
		_ = json.Unmarshal(before, &oldFields)
		_ = json.Unmarshal(after, &newFields)
		for key := range oldFields {
			if _, exists := newFields[key]; !exists {
				delete(original, key)
			}
		}
		for key, value := range newFields {
			original[key] = overlayKnown(original[key], oldFields[key], value)
		}
		encoded, _ := json.Marshal(original)
		return encoded
	}
	if len(after) > 0 && after[0] == '[' {
		var original, oldFields, newFields []json.RawMessage
		_ = json.Unmarshal(raw, &original)
		_ = json.Unmarshal(before, &oldFields)
		_ = json.Unmarshal(after, &newFields)
		byName := func(fields []json.RawMessage) map[string]json.RawMessage {
			values := map[string]json.RawMessage{}
			for _, field := range fields {
				var named struct {
					Name string `json:"name"`
				}
				_ = json.Unmarshal(field, &named)
				values[named.Name] = field
			}
			return values
		}
		rawByName, oldByName := byName(original), byName(oldFields)
		for index, field := range newFields {
			var named struct {
				Name string `json:"name"`
			}
			_ = json.Unmarshal(field, &named)
			newFields[index] = overlayKnown(rawByName[named.Name], oldByName[named.Name], field)
		}
		encoded, _ := json.Marshal(newFields)
		return encoded
	}
	return after
}
