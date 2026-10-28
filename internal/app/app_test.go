package app_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Bobby-P-dev/go-diagram.git/internal/app"
	identityrepository "github.com/Bobby-P-dev/go-diagram.git/internal/modules/identity/repository"
	identityservice "github.com/Bobby-P-dev/go-diagram.git/internal/modules/identity/service"
	platformconfig "github.com/Bobby-P-dev/go-diagram.git/internal/platform/config"
	"github.com/DATA-DOG/go-sqlmock"
)

func TestHandlerWiringWithoutLegacyGlobals(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	credentials := identityservice.NewCredentialService(identityrepository.NewPostgresRepository(db))
	first := app.NewHandler(&platformconfig.EnvConfig{
		AIProvider: "openai", InternalServiceKey: "first-key", CORSAllowedOrigins: "https://first.example",
	}, db, nil, credentials)
	second := app.NewHandler(&platformconfig.EnvConfig{
		AIProvider: "anthropic", InternalServiceKey: "second-key", CORSAllowedOrigins: "https://second.example",
	}, db, nil, credentials)

	for _, test := range []struct {
		name, method, path, key string
		handler                 http.Handler
		want                    int
	}{
		{"health", "GET", "/health", "", first, http.StatusOK},
		{"root", "GET", "/", "", first, http.StatusOK},
		{"diagram requires bearer", "GET", "/api/projects", "", first, http.StatusUnauthorized},
		{"workspace requires bearer", "GET", "/api/settings", "", first, http.StatusUnauthorized},
		{"ui requires bearer", "POST", "/api/ui-design/generate", "", first, http.StatusUnauthorized},
		{"callback missing key", "POST", "/internal/v1/jobs/callback", "", first, http.StatusUnauthorized},
		{"callback wrong key", "POST", "/internal/v1/jobs/callback", "wrong", first, http.StatusUnauthorized},
		{"first callback reaches decoder", "POST", "/internal/v1/jobs/callback", "first-key", first, http.StatusBadRequest},
		{"second callback reaches decoder", "POST", "/internal/v1/jobs/callback", "second-key", second, http.StatusBadRequest},
		{"first rejects second key", "POST", "/internal/v1/jobs/callback", "second-key", first, http.StatusUnauthorized},
		{"second rejects first key", "POST", "/internal/v1/jobs/callback", "first-key", second, http.StatusUnauthorized},
	} {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(test.method, test.path, strings.NewReader("{invalid"))
			request.Header.Set("X-Internal-Service-Key", test.key)
			response := httptest.NewRecorder()
			test.handler.ServeHTTP(response, request)
			if response.Code != test.want {
				t.Fatalf("status %d, want %d: %s", response.Code, test.want, response.Body.String())
			}
		})
	}
	for _, test := range []struct {
		handler http.Handler
		origin  string
	}{
		{first, "https://first.example"}, {second, "https://second.example"},
	} {
		request := httptest.NewRequest(http.MethodOptions, "/api/projects", nil)
		request.Header.Set("Origin", test.origin)
		response := httptest.NewRecorder()
		test.handler.ServeHTTP(response, request)
		if got := response.Header().Get("Access-Control-Allow-Origin"); got != test.origin {
			t.Fatalf("CORS origin = %q, want %q", got, test.origin)
		}
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
