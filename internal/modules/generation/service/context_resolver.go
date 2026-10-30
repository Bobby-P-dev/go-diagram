package service

import (
	"context"
	"regexp"
	"sort"
	"strings"

	"github.com/Bobby-P-dev/go-diagram.git/internal/modules/generation"
)

var uuidPattern = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

type ContextResolver struct{ reader generation.ContextReader }

var _ generation.ContextResolver = (*ContextResolver)(nil)

func NewContextResolver(reader generation.ContextReader) *ContextResolver {
	return &ContextResolver{reader: reader}
}

func validID(value string) bool {
	return uuidPattern.MatchString(value) && value != "00000000-0000-0000-0000-000000000000"
}

func normalize(scope *generation.Scope, input *generation.ResolveInput) error {
	scope.UserID = strings.ToLower(scope.UserID)
	scope.ProjectID = strings.ToLower(scope.ProjectID)
	scope.ThreadID = strings.ToLower(scope.ThreadID)
	if scope.UserID == "" {
		return generation.ErrUnauthenticated
	}
	if !validID(scope.UserID) || !validID(scope.ProjectID) || !validID(scope.ThreadID) {
		return generation.ErrInvalidInput
	}
	if input.Mode == "" {
		input.Mode = "explicit"
	}
	if input.Mode != "explicit" && input.Mode != "project_relevant" {
		return generation.ErrInvalidInput
	}
	if len(input.Sources) > generation.MaxSources || len(input.ThreadIDs) > generation.MaxThreads || len(input.ExcludedArtifactIDs) > generation.MaxSources {
		return generation.ErrInvalidInput
	}
	if input.Mode == "project_relevant" && len(input.Sources) != 0 {
		return generation.ErrInvalidInput
	}
	if input.MaxContentBytes == 0 {
		input.MaxContentBytes = generation.DefaultContentBytes
	}
	if input.MaxContentBytes < 1024 || input.MaxContentBytes > generation.MaxContentBytes {
		return generation.ErrInvalidInput
	}
	seen := map[string]bool{}
	for index := range input.Sources {
		ref := &input.Sources[index]
		ref.ArtifactID = strings.ToLower(ref.ArtifactID)
		ref.RevisionID = strings.ToLower(ref.RevisionID)
		if !validID(ref.ArtifactID) || (ref.RevisionID != "" && !validID(ref.RevisionID)) || seen[ref.ArtifactID] {
			return generation.ErrInvalidInput
		}
		seen[ref.ArtifactID] = true
	}
	for index, artifactID := range input.ExcludedArtifactIDs {
		artifactID = strings.ToLower(artifactID)
		if !validID(artifactID) || seen[artifactID] {
			return generation.ErrInvalidInput
		}
		seen[artifactID] = true
		input.ExcludedArtifactIDs[index] = artifactID
	}
	seen = map[string]bool{scope.ThreadID: true}
	for index, threadID := range input.ThreadIDs {
		threadID = strings.ToLower(threadID)
		if !validID(threadID) || seen[threadID] {
			return generation.ErrInvalidInput
		}
		seen[threadID] = true
		input.ThreadIDs[index] = threadID
	}
	return nil
}

func (resolver *ContextResolver) Resolve(ctx context.Context, scope generation.Scope, input generation.ResolveInput) (*generation.Manifest, error) {
	input.Sources = append([]generation.RevisionRef(nil), input.Sources...)
	input.ThreadIDs = append([]string(nil), input.ThreadIDs...)
	input.ExcludedArtifactIDs = append([]string(nil), input.ExcludedArtifactIDs...)
	if err := normalize(&scope, &input); err != nil {
		return nil, err
	}
	snapshot, err := resolver.reader.ReadSnapshot(ctx, scope, input)
	if err != nil {
		return nil, err
	}
	manifest := &generation.Manifest{
		SchemaVersion: 1, Mode: input.Mode, Project: snapshot.Project, ThreadID: scope.ThreadID,
		Sources: []generation.Source{}, Messages: []generation.Message{}, Omitted: []generation.Omission{},
		Budget:                  generation.Budget{MaxContentBytes: input.MaxContentBytes, UsedContentBytes: len(snapshot.Project.Title) + len(snapshot.Project.Description)},
		CandidateLimitReached:   snapshot.CandidateLimitReached,
		HistoryLimitedThreadIDs: append([]string{}, snapshot.HistoryLimitedThreadIDs...), PreviewOnly: true,
	}
	if manifest.Budget.UsedContentBytes > manifest.Budget.MaxContentBytes {
		return nil, generation.ErrBudgetExceeded
	}
	for _, source := range snapshot.Sources {
		if manifest.Budget.UsedContentBytes+source.ContentBytes > manifest.Budget.MaxContentBytes {
			manifest.Omitted = append(manifest.Omitted, generation.Omission{ArtifactID: source.ArtifactID, RevisionID: source.RevisionID, Reason: "content_budget"})
			continue
		}
		manifest.Sources = append(manifest.Sources, source)
		manifest.Budget.UsedContentBytes += source.ContentBytes
	}
	for _, message := range snapshot.Messages {
		if manifest.Budget.UsedContentBytes+message.ContentBytes > manifest.Budget.MaxContentBytes {
			manifest.Omitted = append(manifest.Omitted, generation.Omission{MessageID: message.ID, ThreadID: message.ThreadID, Reason: "content_budget"})
			continue
		}
		manifest.Messages = append(manifest.Messages, message)
		manifest.Budget.UsedContentBytes += message.ContentBytes
	}
	sort.SliceStable(manifest.Messages, func(left, right int) bool {
		return manifest.Messages[left].SequenceNumber < manifest.Messages[right].SequenceNumber
	})
	return manifest, nil
}
