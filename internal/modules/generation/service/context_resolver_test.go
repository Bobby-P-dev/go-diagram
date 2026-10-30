package service

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/Bobby-P-dev/go-diagram.git/internal/modules/generation"
)

const (
	userID     = "11111111-1111-4111-8111-111111111111"
	projectID  = "22222222-2222-4222-8222-222222222222"
	threadID   = "33333333-3333-4333-8333-333333333333"
	artifactID = "44444444-4444-4444-8444-444444444444"
	revisionID = "55555555-5555-4555-8555-555555555555"
)

type fakeReader struct {
	snapshot generation.Snapshot
	err      error
	calls    int
	input    generation.ResolveInput
	scope    generation.Scope
}

func (reader *fakeReader) ReadSnapshot(_ context.Context, scope generation.Scope, input generation.ResolveInput) (*generation.Snapshot, error) {
	reader.calls++
	reader.input, reader.scope = input, scope
	return &reader.snapshot, reader.err
}

func validScope() generation.Scope {
	return generation.Scope{UserID: userID, ProjectID: projectID, ThreadID: threadID}
}

func TestResolveValidationBeforeStorage(test *testing.T) {
	cases := []struct {
		name  string
		input generation.ResolveInput
	}{
		{"mode", generation.ResolveInput{Mode: "global"}},
		{"low budget", generation.ResolveInput{MaxContentBytes: 10}},
		{"high budget", generation.ResolveInput{MaxContentBytes: generation.MaxContentBytes + 1}},
		{"invalid source", generation.ResolveInput{Sources: []generation.RevisionRef{{ArtifactID: "bad"}}}},
		{"invalid revision", generation.ResolveInput{Sources: []generation.RevisionRef{{ArtifactID: artifactID, RevisionID: "bad"}}}},
		{"duplicate source", generation.ResolveInput{Sources: []generation.RevisionRef{{ArtifactID: artifactID}, {ArtifactID: artifactID}}}},
		{"excluded explicit source", generation.ResolveInput{Sources: []generation.RevisionRef{{ArtifactID: artifactID}}, ExcludedArtifactIDs: []string{artifactID}}},
		{"ambiguous automatic sources", generation.ResolveInput{Mode: "project_relevant", Sources: []generation.RevisionRef{{ArtifactID: artifactID}}}},
		{"duplicate current thread", generation.ResolveInput{ThreadIDs: []string{threadID}}},
		{"invalid thread", generation.ResolveInput{ThreadIDs: []string{"bad"}}},
		{"too many sources", generation.ResolveInput{Sources: make([]generation.RevisionRef, generation.MaxSources+1)}},
		{"too many threads", generation.ResolveInput{ThreadIDs: make([]string, generation.MaxThreads+1)}},
	}
	for _, item := range cases {
		test.Run(item.name, func(test *testing.T) {
			reader := &fakeReader{}
			_, err := NewContextResolver(reader).Resolve(context.Background(), validScope(), item.input)
			if !errors.Is(err, generation.ErrInvalidInput) || reader.calls != 0 {
				test.Fatalf("got error=%v calls=%d", err, reader.calls)
			}
		})
	}
	reader := &fakeReader{}
	_, err := NewContextResolver(reader).Resolve(context.Background(), generation.Scope{}, generation.ResolveInput{})
	if !errors.Is(err, generation.ErrUnauthenticated) || reader.calls != 0 {
		test.Fatalf("got %v", err)
	}
	scope := validScope()
	scope.ProjectID = "00000000-0000-0000-0000-000000000000"
	_, err = NewContextResolver(reader).Resolve(context.Background(), scope, generation.ResolveInput{})
	if !errors.Is(err, generation.ErrInvalidInput) || reader.calls != 0 {
		test.Fatalf("got %v", err)
	}
}

func TestResolveManifestBudgetAndPinnedRevisions(test *testing.T) {
	reader := &fakeReader{snapshot: generation.Snapshot{
		Project: generation.ProjectContext{ID: projectID, Title: "Example"},
		Sources: []generation.Source{
			{ArtifactID: artifactID, RevisionID: revisionID, VersionNumber: 3, Format: "markdown", Content: json.RawMessage(`{"markdown":"PRD"}`), ContentBytes: 18},
			{ArtifactID: "large", RevisionID: "large-rev", ContentBytes: 1024},
		},
		Messages: []generation.Message{
			{ID: "newest", ThreadID: threadID, Content: "new", SequenceNumber: 4, ContentBytes: 3},
			{ID: "oversized", ThreadID: threadID, SequenceNumber: 3, ContentBytes: 2000},
			{ID: "oldest", ThreadID: threadID, Content: "old", SequenceNumber: 2, ContentBytes: 3},
		},
		CandidateLimitReached: true, HistoryLimitedThreadIDs: []string{threadID},
	}}
	manifest, err := NewContextResolver(reader).Resolve(context.Background(), validScope(), generation.ResolveInput{MaxContentBytes: 1024})
	if err != nil {
		test.Fatal(err)
	}
	if manifest.Budget.UsedContentBytes != 31 || len(manifest.Sources) != 1 || len(manifest.Messages) != 2 || len(manifest.Omitted) != 2 {
		test.Fatalf("unexpected manifest: %+v", manifest)
	}
	if manifest.Sources[0].RevisionID != revisionID || manifest.Sources[0].VersionNumber != 3 {
		test.Fatal("revision reference lost")
	}
	if manifest.Messages[0].ID != "oldest" || manifest.Messages[1].ID != "newest" {
		test.Fatal("messages not chronological")
	}
	if manifest.Omitted[0].RevisionID != "large-rev" || manifest.Omitted[1].MessageID != "oversized" {
		test.Fatal("missing omission refs")
	}
	if !manifest.PreviewOnly || !manifest.CandidateLimitReached || len(manifest.HistoryLimitedThreadIDs) != 1 {
		test.Fatal("preview limitations missing")
	}
	if reader.input.Mode != "explicit" {
		test.Fatal("wrong default mode")
	}
}

func TestResolveDefaultsAndDoesNotMutateCaller(test *testing.T) {
	upper := "AAAAAAAA-AAAA-4AAA-8AAA-AAAAAAAAAAAA"
	input := generation.ResolveInput{Sources: []generation.RevisionRef{{ArtifactID: upper}}}
	want := generation.ResolveInput{Sources: []generation.RevisionRef{{ArtifactID: upper}}}
	reader := &fakeReader{}
	manifest, err := NewContextResolver(reader).Resolve(context.Background(), validScope(), input)
	if err != nil {
		test.Fatal(err)
	}
	if !reflect.DeepEqual(input, want) {
		test.Fatal("caller input mutated")
	}
	if reader.input.Sources[0].ArtifactID != strings.ToLower(upper) || reader.input.MaxContentBytes != generation.DefaultContentBytes {
		test.Fatal("defaults or normalization missing")
	}
	encoded, err := json.Marshal(manifest)
	if err != nil || strings.Contains(string(encoded), `"sources":null`) || strings.Contains(string(encoded), `"messages":null`) {
		test.Fatalf("invalid empty manifest: %s %v", encoded, err)
	}
}

func TestResolveRejectsOversizedProjectAndPropagatesErrors(test *testing.T) {
	reader := &fakeReader{snapshot: generation.Snapshot{Project: generation.ProjectContext{Description: strings.Repeat("x", 1025)}}}
	_, err := NewContextResolver(reader).Resolve(context.Background(), validScope(), generation.ResolveInput{MaxContentBytes: 1024})
	if !errors.Is(err, generation.ErrBudgetExceeded) {
		test.Fatalf("got %v", err)
	}
	reader.err = generation.ErrNotFound
	_, err = NewContextResolver(reader).Resolve(context.Background(), validScope(), generation.ResolveInput{})
	if !errors.Is(err, generation.ErrNotFound) {
		test.Fatalf("got %v", err)
	}
}
