package generation

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

func mutationBase(kind string) DiagramGraph {
	nodeType := "process"
	if kind == "erd" {
		nodeType = "entity"
	}
	if kind == "sequence" {
		nodeType = "participant"
	}
	graph := DiagramGraph{Nodes: []DiagramNode{{ID: "First", Type: nodeType, Data: DiagramNodeData{Label: "First"}}, {ID: "other", Type: nodeType, Data: DiagramNodeData{Label: "Other"}}}, Edges: []DiagramEdge{{ID: "link", Source: "First", Target: "other", Label: "Action"}}, Metadata: DiagramMetadata{DiagramType: kind}}
	if kind == "erd" {
		for index := range graph.Nodes {
			graph.Nodes[index].Data.Fields = []DiagramField{{Name: "id", DataType: "uuid", PrimaryKey: true}}
		}
		graph.Edges[0].Cardinality = "1:N"
	}
	if kind == "sequence" {
		graph.Edges[0].SequenceNumber = 1
		graph.Edges[0].MessageType = "call"
	}
	return graph
}

func TestDiagramMutationScope(test *testing.T) {
	for _, kind := range []string{"erd", "flowchart", "sequence"} {
		test.Run(kind, func(test *testing.T) {
			base := mutationBase(kind)
			mutation := &DiagramMutation{NodeIDs: []string{"First"}, EdgeIDs: []string{}}
			updated := base.Nodes[0]
			updated.Data.Label = "Changed"
			result, err := ApplyDiagramMutation(base, mutation, []DiagramNode{updated}, []DiagramEdge{})
			if err != nil {
				test.Fatal(err)
			}
			if base.Nodes[0].Data.Label != "First" || result.Nodes[0].Data.Label != "Changed" || !reflect.DeepEqual(base.Nodes[1], result.Nodes[1]) || !reflect.DeepEqual(base.Edges, result.Edges) {
				test.Fatal("scope or base changed")
			}
			for _, scenario := range []struct {
				name   string
				modify func(*DiagramGraph)
			}{
				{"unselected node", func(graph *DiagramGraph) { graph.Nodes[1].Data.Label = "Unauthorized" }},
				{"unselected edge", func(graph *DiagramGraph) { graph.Edges[0].Label = "Unauthorized" }},
				{"node id", func(graph *DiagramGraph) { graph.Nodes[0].ID = "renamed" }},
				{"node type", func(graph *DiagramGraph) { graph.Nodes[0].Type = "decision" }},
				{"add", func(graph *DiagramGraph) {
					graph.Nodes = append(graph.Nodes, DiagramNode{ID: "new", Type: base.Nodes[0].Type, Data: base.Nodes[0].Data})
				}},
				{"remove", func(graph *DiagramGraph) { graph.Nodes = graph.Nodes[:1] }},
				{"reorder", func(graph *DiagramGraph) { graph.Nodes[0], graph.Nodes[1] = graph.Nodes[1], graph.Nodes[0] }},
			} {
				test.Run(scenario.name, func(test *testing.T) {
					copyGraph := *result
					copyGraph.Nodes = append([]DiagramNode{}, result.Nodes...)
					copyGraph.Edges = append([]DiagramEdge{}, result.Edges...)
					scenario.modify(&copyGraph)
					if ValidateDiagramMutationResult(base, copyGraph, mutation) == nil {
						test.Fatal("accepted outside scope")
					}
				})
			}
			if _, err := ApplyDiagramMutation(base, mutation, base.Nodes, []DiagramEdge{}); err == nil {
				test.Fatal("accepted full graph as patch")
			}
			if _, err := ApplyDiagramMutation(base, mutation, []DiagramNode{base.Nodes[0]}, []DiagramEdge{}); err == nil {
				test.Fatal("accepted no-op")
			}
			edgeMutation := &DiagramMutation{NodeIDs: []string{}, EdgeIDs: []string{"link"}}
			edge := base.Edges[0]
			edge.Label = "Updated message"
			if _, err := ApplyDiagramMutation(base, edgeMutation, []DiagramNode{}, []DiagramEdge{edge}); err != nil {
				test.Fatal(err)
			}
			edge.Source = "other"
			if _, err := ApplyDiagramMutation(base, edgeMutation, []DiagramNode{}, []DiagramEdge{edge}); err == nil {
				test.Fatal("endpoint change accepted")
			}
			if kind == "sequence" {
				edge = base.Edges[0]
				edge.SequenceNumber = 2
				if _, err := ApplyDiagramMutation(base, edgeMutation, []DiagramNode{}, []DiagramEdge{edge}); err == nil {
					test.Fatal("sequence reorder accepted")
				}
			}
		})
	}
}

func TestDiagramMutationTargets(test *testing.T) {
	base := mutationBase("flowchart")
	for _, mutation := range []*DiagramMutation{
		{}, {NodeIDs: []string{}, EdgeIDs: []string{}}, {NodeIDs: []string{"First"}},
		{NodeIDs: []string{"first"}, EdgeIDs: []string{}}, {NodeIDs: []string{"First", "First"}, EdgeIDs: []string{}},
		{NodeIDs: []string{}, EdgeIDs: []string{"missing"}}, {NodeIDs: []string{"bad/id"}, EdgeIDs: []string{}},
		{NodeIDs: strings.Split(strings.Repeat("First,", 32)+"First", ","), EdgeIDs: []string{}},
	} {
		if mutation.ValidateBase(&base) == nil {
			test.Fatalf("accepted invalid targets %+v", mutation)
		}
	}
}

func TestDiagramMutationPreservesRawSnapshot(test *testing.T) {
	raw := json.RawMessage(`{"nodes":[{"id":"First","type":"entity","position":{"x":12.75,"y":90},"custom":{"huge":900719925474099312345},"data":{"label":"First","description":"Delete me","extra":"keep","fields":[{"name":"id","data_type":"uuid","primary_key":true,"nullable":false,"extension":"keep field"}]}},{"id":"other","type":"entity","data":{"label":"Other","fields":[{"name":"id","data_type":"uuid","primary_key":true,"nullable":false}]},"position":{"x":200,"y":0}}],"edges":[{"id":"link","source":"First","target":"other","label":"Action","cardinality":"1:N","custom":true}],"metadata":{"diagram_type":"erd","custom":"keep meta"}}`)
	var base DiagramGraph
	if err := json.Unmarshal(raw, &base); err != nil {
		test.Fatal(err)
	}
	changed := base.Nodes[0]
	changed.Data.Label = "Renamed"
	changed.Data.Description = ""
	changed.Data.Fields = append([]DiagramField{}, changed.Data.Fields...)
	changed.Data.Fields[0].DataType = "text"
	changed.Data.Fields = append(changed.Data.Fields, DiagramField{Name: "email", DataType: "varchar", Nullable: true})
	mutation := &DiagramMutation{NodeIDs: []string{"First"}, EdgeIDs: []string{}}
	result, err := ApplyDiagramMutation(base, mutation, []DiagramNode{changed}, []DiagramEdge{})
	if err != nil {
		test.Fatal(err)
	}
	preserved, err := PreserveDiagramMutation(raw, *result, mutation)
	if err != nil {
		test.Fatal(err)
	}
	encoded, _ := json.Marshal(preserved)
	for _, text := range []string{"900719925474099312345", `"x":12.75`, `"extension":"keep field"`, `"extra":"keep"`, `"custom":"keep meta"`, `"name":"email"`, `"data_type":"text"`} {
		if !strings.Contains(string(encoded), text) {
			test.Fatalf("lost %s: %s", text, encoded)
		}
	}
	if strings.Contains(string(encoded), "Delete me") {
		test.Fatal("description removal was not applied")
	}
	var original DiagramSnapshot
	_ = json.Unmarshal(raw, &original)
	if string(original.Nodes[1]) != string(preserved.Nodes[1]) || string(original.Edges[0]) != string(preserved.Edges[0]) || string(original.Metadata) != string(preserved.Metadata) {
		test.Fatal("unselected raw content changed")
	}
	run := Run{Prompt: "PRIVATE", BaseGraphRaw: raw, BaseGraph: &base, Mutation: mutation}
	serialized, _ := json.Marshal(run)
	if strings.Contains(string(serialized), "PRIVATE") || strings.Contains(string(serialized), "900719925474099312345") {
		test.Fatal("private snapshot exposed")
	}
}
