package controller

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Bobby-P-dev/go-diagram.git/internal/modules/generation"
	"github.com/Bobby-P-dev/go-diagram.git/internal/modules/identity/authcontext"
	identityservice "github.com/Bobby-P-dev/go-diagram.git/internal/modules/identity/service"
)

type fakeResolver struct {
	calls int
	err   error
	scope generation.Scope
}

func (resolver *fakeResolver) Resolve(_ context.Context, scope generation.Scope, _ generation.ResolveInput) (*generation.Manifest, error) {
	resolver.calls++
	resolver.scope = scope
	return &generation.Manifest{PreviewOnly: true}, resolver.err
}

func TestContextController(test *testing.T) {
	cases := []struct {
		name, body    string
		auth          bool
		err           error
		status, calls int
	}{
		{"authenticated preview", `{}`, true, nil, 200, 1},
		{"auth required", `{}`, false, nil, 401, 0},
		{"null", `null`, true, nil, 400, 0},
		{"unknown field", `{"user_id":"forged"}`, true, nil, 400, 0},
		{"trailing body", `{} {}`, true, nil, 400, 0},
		{"malformed", `{`, true, nil, 400, 0},
		{"too large", `{"mode":"` + strings.Repeat("x", 33*1024) + `"}`, true, nil, 413, 0},
		{"too large trailing whitespace", `{}` + strings.Repeat(" ", 33*1024), true, nil, 413, 0},
		{"not found", `{}`, true, generation.ErrNotFound, 404, 1},
		{"invalid", `{}`, true, generation.ErrInvalidInput, 400, 1},
		{"budget", `{}`, true, generation.ErrBudgetExceeded, 422, 1},
		{"internal", `{}`, true, errors.New("secret database credentials"), 500, 1},
	}
	for _, item := range cases {
		test.Run(item.name, func(test *testing.T) {
			resolver := &fakeResolver{err: item.err}
			controller := NewContextController(resolver)
			mux := http.NewServeMux()
			mux.HandleFunc("POST /api/v2/projects/{projectId}/threads/{threadId}/context/resolve", controller.Resolve)
			request := httptest.NewRequest(http.MethodPost, "/api/v2/projects/project-a/threads/thread-a/context/resolve", strings.NewReader(item.body))
			if item.auth {
				request = request.WithContext(authcontext.WithUser(request.Context(), &identityservice.CredentialRecord{ID: "server-user"}))
			}
			response := httptest.NewRecorder()
			mux.ServeHTTP(response, request)
			if response.Code != item.status || resolver.calls != item.calls {
				test.Fatalf("status=%d calls=%d body=%s", response.Code, resolver.calls, response.Body)
			}
			if strings.Contains(response.Body.String(), "secret database") {
				test.Fatal("internal error leaked")
			}
			if item.status >= 400 {
				var envelope struct{ Code, RequestID string }
				var body map[string]string
				if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
					test.Fatal(err)
				}
				envelope.Code, envelope.RequestID = body["code"], body["request_id"]
				if envelope.Code == "" || envelope.RequestID == "" || response.Header().Get("X-Request-ID") != envelope.RequestID {
					test.Fatal("not a V2 error envelope")
				}
			}
			if item.calls > 0 && (resolver.scope.UserID != "server-user" || resolver.scope.ProjectID != "project-a" || resolver.scope.ThreadID != "thread-a") {
				test.Fatalf("incorrect server scope: %+v", resolver.scope)
			}
		})
	}
}
