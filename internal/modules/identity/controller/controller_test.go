package controller

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/Bobby-P-dev/go-diagram.git/internal/modules/identity/authcontext"
	"github.com/Bobby-P-dev/go-diagram.git/internal/modules/identity/dto"
	"github.com/Bobby-P-dev/go-diagram.git/internal/modules/identity/service"
)

type credentialStub struct {
	findByKey func(context.Context, string) (*service.CredentialRecord, error)
	lastLogin func(context.Context, string) error
	findByID  func(context.Context, string) (*service.CredentialRecord, error)
	create    func(context.Context, service.CreateInput) (*service.CredentialRecord, error)
	update    func(context.Context, service.UpdateInput) (*service.CredentialRecord, error)
	delete    func(context.Context, string) error
	list      func(context.Context) ([]service.Summary, error)
}

func (stub *credentialStub) FindByKey(ctx context.Context, key string) (*service.CredentialRecord, error) {
	return stub.findByKey(ctx, key)
}

func (stub *credentialStub) UpdateLastLogin(ctx context.Context, id string) error {
	return stub.lastLogin(ctx, id)
}

func (stub *credentialStub) FindByID(ctx context.Context, id string) (*service.CredentialRecord, error) {
	return stub.findByID(ctx, id)
}

func (stub *credentialStub) Create(ctx context.Context, input service.CreateInput) (*service.CredentialRecord, error) {
	return stub.create(ctx, input)
}

func (stub *credentialStub) Update(ctx context.Context, input service.UpdateInput) (*service.CredentialRecord, error) {
	return stub.update(ctx, input)
}

func (stub *credentialStub) Delete(ctx context.Context, id string) error {
	return stub.delete(ctx, id)
}

func (stub *credentialStub) GetAllWithStats(ctx context.Context) ([]service.Summary, error) {
	return stub.list(ctx)
}

func canceledRequestContext(t *testing.T) context.Context {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	t.Cleanup(cancel)
	cancel()
	return ctx
}

func TestVerifyPreservesContextAndTokenContract(t *testing.T) {
	ctx := canceledRequestContext(t)
	var calls []string
	stub := &credentialStub{
		findByKey: func(actual context.Context, key string) (*service.CredentialRecord, error) {
			if actual != ctx || !errors.Is(actual.Err(), context.Canceled) || key != "RL-raw-secret" {
				t.Fatalf("lost request context or normalization: %v, %q", actual.Err(), key)
			}
			calls = append(calls, "find")
			return &service.CredentialRecord{ID: "user-1", CredentialKey: "RL-raw-s********", IsActive: true}, nil
		},
		lastLogin: func(actual context.Context, id string) error {
			if actual != ctx || id != "user-1" {
				t.Fatal("last login lost request context or ID")
			}
			calls = append(calls, "login")
			return context.Canceled
		},
	}
	request := httptest.NewRequest(http.MethodPost, "/api/auth/verify", strings.NewReader(`{"credential_key":"  RL-raw-secret  "}`)).WithContext(ctx)
	response := httptest.NewRecorder()
	NewAuthController(stub).Verify(response, request)
	if response.Code != http.StatusOK || !reflect.DeepEqual(calls, []string{"find", "login"}) {
		t.Fatalf("status/calls: %d %v: %s", response.Code, calls, response.Body.String())
	}
	var body struct {
		Status string            `json:"status"`
		Data   dto.LoginResponse `json:"data"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Status != "success" || body.Data.Token != "RL-raw-secret" || body.Data.User.CredentialKey != "RL-raw-s********" {
		t.Fatalf("login contract changed: %+v", body)
	}
}

func TestVerifyErrorContract(t *testing.T) {
	past := time.Now().Add(-time.Hour)
	for _, test := range []struct {
		name, body, message string
		user                *service.CredentialRecord
		err                 error
		status              int
	}{
		{"invalid JSON", "{", "Invalid Request", nil, nil, 400},
		{"blank key", `{"credential_key":" "}`, "Kredensial Diperlukan", nil, nil, 400},
		{"not found", `{"credential_key":"bad"}`, "Kredensial Tidak Ditemukan", nil, errors.New("missing"), 401},
		{"disabled", `{"credential_key":"disabled"}`, "Kredensial Dinonaktifkan", &service.CredentialRecord{}, nil, 403},
		{"expired", `{"credential_key":"expired"}`, "Masa Aktif Kedaluwarsa", &service.CredentialRecord{IsActive: true, ExpiresAt: &past}, nil, 403},
	} {
		t.Run(test.name, func(t *testing.T) {
			stub := &credentialStub{findByKey: func(context.Context, string) (*service.CredentialRecord, error) {
				return test.user, test.err
			}}
			response := httptest.NewRecorder()
			NewAuthController(stub).Verify(response, httptest.NewRequest(http.MethodPost, "/api/auth/verify", strings.NewReader(test.body)))
			if response.Code != test.status || !strings.Contains(response.Body.String(), `"message":"`+test.message+`"`) {
				t.Fatalf("unexpected response: %d %s", response.Code, response.Body.String())
			}
		})
	}
}

func TestProfileAndLogoutContract(t *testing.T) {
	controller := NewAuthController(nil)
	request := httptest.NewRequest(http.MethodGet, "/api/auth/me", nil)
	response := httptest.NewRecorder()
	controller.Me(response, request)
	if response.Code != http.StatusUnauthorized {
		t.Fatal(response.Code)
	}
	user := &service.CredentialRecord{ID: "user-1", CredentialKey: "RL-mask********", Role: "admin", IsActive: true}
	response = httptest.NewRecorder()
	controller.Me(response, request.WithContext(authcontext.WithUser(request.Context(), user)))
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"credential_key":"RL-mask********"`) {
		t.Fatalf("profile contract: %d %s", response.Code, response.Body.String())
	}
	response = httptest.NewRecorder()
	controller.Logout(response, httptest.NewRequest(http.MethodPost, "/api/auth/logout", nil))
	if response.Code != http.StatusOK || response.Body.String() != "{\"status\":\"success\",\"message\":\"Berhasil logout\",\"data\":{\"success\":true}}\n" {
		t.Fatal(response.Body.String())
	}
}

func TestAdminPreservesContextAndResponseContract(t *testing.T) {
	ctx := canceledRequestContext(t)
	check := func(actual context.Context) {
		t.Helper()
		if actual != ctx || !errors.Is(actual.Err(), context.Canceled) {
			t.Fatal("admin call did not preserve canceled request context")
		}
	}
	future := time.Now().Add(24 * time.Hour)
	var calls []string
	stub := &credentialStub{
		list: func(actual context.Context) ([]service.Summary, error) {
			check(actual)
			calls = append(calls, "list")
			return []service.Summary{{ID: "id", CredentialKey: "RL-mask********", ProjectCount: 7}}, nil
		},
		create: func(actual context.Context, input service.CreateInput) (*service.CredentialRecord, error) {
			check(actual)
			calls = append(calls, "create")
			if input.Role != "user" || !input.CanGenerateDiagram || !input.CanGenerateUI || input.ExpiresAt != nil {
				t.Fatalf("legacy create defaults changed: %+v", input)
			}
			return &service.CredentialRecord{ID: "id", CredentialKey: "RL-once-only"}, nil
		},
		findByID: func(actual context.Context, id string) (*service.CredentialRecord, error) {
			check(actual)
			calls = append(calls, "find")
			if id != "id" {
				t.Fatal(id)
			}
			return &service.CredentialRecord{ID: id, ExpiresAt: &future}, nil
		},
		update: func(actual context.Context, input service.UpdateInput) (*service.CredentialRecord, error) {
			check(actual)
			calls = append(calls, "update")
			if input.ID != "id" || !input.UpdateExpiresAt || input.ExpiresAt == nil || !input.ExpiresAt.Equal(future.Add(time.Hour)) {
				t.Fatalf("extend input: %+v", input)
			}
			return &service.CredentialRecord{ID: input.ID, CredentialKey: "RL-mask********", ExpiresAt: input.ExpiresAt}, nil
		},
		delete: func(actual context.Context, id string) error {
			check(actual)
			calls = append(calls, "delete")
			if id != "id" {
				t.Fatal(id)
			}
			return nil
		},
	}
	controller := NewAdminController(stub)
	for _, test := range []struct {
		name, body, expected string
		handler              http.HandlerFunc
		status               int
	}{
		{"list", "", `"project_count":7`, controller.GetCredentials, 200},
		{"create", `{"name":"Client","role":"other","can_generate_diagram":false,"can_generate_ui":false}`, `"credential_key":"RL-once-only"`, controller.CreateCredential, 201},
		{"update", `{"extend_hours":1}`, `"credential_key":"RL-mask********"`, controller.UpdateCredential, 200},
		{"delete", "", `"success":true`, controller.DeleteCredential, 200},
	} {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodPost, "/api/admin/credentials/id", strings.NewReader(test.body)).WithContext(ctx)
			request.SetPathValue("id", "id")
			response := httptest.NewRecorder()
			test.handler(response, request)
			if response.Code != test.status || !strings.Contains(response.Body.String(), test.expected) {
				t.Fatalf("response: %d %s", response.Code, response.Body.String())
			}
		})
	}
	if !reflect.DeepEqual(calls, []string{"list", "create", "find", "update", "delete"}) {
		t.Fatalf("unexpected calls: %v", calls)
	}
}

func TestAdminExpiryUpdateCompatibility(t *testing.T) {
	for _, test := range []struct {
		name, body string
		update     bool
		expiry     string
	}{
		{"omitted", `{}`, false, ""},
		{"permanent", `{"set_permanent":true}`, true, ""},
		{"empty", `{"expires_at":""}`, true, ""},
		{"never", `{"expires_at":"never"}`, true, ""},
		{"string null", `{"expires_at":"null"}`, true, ""},
		{"JSON null", `{"expires_at":null}`, false, ""},
		{"invalid ignored", `{"expires_at":"invalid"}`, false, ""},
		{"date only", `{"expires_at":"2030-01-02"}`, true, "2030-01-02T00:00:00Z"},
		{"date time", `{"expires_at":"2030-01-02 15:04"}`, true, "2030-01-02T15:04:00Z"},
	} {
		t.Run(test.name, func(t *testing.T) {
			called := false
			stub := &credentialStub{update: func(_ context.Context, input service.UpdateInput) (*service.CredentialRecord, error) {
				called = true
				formatted := ""
				if input.ExpiresAt != nil {
					formatted = input.ExpiresAt.Format(time.RFC3339)
				}
				if input.UpdateExpiresAt != test.update || formatted != test.expiry {
					t.Fatalf("expiry input: %+v", input)
				}
				return &service.CredentialRecord{ID: "id"}, nil
			}}
			request := httptest.NewRequest(http.MethodPut, "/api/admin/credentials/id", strings.NewReader(test.body))
			request.SetPathValue("id", "id")
			response := httptest.NewRecorder()
			NewAdminController(stub).UpdateCredential(response, request)
			if response.Code != http.StatusOK || !called {
				t.Fatalf("response: %d %s", response.Code, response.Body.String())
			}
		})
	}
}

func TestAdminErrorContract(t *testing.T) {
	failure := errors.New("operation failed")
	stub := &credentialStub{
		list:   func(context.Context) ([]service.Summary, error) { return nil, failure },
		create: func(context.Context, service.CreateInput) (*service.CredentialRecord, error) { return nil, failure },
		update: func(context.Context, service.UpdateInput) (*service.CredentialRecord, error) { return nil, failure },
		delete: func(context.Context, string) error { return failure },
	}
	controller := NewAdminController(stub)
	for _, test := range []struct {
		name, body, id string
		handler        http.HandlerFunc
		status         int
	}{
		{"list error", "", "", controller.GetCredentials, 500},
		{"create error", `{}`, "", controller.CreateCredential, 500},
		{"create invalid JSON", `{`, "", controller.CreateCredential, 400},
		{"update error", `{}`, "id", controller.UpdateCredential, 500},
		{"update invalid JSON", `{`, "id", controller.UpdateCredential, 400},
		{"update missing ID", `{}`, "", controller.UpdateCredential, 400},
		{"delete error", "", "id", controller.DeleteCredential, 400},
		{"delete missing ID", "", "", controller.DeleteCredential, 400},
	} {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(test.body))
			request.SetPathValue("id", test.id)
			response := httptest.NewRecorder()
			test.handler(response, request)
			if response.Code != test.status || !strings.Contains(response.Body.String(), `"status":"error"`) {
				t.Fatalf("response: %d %s", response.Code, response.Body.String())
			}
		})
	}
}
