package generation

import (
	"context"
	"errors"
)

var (
	ErrInvalidDiagram         = errors.New("invalid structured diagram output")
	ErrDiagramRepairFailed    = errors.New("failed to generate valid structured diagram output")
	ErrDiagramTooLarge        = errors.New("diagram output exceeds size limit")
	ErrUnsupportedDiagramType = errors.New("unsupported diagram type")
)

type DiagramInput struct {
	Type      string           `json:"type"`
	Title     string           `json:"title"`
	Prompt    string           `json:"prompt"`
	BaseGraph *DiagramGraph    `json:"base_graph,omitempty"`
	Manifest  *Manifest        `json:"manifest"`
	Mutation  *DiagramMutation `json:"mutation,omitempty"`
}

type DiagramGraph struct {
	Nodes    []DiagramNode   `json:"nodes"`
	Edges    []DiagramEdge   `json:"edges"`
	Metadata DiagramMetadata `json:"metadata"`
}

type DiagramMetadata struct {
	DiagramType string `json:"diagram_type"`
}

type DiagramNode struct {
	ID   string          `json:"id"`
	Type string          `json:"type"`
	Data DiagramNodeData `json:"data"`
}

type DiagramNodeData struct {
	Label       string         `json:"label"`
	Description string         `json:"description,omitempty"`
	Fields      []DiagramField `json:"fields,omitempty"`
}

type DiagramField struct {
	Name       string `json:"name"`
	DataType   string `json:"data_type"`
	PrimaryKey bool   `json:"primary_key"`
	Nullable   bool   `json:"nullable"`
}

type DiagramEdge struct {
	ID             string `json:"id"`
	Source         string `json:"source"`
	Target         string `json:"target"`
	Label          string `json:"label,omitempty"`
	Cardinality    string `json:"cardinality,omitempty"`
	SequenceNumber int    `json:"sequence_number,omitempty"`
	MessageType    string `json:"message_type,omitempty"`
}

type DiagramCandidate struct {
	Type             string          `json:"type"`
	Title            string          `json:"title"`
	GraphNodes       []DiagramNode   `json:"graph_nodes"`
	GraphEdges       []DiagramEdge   `json:"graph_edges"`
	Metadata         DiagramMetadata `json:"metadata"`
	ChangeSummary    string          `json:"change_summary"`
	AssistantMessage string          `json:"assistant_message"`
	Format           string          `json:"format"`
	SchemaVersion    int             `json:"schema_version"`
	Sources          []RevisionRef   `json:"sources"`
}

type DiagramEngine interface {
	GenerateDiagram(context.Context, DiagramInput, ProgressSink) (*DiagramCandidate, error)
}
