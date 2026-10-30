package generation

import (
	"reflect"
)

const MaxMutationTargets = 32

type DiagramMutation struct {
	NodeIDs []string `json:"node_ids"`
	EdgeIDs []string `json:"edge_ids"`
}

func (mutation *DiagramMutation) Validate() error {
	if mutation == nil {
		return nil
	}
	count := len(mutation.NodeIDs) + len(mutation.EdgeIDs)
	if count == 0 || count > MaxMutationTargets || mutation.NodeIDs == nil || mutation.EdgeIDs == nil {
		return ErrInvalidInput
	}
	for _, ids := range [][]string{mutation.NodeIDs, mutation.EdgeIDs} {
		seen := map[string]bool{}
		for _, id := range ids {
			if !diagramIDPattern.MatchString(id) || seen[id] {
				return ErrInvalidInput
			}
			seen[id] = true
		}
	}
	return nil
}

func (mutation *DiagramMutation) ValidateBase(base *DiagramGraph) error {
	if err := mutation.Validate(); err != nil {
		return err
	}
	if mutation == nil {
		return nil
	}
	if base == nil {
		return ErrInvalidInput
	}
	nodes, edges := map[string]bool{}, map[string]bool{}
	for _, node := range base.Nodes {
		nodes[node.ID] = true
	}
	for _, edge := range base.Edges {
		edges[edge.ID] = true
	}
	for _, id := range mutation.NodeIDs {
		if !nodes[id] {
			return ErrInvalidInput
		}
	}
	for _, id := range mutation.EdgeIDs {
		if !edges[id] {
			return ErrInvalidInput
		}
	}
	return nil
}

func ApplyDiagramMutation(base DiagramGraph, mutation *DiagramMutation, nodes []DiagramNode, edges []DiagramEdge) (*DiagramGraph, error) {
	if mutation == nil || mutation.ValidateBase(&base) != nil || nodes == nil || edges == nil || len(nodes) != len(mutation.NodeIDs) || len(edges) != len(mutation.EdgeIDs) {
		return nil, invalidDiagram("patch must replace exactly the selected nodes and edges")
	}
	nodeUpdates, edgeUpdates := map[string]DiagramNode{}, map[string]DiagramEdge{}
	for _, node := range nodes {
		if _, exists := nodeUpdates[node.ID]; exists {
			return nil, invalidDiagram("duplicate node replacement")
		}
		nodeUpdates[node.ID] = node
	}
	for _, edge := range edges {
		if _, exists := edgeUpdates[edge.ID]; exists {
			return nil, invalidDiagram("duplicate edge replacement")
		}
		edgeUpdates[edge.ID] = edge
	}
	for _, id := range mutation.NodeIDs {
		if _, ok := nodeUpdates[id]; !ok {
			return nil, invalidDiagram("patch contains an unselected node or misses a target")
		}
	}
	for _, id := range mutation.EdgeIDs {
		if _, ok := edgeUpdates[id]; !ok {
			return nil, invalidDiagram("patch contains an unselected edge or misses a target")
		}
	}
	result := DiagramGraph{Nodes: append([]DiagramNode{}, base.Nodes...), Edges: append([]DiagramEdge{}, base.Edges...), Metadata: base.Metadata}
	for index, node := range result.Nodes {
		if update, ok := nodeUpdates[node.ID]; ok {
			result.Nodes[index] = update
		}
	}
	for index, edge := range result.Edges {
		if update, ok := edgeUpdates[edge.ID]; ok {
			result.Edges[index] = update
		}
	}
	if err := ValidateDiagramMutationResult(base, result, mutation); err != nil {
		return nil, err
	}
	return &result, nil
}

func ValidateDiagramMutationResult(base, result DiagramGraph, mutation *DiagramMutation) error {
	if mutation == nil || mutation.ValidateBase(&base) != nil {
		return invalidDiagram("mutation requires a valid frozen graph and targets")
	}
	if err := ValidateDiagramGraph(result, base.Metadata.DiagramType); err != nil {
		return err
	}
	if len(base.Nodes) != len(result.Nodes) || len(base.Edges) != len(result.Edges) || base.Metadata != result.Metadata {
		return invalidDiagram("mutation cannot add or remove graph elements or change metadata")
	}
	nodes, edges := map[string]bool{}, map[string]bool{}
	for _, id := range mutation.NodeIDs {
		nodes[id] = true
	}
	for _, id := range mutation.EdgeIDs {
		edges[id] = true
	}
	changed := false
	for index, before := range base.Nodes {
		after := result.Nodes[index]
		if before.ID != after.ID || before.Type != after.Type || (!nodes[before.ID] && !reflect.DeepEqual(before, after)) {
			return invalidDiagram("mutation changed node identity, type, order, or an unselected node")
		}
		changed = changed || !reflect.DeepEqual(before, after)
	}
	for index, before := range base.Edges {
		after := result.Edges[index]
		if before.ID != after.ID || before.Source != after.Source || before.Target != after.Target || before.SequenceNumber != after.SequenceNumber || (!edges[before.ID] && before != after) {
			return invalidDiagram("mutation changed edge identity, endpoints, order, or an unselected edge")
		}
		changed = changed || before != after
	}
	if !changed {
		return invalidDiagram("mutation must make at least one effective change")
	}
	return nil
}
