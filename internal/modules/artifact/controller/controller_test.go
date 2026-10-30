package controller

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Bobby-P-dev/go-diagram.git/internal/modules/artifact"
	"github.com/Bobby-P-dev/go-diagram.git/internal/modules/artifact/dto"
	"github.com/Bobby-P-dev/go-diagram.git/internal/modules/identity/authcontext"
	identityservice "github.com/Bobby-P-dev/go-diagram.git/internal/modules/identity/service"
)

type mockArtifactService struct {
	createArtifactFn  func(ctx context.Context, userID, projectID string, req dto.CreateArtifactRequest, key string) (*artifact.ArtifactDetail, error)
	getArtifactFn     func(ctx context.Context, userID, projectID, artifactID string) (*artifact.ArtifactDetail, error)
	listArtifactsFn   func(ctx context.Context, userID, projectID string, query artifact.PageQuery) (*artifact.PageResult[*artifact.Artifact], error)
	getRevisionFn     func(ctx context.Context, userID, projectID, artifactID, revisionID string) (*artifact.ArtifactRevision, error)
	listRevisionsFn   func(ctx context.Context, userID, projectID, artifactID string, query artifact.PageQuery) (*artifact.PageResult[*artifact.RevisionSummary], error)
	appendRevisionFn  func(ctx context.Context, userID, projectID, artifactID string, req dto.AppendRevisionRequest, key string) (*artifact.ArtifactRevision, error)
	restoreRevisionFn func(ctx context.Context, userID, projectID, artifactID string, req dto.RestoreRevisionRequest, key string) (*artifact.ArtifactRevision, error)
}

func (mock *mockArtifactService) CreateArtifact(ctx context.Context, userID, projectID string, req dto.CreateArtifactRequest, key string) (*artifact.ArtifactDetail, error) {
	if mock.createArtifactFn != nil {
		return mock.createArtifactFn(ctx, userID, projectID, req, key)
	}
	return nil, nil
}

func (mock *mockArtifactService) GetArtifact(ctx context.Context, userID, projectID, artifactID string) (*artifact.ArtifactDetail, error) {
	if mock.getArtifactFn != nil {
		return mock.getArtifactFn(ctx, userID, projectID, artifactID)
	}
	return nil, nil
}

func (mock *mockArtifactService) ListArtifacts(ctx context.Context, userID, projectID string, query artifact.PageQuery) (*artifact.PageResult[*artifact.Artifact], error) {
	if mock.listArtifactsFn != nil {
		return mock.listArtifactsFn(ctx, userID, projectID, query)
	}
	return nil, nil
}

func (mock *mockArtifactService) GetRevision(ctx context.Context, userID, projectID, artifactID, revisionID string) (*artifact.ArtifactRevision, error) {
	if mock.getRevisionFn != nil {
		return mock.getRevisionFn(ctx, userID, projectID, artifactID, revisionID)
	}
	return nil, nil
}

func (mock *mockArtifactService) ListRevisions(ctx context.Context, userID, projectID, artifactID string, query artifact.PageQuery) (*artifact.PageResult[*artifact.RevisionSummary], error) {
	if mock.listRevisionsFn != nil {
		return mock.listRevisionsFn(ctx, userID, projectID, artifactID, query)
	}
	return nil, nil
}

func (mock *mockArtifactService) AppendRevision(ctx context.Context, userID, projectID, artifactID string, req dto.AppendRevisionRequest, key string) (*artifact.ArtifactRevision, error) {
	if mock.appendRevisionFn != nil {
		return mock.appendRevisionFn(ctx, userID, projectID, artifactID, req, key)
	}
	return nil, nil
}

func (mock *mockArtifactService) RestoreRevision(ctx context.Context, userID, projectID, artifactID string, req dto.RestoreRevisionRequest, key string) (*artifact.ArtifactRevision, error) {
	if mock.restoreRevisionFn != nil {
		return mock.restoreRevisionFn(ctx, userID, projectID, artifactID, req, key)
	}
	return nil, nil
}

const (
	testUserID    = "11111111-1111-1111-1111-111111111111"
	testProjectID = "22222222-2222-2222-2222-222222222222"
	testArtID     = "33333333-3333-3333-3333-333333333333"
	testRevID     = "44444444-4444-4444-4444-444444444444"
)

func withAuth(req *http.Request) *http.Request {
	return req.WithContext(authcontext.WithUser(req.Context(), &identityservice.CredentialRecord{
		ID:       testUserID,
		Role:     "user",
		IsActive: true,
	}))
}

func TestController_CreateArtifact(test *testing.T) {
	svc := &mockArtifactService{}
	ctrl := NewArtifactController(svc)

	test.Run("unauthenticated", func(test *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/v2/projects/"+testProjectID+"/artifacts", bytes.NewBufferString(`{"title":"ERD"}`))
		req.SetPathValue("projectId", testProjectID)
		response := httptest.NewRecorder()
		ctrl.CreateArtifact(response, req)
		if response.Code != http.StatusUnauthorized {
			test.Fatalf("expected 401, got %d", response.Code)
		}
	})

	test.Run("missing idempotency key", func(test *testing.T) {
		req := withAuth(httptest.NewRequest(http.MethodPost, "/api/v2/projects/"+testProjectID+"/artifacts", bytes.NewBufferString(`{"title":"ERD"}`)))
		req.SetPathValue("projectId", testProjectID)
		response := httptest.NewRecorder()
		ctrl.CreateArtifact(response, req)
		if response.Code != http.StatusBadRequest {
			test.Fatalf("expected 400, got %d", response.Code)
		}
	})

	test.Run("success", func(test *testing.T) {
		svc.createArtifactFn = func(ctx context.Context, userID, projectID string, req dto.CreateArtifactRequest, key string) (*artifact.ArtifactDetail, error) {
			return &artifact.ArtifactDetail{
				Artifact: artifact.Artifact{
					ID:        testArtID,
					ProjectID: projectID,
					Title:     req.Title,
					Type:      req.Type,
					CreatedAt: time.Now(),
					UpdatedAt: time.Now(),
				},
				CurrentRevision: &artifact.ArtifactRevision{
					ID:            testRevID,
					ArtifactID:    testArtID,
					VersionNumber: 1,
				},
			}, nil
		}

		req := withAuth(httptest.NewRequest(http.MethodPost, "/api/v2/projects/"+testProjectID+"/artifacts", bytes.NewBufferString(`{"type":"erd","title":"Core ERD","graph_nodes":[],"graph_edges":[]}`)))
		req.SetPathValue("projectId", testProjectID)
		req.Header.Set("Idempotency-Key", "art-key-1")
		response := httptest.NewRecorder()
		ctrl.CreateArtifact(response, req)

		if response.Code != http.StatusCreated {
			test.Fatalf("expected 201, got %d", response.Code)
		}
	})
}

func TestController_AppendRevision(test *testing.T) {
	svc := &mockArtifactService{}
	ctrl := NewArtifactController(svc)

	test.Run("revision conflict", func(test *testing.T) {
		svc.appendRevisionFn = func(ctx context.Context, userID, projectID, artifactID string, req dto.AppendRevisionRequest, key string) (*artifact.ArtifactRevision, error) {
			return nil, artifact.ErrRevisionConflict
		}

		req := withAuth(httptest.NewRequest(http.MethodPost, "/api/v2/projects/"+testProjectID+"/artifacts/"+testArtID+"/revisions", bytes.NewBufferString(`{"expected_revision_id":"`+testRevID+`","change_summary":"Updated"}`)))
		req.SetPathValue("projectId", testProjectID)
		req.SetPathValue("artifactId", testArtID)
		req.Header.Set("Idempotency-Key", "rev-key-1")
		response := httptest.NewRecorder()
		ctrl.AppendRevision(response, req)

		if response.Code != http.StatusConflict {
			test.Fatalf("expected 409, got %d", response.Code)
		}
		var errResp map[string]interface{}
		_ = json.Unmarshal(response.Body.Bytes(), &errResp)
		if errResp["code"] != "REVISION_CONFLICT" {
			test.Fatalf("expected REVISION_CONFLICT code, got %v", errResp["code"])
		}
	})
}

func TestController_RestoreRevision(test *testing.T) {
	svc := &mockArtifactService{}
	ctrl := NewArtifactController(svc)

	test.Run("success", func(test *testing.T) {
		svc.restoreRevisionFn = func(ctx context.Context, userID, projectID, artifactID string, req dto.RestoreRevisionRequest, key string) (*artifact.ArtifactRevision, error) {
			return &artifact.ArtifactRevision{
				ID:            "restored-rev-id",
				ArtifactID:    artifactID,
				VersionNumber: 3,
				ChangeSummary: "Restored to version 1",
				AuthorType:    "restore",
			}, nil
		}

		req := withAuth(httptest.NewRequest(http.MethodPost, "/api/v2/projects/"+testProjectID+"/artifacts/"+testArtID+"/restores", bytes.NewBufferString(`{"target_revision_id":"target-rev-id","expected_revision_id":"`+testRevID+`"}`)))
		req.SetPathValue("projectId", testProjectID)
		req.SetPathValue("artifactId", testArtID)
		req.Header.Set("Idempotency-Key", "restore-key-1")
		response := httptest.NewRecorder()
		ctrl.RestoreRevision(response, req)

		if response.Code != http.StatusCreated {
			test.Fatalf("expected 201, got %d", response.Code)
		}
	})
}
