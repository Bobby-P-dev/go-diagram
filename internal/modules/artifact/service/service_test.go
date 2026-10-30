package service

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/Bobby-P-dev/go-diagram.git/internal/modules/artifact"
	"github.com/Bobby-P-dev/go-diagram.git/internal/modules/artifact/dto"
)

type mockArtifactRepo struct {
	createArtifactFn  func(ctx context.Context, userID, projectID string, input artifact.CreateArtifactInput, write artifact.WriteRequest) (*artifact.ArtifactDetail, error)
	getArtifactFn     func(ctx context.Context, userID, projectID, artifactID string) (*artifact.ArtifactDetail, error)
	listArtifactsFn   func(ctx context.Context, userID, projectID string, query artifact.PageQuery) (*artifact.PageResult[*artifact.Artifact], error)
	getRevisionFn     func(ctx context.Context, userID, projectID, artifactID, revisionID string) (*artifact.ArtifactRevision, error)
	listRevisionsFn   func(ctx context.Context, userID, projectID, artifactID string, query artifact.PageQuery) (*artifact.PageResult[*artifact.RevisionSummary], error)
	appendRevisionFn  func(ctx context.Context, userID, projectID, artifactID string, input artifact.AppendRevisionInput, write artifact.WriteRequest) (*artifact.ArtifactRevision, error)
	restoreRevisionFn func(ctx context.Context, userID, projectID, artifactID string, input artifact.RestoreRevisionInput, write artifact.WriteRequest) (*artifact.ArtifactRevision, error)
}

func (mock *mockArtifactRepo) CreateArtifact(ctx context.Context, userID, projectID string, input artifact.CreateArtifactInput, write artifact.WriteRequest) (*artifact.ArtifactDetail, error) {
	if mock.createArtifactFn != nil {
		return mock.createArtifactFn(ctx, userID, projectID, input, write)
	}
	return nil, errors.New("not implemented")
}

func (mock *mockArtifactRepo) GetArtifact(ctx context.Context, userID, projectID, artifactID string) (*artifact.ArtifactDetail, error) {
	if mock.getArtifactFn != nil {
		return mock.getArtifactFn(ctx, userID, projectID, artifactID)
	}
	return nil, errors.New("not implemented")
}

func (mock *mockArtifactRepo) ListArtifacts(ctx context.Context, userID, projectID string, query artifact.PageQuery) (*artifact.PageResult[*artifact.Artifact], error) {
	if mock.listArtifactsFn != nil {
		return mock.listArtifactsFn(ctx, userID, projectID, query)
	}
	return nil, errors.New("not implemented")
}

func (mock *mockArtifactRepo) GetRevision(ctx context.Context, userID, projectID, artifactID, revisionID string) (*artifact.ArtifactRevision, error) {
	if mock.getRevisionFn != nil {
		return mock.getRevisionFn(ctx, userID, projectID, artifactID, revisionID)
	}
	return nil, errors.New("not implemented")
}

func (mock *mockArtifactRepo) ListRevisions(ctx context.Context, userID, projectID, artifactID string, query artifact.PageQuery) (*artifact.PageResult[*artifact.RevisionSummary], error) {
	if mock.listRevisionsFn != nil {
		return mock.listRevisionsFn(ctx, userID, projectID, artifactID, query)
	}
	return nil, errors.New("not implemented")
}

func (mock *mockArtifactRepo) AppendRevision(ctx context.Context, userID, projectID, artifactID string, input artifact.AppendRevisionInput, write artifact.WriteRequest) (*artifact.ArtifactRevision, error) {
	if mock.appendRevisionFn != nil {
		return mock.appendRevisionFn(ctx, userID, projectID, artifactID, input, write)
	}
	return nil, errors.New("not implemented")
}

func (mock *mockArtifactRepo) RestoreRevision(ctx context.Context, userID, projectID, artifactID string, input artifact.RestoreRevisionInput, write artifact.WriteRequest) (*artifact.ArtifactRevision, error) {
	if mock.restoreRevisionFn != nil {
		return mock.restoreRevisionFn(ctx, userID, projectID, artifactID, input, write)
	}
	return nil, errors.New("not implemented")
}

const (
	testUserID    = "11111111-1111-1111-1111-111111111111"
	testProjectID = "22222222-2222-2222-2222-222222222222"
	testArtID     = "33333333-3333-3333-3333-333333333333"
	testRevID     = "44444444-4444-4444-4444-444444444444"
)

func TestService_CreateArtifact(test *testing.T) {
	repo := &mockArtifactRepo{}
	svc := NewService(repo)

	test.Run("unauthenticated", func(test *testing.T) {
		_, err := svc.CreateArtifact(context.Background(), "", testProjectID, dto.CreateArtifactRequest{Title: "ERD"}, "key-1")
		if !errors.Is(err, artifact.ErrUnauthenticated) {
			test.Fatalf("expected ErrUnauthenticated, got %v", err)
		}
	})

	test.Run("invalid project ID", func(test *testing.T) {
		_, err := svc.CreateArtifact(context.Background(), testUserID, "not-a-uuid", dto.CreateArtifactRequest{Title: "ERD"}, "key-1")
		if !errors.Is(err, artifact.ErrInvalidInput) {
			test.Fatalf("expected ErrInvalidInput, got %v", err)
		}
	})

	test.Run("invalid content format mismatch", func(test *testing.T) {
		doc := "markdown content"
		_, err := svc.CreateArtifact(context.Background(), testUserID, testProjectID, dto.CreateArtifactRequest{
			Type:            "erd",
			Title:           "Database Model",
			DocumentContent: &doc,
		}, "key-1")
		if !errors.Is(err, artifact.ErrContentInvalid) {
			test.Fatalf("expected ErrContentInvalid, got %v", err)
		}
	})

	test.Run("success", func(test *testing.T) {
		repo.createArtifactFn = func(ctx context.Context, userID, projectID string, input artifact.CreateArtifactInput, write artifact.WriteRequest) (*artifact.ArtifactDetail, error) {
			return &artifact.ArtifactDetail{
				Artifact: artifact.Artifact{
					ID:        testArtID,
					ProjectID: projectID,
					Type:      input.Type,
					Title:     input.Title,
					Slug:      input.Slug,
					Format:    "graph",
					CreatedAt: time.Now(),
					UpdatedAt: time.Now(),
				},
				CurrentRevision: &artifact.ArtifactRevision{
					ID:            testRevID,
					ArtifactID:    testArtID,
					VersionNumber: 1,
					ContentFormat: "graph",
				},
			}, nil
		}

		res, err := svc.CreateArtifact(context.Background(), testUserID, testProjectID, dto.CreateArtifactRequest{
			Type:       "erd",
			Title:      "E-Commerce ERD",
			GraphNodes: json.RawMessage(`[]`),
			GraphEdges: json.RawMessage(`[]`),
		}, "key-art-1")
		if err != nil {
			test.Fatalf("unexpected error: %v", err)
		}
		if res.Artifact.ID != testArtID || res.CurrentRevision.ID != testRevID {
			test.Fatalf("unexpected detail: %+v", res)
		}
	})
}

func TestService_AppendRevision(test *testing.T) {
	repo := &mockArtifactRepo{}
	svc := NewService(repo)

	test.Run("invalid expected revision id", func(test *testing.T) {
		_, err := svc.AppendRevision(context.Background(), testUserID, testProjectID, testArtID, dto.AppendRevisionRequest{
			ExpectedRevisionID: "bad-id",
			ChangeSummary:      "Added orders table",
		}, "key-1")
		if !errors.Is(err, artifact.ErrInvalidInput) {
			test.Fatalf("expected ErrInvalidInput, got %v", err)
		}
	})

	test.Run("success", func(test *testing.T) {
		repo.appendRevisionFn = func(ctx context.Context, userID, projectID, artifactID string, input artifact.AppendRevisionInput, write artifact.WriteRequest) (*artifact.ArtifactRevision, error) {
			return &artifact.ArtifactRevision{
				ID:            "new-rev-uuid",
				ArtifactID:    artifactID,
				VersionNumber: 2,
				ChangeSummary: input.ChangeSummary,
				ContentFormat: "graph",
			}, nil
		}

		res, err := svc.AppendRevision(context.Background(), testUserID, testProjectID, testArtID, dto.AppendRevisionRequest{
			ExpectedRevisionID: testRevID,
			ChangeSummary:      "Added orders table",
			GraphNodes:         json.RawMessage(`[{"id":"1"}]`),
			GraphEdges:         json.RawMessage(`[]`),
		}, "key-rev-2")
		if err != nil {
			test.Fatalf("unexpected error: %v", err)
		}
		if res.VersionNumber != 2 || res.ChangeSummary != "Added orders table" {
			test.Fatalf("unexpected revision: %+v", res)
		}
	})
}
