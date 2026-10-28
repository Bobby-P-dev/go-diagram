package controller_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	uidesigncontroller "github.com/Bobby-P-dev/go-diagram.git/internal/modules/uidesign/controller"
)

func TestUITemplateListIsEmptyWithoutService(t *testing.T) {
	controller := uidesigncontroller.NewUIDesignController(nil)
	response := httptest.NewRecorder()
	controller.GetTemplates(response, httptest.NewRequest(http.MethodGet, "/api/ui-design/templates", nil))
	var envelope struct {
		Data json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	if response.Code != http.StatusOK || string(envelope.Data) != "[]" {
		t.Fatalf("expected empty compatibility list, got %d: %s", response.Code, response.Body.String())
	}
}

func TestGenerationRejectsUITemplatesBeforeServiceOrStreaming(t *testing.T) {
	userReader := func(context.Context) *uidesigncontroller.AuthenticatedUser {
		return &uidesigncontroller.AuthenticatedUser{ID: "owner", CanGenerateUI: true}
	}
	controller := uidesigncontroller.NewUIDesignControllerWithUserReader(nil, userReader)
	asyncController := uidesigncontroller.NewAsyncJobControllerWithOptions(nil, userReader, nil)
	for _, endpoint := range []struct {
		path    string
		handler http.HandlerFunc
	}{
		{path: "/api/ui-design/generate", handler: controller.Create},
		{path: "/api/ui-design/generate/stream", handler: controller.CreateStream},
		{path: "/api/ui-design/generate/async", handler: asyncController.CreateAsyncUIDesign},
	} {
		for _, payload := range []string{
			`{"template_id":"old-template"}`,
			`{"template_id":"old-template","prompt":"Create a custom bookstore"}`,
			`{"template_id":" "}`,
		} {
			t.Run(endpoint.path+payload, func(t *testing.T) {
				response := httptest.NewRecorder()
				endpoint.handler(response, httptest.NewRequest(http.MethodPost, endpoint.path, strings.NewReader(payload)))
				if response.Code != http.StatusBadRequest || !strings.Contains(response.Body.String(), "template_id") {
					t.Fatalf("expected 400 with template guidance, got %d: %s", response.Code, response.Body.String())
				}
				if strings.Contains(response.Header().Get("Content-Type"), "text/event-stream") {
					t.Fatal("invalid requests must be rejected before starting SSE")
				}
			})
		}
	}
}
