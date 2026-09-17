package controllers

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Bobby-P-dev/go-diagram.git/src/config"
	"github.com/Bobby-P-dev/go-diagram.git/src/services"
)

func TestAsyncJobController_UnauthorizedCallback(t *testing.T) {
	config.Env = &config.EnvConfig{
		InternalServiceKey: "test-secret-key",
	}

	dispatcher := services.NewJobDispatcherService(nil, nil, nil, nil)
	controller := NewAsyncJobController(dispatcher, nil)

	body := []byte(`{"job_id": "job-1", "status": "success"}`)
	req := httptest.NewRequest("POST", "/internal/v1/jobs/callback", bytes.NewBuffer(body))
	req.Header.Set("X-Internal-Service-Key", "invalid-key")
	w := httptest.NewRecorder()

	controller.HandleJobCallback(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("Expected status 401 Unauthorized, got %d", w.Code)
	}
}

func TestAsyncJobController_InvalidJSONDispatch(t *testing.T) {
	config.Env = &config.EnvConfig{
		InternalServiceKey: "test-secret-key",
	}

	dispatcher := services.NewJobDispatcherService(nil, nil, nil, nil)
	controller := NewAsyncJobController(dispatcher, nil)

	body := []byte(`{invalid-json}`)
	req := httptest.NewRequest("POST", "/api/ui-design/generate/async", bytes.NewBuffer(body))
	w := httptest.NewRecorder()

	controller.CreateAsyncUIDesign(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400 Bad Request, got %d", w.Code)
	}
}
