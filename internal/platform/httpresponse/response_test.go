package httpresponse

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRespondJSON(t *testing.T) {
	rec := httptest.NewRecorder()
	payload := map[string]string{"foo": "bar"}
	RespondJSON(rec, http.StatusOK, "sukses", payload)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Fatalf("expected Content-Type application/json, got %q", ct)
	}

	var resp SuccessResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.Status != "success" || resp.Message != "sukses" {
		t.Fatalf("unexpected response: %+v", resp)
	}
}

func TestRespondError(t *testing.T) {
	rec := httptest.NewRecorder()
	RespondError(rec, http.StatusForbidden, "Akses Ditolak", "Detail error")

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected status 403, got %d", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Fatalf("expected Content-Type application/json, got %q", ct)
	}

	var resp ErrorResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.Status != "error" || resp.Message != "Akses Ditolak" || resp.Error != "Detail error" {
		t.Fatalf("unexpected error response: %+v", resp)
	}
}
