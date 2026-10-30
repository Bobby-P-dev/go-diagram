package service

import (
	"context"

	"github.com/Bobby-P-dev/go-diagram.git/internal/modules/generation"
)

func (coordinator *RunCoordinator) generate(ctx context.Context, run *generation.Run) (*generation.ArtifactCandidate, error) {
	format, err := generation.OutputFormat(run.OutputType)
	if err != nil {
		return nil, err
	}
	var result *generation.ArtifactCandidate
	if format == "graph" {
		if coordinator.diagramEngine == nil {
			return nil, generation.ErrEngineUnavailable
		}
		candidate, err := coordinator.diagramEngine.GenerateDiagram(ctx, generation.DiagramInput{
			Type: run.OutputType, Title: run.OutputTitle, Prompt: run.Prompt, BaseGraph: run.BaseGraph, Manifest: run.Manifest, Mutation: run.Mutation,
		}, nil)
		if err != nil {
			return nil, err
		}
		if candidate == nil {
			return nil, generation.ErrInvalidDiagram
		}
		result = &generation.ArtifactCandidate{
			Type: candidate.Type, Title: candidate.Title, Graph: &generation.DiagramGraph{Nodes: candidate.GraphNodes, Edges: candidate.GraphEdges, Metadata: candidate.Metadata},
			ChangeSummary: candidate.ChangeSummary, AssistantMessage: candidate.AssistantMessage, Format: candidate.Format, SchemaVersion: candidate.SchemaVersion, Sources: candidate.Sources,
		}
	} else {
		if coordinator.engine == nil {
			return nil, generation.ErrEngineUnavailable
		}
		candidate, err := coordinator.engine.GenerateDocument(ctx, generation.DocumentInput{
			Type: run.OutputType, Title: run.OutputTitle, Prompt: run.Prompt, BaseDocument: run.BaseDocument, Manifest: run.Manifest,
		}, nil)
		if err != nil {
			return nil, err
		}
		if candidate == nil {
			return nil, generation.ErrInvalidDocument
		}
		result = &generation.ArtifactCandidate{
			Type: candidate.Type, Title: candidate.Title, DocumentContent: candidate.DocumentContent,
			ChangeSummary: candidate.ChangeSummary, AssistantMessage: candidate.AssistantMessage, Format: candidate.Format, SchemaVersion: candidate.SchemaVersion, Sources: candidate.Sources,
		}
	}
	if result.Type != run.OutputType || (run.OutputTitle != "" && result.Title != run.OutputTitle) {
		return nil, generation.ErrInvalidInput
	}
	if run.Mutation != nil {
		if run.BaseGraph == nil || result.Graph == nil {
			return nil, generation.ErrInvalidInput
		}
		if err := generation.ValidateDiagramMutationResult(*run.BaseGraph, *result.Graph, run.Mutation); err != nil {
			return nil, err
		}
	}
	return result, result.Validate()
}
