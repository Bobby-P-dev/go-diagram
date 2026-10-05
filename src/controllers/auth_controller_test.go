package controllers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Bobby-P-dev/go-diagram.git/src/dtos"
	"github.com/Bobby-P-dev/go-diagram.git/src/entities"
	"github.com/Bobby-P-dev/go-diagram.git/src/middlewares"
)

type mockCredentialModel struct {
	findByKeyFunc         func(key string) (*entities.AccessCredential, error)
	findByIDFunc          func(id string) (*entities.AccessCredential, error)
	createFunc            func(name, role string, expiresAt *time.Time, canDiagram, canUI bool) (*entities.AccessCredential, error)
	updateFunc            func(id string, name string, isActive *bool, expiresAt *time.Time) (*entities.AccessCredential, error)
	updateFullFunc        func(id string, name string, role string, isActive *bool, expiresAt *time.Time, updateExpiresAt bool, canDiagram *bool, canUI *bool) (*entities.AccessCredential, error)
	deleteFunc            func(id string) error
	getAllWithStatsFunc   func() ([]dtos.CredentialSummaryDTO, error)
	updateLastLoginFunc   func(id string) error
	ensureMasterAdminFunc func(defaultKey string) error
}

func (m *mockCredentialModel) FindByKey(key string) (*entities.AccessCredential, error) {
	if m.findByKeyFunc != nil {
		return m.findByKeyFunc(key)
	}
	return nil, fmt.Errorf("not found")
}

func (m *mockCredentialModel) FindByID(id string) (*entities.AccessCredential, error) {
	if m.findByIDFunc != nil {
		return m.findByIDFunc(id)
	}
	return nil, fmt.Errorf("not found")
}

func (m *mockCredentialModel) Create(name, role string, expiresAt *time.Time, canDiagram, canUI bool) (*entities.AccessCredential, error) {
	if m.createFunc != nil {
		return m.createFunc(name, role, expiresAt, canDiagram, canUI)
	}
	return nil, nil
}

func (m *mockCredentialModel) Update(id string, name string, isActive *bool, expiresAt *time.Time) (*entities.AccessCredential, error) {
	if m.updateFunc != nil {
		return m.updateFunc(id, name, isActive, expiresAt)
	}
	return nil, nil
}

func (m *mockCredentialModel) UpdateFull(id string, name string, role string, isActive *bool, expiresAt *time.Time, updateExpiresAt bool, canDiagram *bool, canUI *bool) (*entities.AccessCredential, error) {
	if m.updateFullFunc != nil {
		return m.updateFullFunc(id, name, role, isActive, expiresAt, updateExpiresAt, canDiagram, canUI)
	}
	if m.updateFunc != nil {
		return m.updateFunc(id, name, isActive, expiresAt)
	}
	return nil, nil
}

func (m *mockCredentialModel) Delete(id string) error {
	if m.deleteFunc != nil {
		return m.deleteFunc(id)
	}
	return nil
}

func (m *mockCredentialModel) GetAllWithStats() ([]dtos.CredentialSummaryDTO, error) {
	if m.getAllWithStatsFunc != nil {
		return m.getAllWithStatsFunc()
	}
	return nil, nil
}

func (m *mockCredentialModel) UpdateLastLogin(id string) error {
	if m.updateLastLoginFunc != nil {
		return m.updateLastLoginFunc(id)
	}
	return nil
}

func (m *mockCredentialModel) EnsureMasterAdmin(defaultKey string) error {
	if m.ensureMasterAdminFunc != nil {
		return m.ensureMasterAdminFunc(defaultKey)
	}
	return nil
}

func TestAuthController_Verify_Validation(t *testing.T) {
	mock := &mockCredentialModel{}
	controller := NewAuthController(mock)

	// Missing key
	req := httptest.NewRequest("POST", "/api/auth/verify", bytes.NewBuffer([]byte(`{"credential_key": ""}`)))
	w := httptest.NewRecorder()
	controller.Verify(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected 400 Bad Request, got %d", w.Code)
	}

	// Invalid key
	req = httptest.NewRequest("POST", "/api/auth/verify", bytes.NewBuffer([]byte(`{"credential_key": "NON-EXISTENT"}`)))
	w = httptest.NewRecorder()
	controller.Verify(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("Expected 401 Unauthorized, got %d", w.Code)
	}
}

func TestAuthController_Verify_Disabled(t *testing.T) {
	mock := &mockCredentialModel{
		findByKeyFunc: func(key string) (*entities.AccessCredential, error) {
			return &entities.AccessCredential{
				ID:            "user-1",
				CredentialKey: key,
				Name:          "Disabled User",
				Role:          "user",
				IsActive:      false,
			}, nil
		},
	}
	controller := NewAuthController(mock)

	req := httptest.NewRequest("POST", "/api/auth/verify", bytes.NewBuffer([]byte(`{"credential_key": "RL-DISABLED"}`)))
	w := httptest.NewRecorder()
	controller.Verify(w, req)
	if w.Code != http.StatusForbidden {
		t.Errorf("Expected 403 Forbidden for disabled user, got %d", w.Code)
	}
}

func TestAuthController_Verify_Expired(t *testing.T) {
	yesterday := time.Now().Add(-24 * time.Hour)
	mock := &mockCredentialModel{
		findByKeyFunc: func(key string) (*entities.AccessCredential, error) {
			return &entities.AccessCredential{
				ID:            "user-2",
				CredentialKey: key,
				Name:          "Expired User",
				Role:          "user",
				IsActive:      true,
				ExpiresAt:     &yesterday,
			}, nil
		},
	}
	controller := NewAuthController(mock)

	req := httptest.NewRequest("POST", "/api/auth/verify", bytes.NewBuffer([]byte(`{"credential_key": "RL-EXPIRED"}`)))
	w := httptest.NewRecorder()
	controller.Verify(w, req)
	if w.Code != http.StatusForbidden {
		t.Errorf("Expected 403 Forbidden for expired credential, got %d", w.Code)
	}
}

func TestAuthController_Verify_Success(t *testing.T) {
	tomorrow := time.Now().Add(24 * time.Hour)
	mock := &mockCredentialModel{
		findByKeyFunc: func(key string) (*entities.AccessCredential, error) {
			return &entities.AccessCredential{
				ID:            "user-3",
				CredentialKey: key,
				Name:          "Valid Client",
				Role:          "user",
				IsActive:      true,
				ExpiresAt:     &tomorrow,
			}, nil
		},
		updateLastLoginFunc: func(id string) error {
			return nil
		},
	}
	controller := NewAuthController(mock)

	req := httptest.NewRequest("POST", "/api/auth/verify", bytes.NewBuffer([]byte(`{"credential_key": "RL-VALID-1234"}`)))
	w := httptest.NewRecorder()
	controller.Verify(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK, got %d. Body: %s", w.Code, w.Body.String())
	}

	var envelope struct {
		Status string             `json:"status"`
		Data   dtos.LoginResponse `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("JSON decode error: %v", err)
	}

	if envelope.Data.Token != "RL-VALID-1234" {
		t.Errorf("Expected token 'RL-VALID-1234', got '%s'", envelope.Data.Token)
	}
	if envelope.Data.User.Name != "Valid Client" {
		t.Errorf("Expected user name 'Valid Client', got '%s'", envelope.Data.User.Name)
	}
}

func TestAuthController_Me_Authenticated(t *testing.T) {
	mock := &mockCredentialModel{}
	controller := NewAuthController(mock)

	user := &entities.AccessCredential{
		ID:            "admin-1",
		CredentialKey: "ADM-TEST",
		Name:          "Admin Test",
		Role:          "admin",
		IsActive:      true,
	}

	req := httptest.NewRequest("GET", "/api/auth/me", nil)
	ctx := context.WithValue(req.Context(), middlewares.UserContextKey, user)
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()
	controller.Me(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK, got %d", w.Code)
	}
}

func TestAdminController_CreateCredential(t *testing.T) {
	now := time.Now()
	mock := &mockCredentialModel{
		createFunc: func(name, role string, expiresAt *time.Time, canDiagram, canUI bool) (*entities.AccessCredential, error) {
			return &entities.AccessCredential{
				ID:                 "new-user-id",
				CredentialKey:      "RL-NEW-KEY-123",
				Name:               name,
				Role:               role,
				IsActive:           true,
				CanGenerateDiagram: canDiagram,
				CanGenerateUI:      canUI,
				ExpiresAt:          expiresAt,
				CreatedAt:          now,
				UpdatedAt:          now,
			}, nil
		},
	}
	adminCtrl := NewAdminController(mock)

	body := []byte(`{"name": "Client Beta", "duration_days": 7}`)
	req := httptest.NewRequest("POST", "/api/admin/credentials", bytes.NewBuffer(body))
	w := httptest.NewRecorder()

	adminCtrl.CreateCredential(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("Expected 201 Created, got %d. Body: %s", w.Code, w.Body.String())
	}

	var envelope struct {
		Status string                     `json:"status"`
		Data   dtos.CredentialSummaryDTO `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("JSON decode error: %v", err)
	}

	if envelope.Data.Name != "Client Beta" {
		t.Errorf("Expected 'Client Beta', got '%s'", envelope.Data.Name)
	}
	if envelope.Data.CredentialKey != "RL-NEW-KEY-123" {
		t.Errorf("Expected 'RL-NEW-KEY-123', got '%s'", envelope.Data.CredentialKey)
	}
}
