package service

import (
	"encoding/json"
	"strings"

	"github.com/Bobby-P-dev/go-diagram.git/internal/modules/generation"
)

func validateEngineManifest(manifest *generation.Manifest) error {
	if manifest == nil || manifest.SchemaVersion != 1 || !validID(strings.ToLower(manifest.Project.ID)) || !validID(strings.ToLower(manifest.ThreadID)) {
		return generation.ErrInvalidInput
	}
	if len(manifest.Sources) > generation.CandidateLimit || len(manifest.Messages) > (generation.MaxThreads+1)*generation.MessagesPerThread {
		return generation.ErrInvalidInput
	}
	seen := map[string]bool{}
	for _, source := range manifest.Sources {
		artifactID := strings.ToLower(source.ArtifactID)
		if !validID(artifactID) || !validID(strings.ToLower(source.RevisionID)) || source.VersionNumber < 1 || seen[artifactID] || !json.Valid(source.Content) {
			return generation.ErrInvalidInput
		}
		seen[artifactID] = true
	}
	for _, message := range manifest.Messages {
		if message.Role != "user" && message.Role != "assistant" {
			return generation.ErrInvalidInput
		}
	}
	return nil
}
