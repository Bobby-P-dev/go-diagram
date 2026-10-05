package controllers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Bobby-P-dev/go-diagram.git/src/dtos"
	"github.com/Bobby-P-dev/go-diagram.git/src/entities"
)

type mockShareModel struct {
	createOrGetShareFunc    func(projectID, title string) (*entities.ProjectShare, error)
	getShareByProjectIDFunc func(projectID string) (*entities.ProjectShare, error)
	getShareByTokenFunc     func(token string) (*entities.ProjectShare, *entities.Project, []entities.ChatMessage, error)
	revokeShareFunc         func(projectID string) error
	forkSharedProjectFunc   func(token, newTitle string) (*entities.Project, error)
}

func (m *mockShareModel) CreateOrGetShare(projectID, title string) (*entities.ProjectShare, error) {
	if m.createOrGetShareFunc != nil {
		return m.createOrGetShareFunc(projectID, title)
	}
	return nil, nil
}

func (m *mockShareModel) GetShareByProjectID(projectID string) (*entities.ProjectShare, error) {
	if m.getShareByProjectIDFunc != nil {
		return m.getShareByProjectIDFunc(projectID)
	}
	return nil, nil
}

func (m *mockShareModel) GetShareByToken(token string) (*entities.ProjectShare, *entities.Project, []entities.ChatMessage, error) {
	if m.getShareByTokenFunc != nil {
		return m.getShareByTokenFunc(token)
	}
	return nil, nil, nil, nil
}

func (m *mockShareModel) RevokeShare(projectID string) error {
	if m.revokeShareFunc != nil {
		return m.revokeShareFunc(projectID)
	}
	return nil
}

func (m *mockShareModel) ForkSharedProject(token, newTitle string) (*entities.Project, error) {
	if m.forkSharedProjectFunc != nil {
		return m.forkSharedProjectFunc(token, newTitle)
	}
	return nil, nil
}

func (m *mockShareModel) ForkSharedProjectWithUser(token, newTitle, userID string) (*entities.Project, error) {
	if m.forkSharedProjectFunc != nil {
		return m.forkSharedProjectFunc(token, newTitle)
	}
	return nil, nil
}

func TestShareController_MissingProjectID(t *testing.T) {
	mock := &mockShareModel{}
	controller := NewShareController(mock, nil, nil)

	// Create share without project ID
	req := httptest.NewRequest("POST", "/api/projects//share", bytes.NewBuffer([]byte("{}")))
	w := httptest.NewRecorder()
	controller.CreateShare(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected 400 Bad Request, got %d", w.Code)
	}

	// Get status without project ID
	req = httptest.NewRequest("GET", "/api/projects//share", nil)
	w = httptest.NewRecorder()
	controller.GetShareStatus(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected 400 Bad Request, got %d", w.Code)
	}

	// Revoke without project ID
	req = httptest.NewRequest("DELETE", "/api/projects//share", nil)
	w = httptest.NewRecorder()
	controller.RevokeShare(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected 400 Bad Request, got %d", w.Code)
	}
}

func TestShareController_GetSharedProject_Success(t *testing.T) {
	now := time.Now()
	mock := &mockShareModel{
		getShareByTokenFunc: func(token string) (*entities.ProjectShare, *entities.Project, []entities.ChatMessage, error) {
			share := &entities.ProjectShare{
				ID:         "share-1",
				ShareToken: token,
				ProjectID:  "proj-1",
				Title:      "Sample Shared Title",
				IsActive:   true,
				ViewCount:  10,
				CreatedAt:  now,
				UpdatedAt:  now,
			}
			proj := &entities.Project{
				ID:          "proj-1",
				Title:       "Sample Shared Title",
				DiagramType: "flowchart",
			}
			messages := []entities.ChatMessage{
				{
					ID:        "msg-1",
					ProjectID: "proj-1",
					Role:      "user",
					Content:   "Buat flow login",
					CreatedAt: now,
				},
			}
			return share, proj, messages, nil
		},
	}
	controller := NewShareController(mock, nil, nil)

	req := httptest.NewRequest("GET", "/api/shared/sample-token-123", nil)
	req.SetPathValue("token", "sample-token-123")
	w := httptest.NewRecorder()
	controller.GetSharedProject(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK, got %d. Body: %s", w.Code, w.Body.String())
	}

	var envelope struct {
		Status string                          `json:"status"`
		Data   dtos.SharedProjectDetailResponse `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("Failed to parse JSON response: %v", err)
	}

	resp := envelope.Data
	if resp.Share.ShareToken != "sample-token-123" {
		t.Errorf("Expected token sample-token-123, got %s", resp.Share.ShareToken)
	}
	if len(resp.Messages) != 1 {
		t.Errorf("Expected 1 message, got %d", len(resp.Messages))
	}
	if resp.Project.Title != "Sample Shared Title" {
		t.Errorf("Expected project title 'Sample Shared Title', got '%s'", resp.Project.Title)
	}
}
