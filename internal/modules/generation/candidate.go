package generation

import "strings"

type ArtifactCandidate struct {
	Type             string
	Title            string
	DocumentContent  string
	Graph            *DiagramGraph
	ChangeSummary    string
	AssistantMessage string
	Format           string
	SchemaVersion    int
	Sources          []RevisionRef
}

func OutputFormat(outputType string) (string, error) {
	switch outputType {
	case "brainstorm", "prd":
		return "markdown", nil
	case "erd", "flowchart", "sequence":
		return "graph", nil
	default:
		return "", ErrUnsupportedType
	}
}

func (candidate *ArtifactCandidate) Validate() error {
	if candidate == nil {
		return ErrInvalidInput
	}
	format, err := OutputFormat(candidate.Type)
	if err != nil {
		return err
	}
	if candidate.Format != format || candidate.SchemaVersion != 1 || !diagramText(candidate.Title, 255, 1024, true) || !diagramText(candidate.ChangeSummary, 2048, 2048, true) || !diagramText(candidate.AssistantMessage, 4096, 4096, true) {
		return ErrInvalidInput
	}
	if format == "markdown" {
		if candidate.Graph != nil || !diagramText(candidate.DocumentContent, 128*1024, 128*1024, true) || strings.ContainsRune(candidate.DocumentContent, '\x00') {
			return ErrInvalidDocument
		}
		return nil
	}
	if candidate.DocumentContent != "" || candidate.Graph == nil {
		return ErrInvalidDiagram
	}
	return ValidateDiagramGraph(*candidate.Graph, candidate.Type)
}
