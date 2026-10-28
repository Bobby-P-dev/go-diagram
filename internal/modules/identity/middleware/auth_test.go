package middleware

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Bobby-P-dev/go-diagram.git/internal/modules/identity/authcontext"
	"github.com/Bobby-P-dev/go-diagram.git/internal/modules/identity/service"
)

type finderStub struct {
	find func(context.Context, string) (*service.CredentialRecord, error)
}

func (f *finderStub) FindByKey(ctx context.Context, key string) (*service.CredentialRecord, error) {
	return f.find(ctx, key)
}

func TestAuthMiddlewarePublicEndpoints(t *testing.T) {
	stub := &finderStub{
		find: func(ctx context.Context, key string) (*service.CredentialRecord, error) {
			if key == "valid-key" {
				return &service.CredentialRecord{ID: "pub-user", Role: "user", IsActive: true}, nil
			}
			return nil, errors.New("not found")
		},
	}

	middleware := AuthMiddleware(stub)

	publicPaths := []struct {
		method string
		path   string
	}{
		{http.MethodOptions, "/api/projects"},
		{http.MethodGet, "/health"},
		{http.MethodGet, "/"},
		{http.MethodPost, "/api/auth/verify"},
		{http.MethodGet, "/api/shared/token-123"},
		{http.MethodPost, "/internal/v1/jobs/callback"},
	}

	for _, p := range publicPaths {
		t.Run(p.method+" "+p.path, func(t *testing.T) {
			called := false
			next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				called = true
				w.WriteHeader(http.StatusOK)
			})

			// without token
			req := httptest.NewRequest(p.method, p.path, nil)
			rec := httptest.NewRecorder()
			middleware(next).ServeHTTP(rec, req)
			if !called || rec.Code != http.StatusOK {
				t.Fatalf("expected next handler to be called without token, got code %d", rec.Code)
			}

			// with valid token
			called = false
			var injectedUser *service.CredentialRecord
			nextWithUser := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				called = true
				injectedUser = authcontext.UserFromContext(r.Context())
				w.WriteHeader(http.StatusOK)
			})
			req = httptest.NewRequest(p.method, p.path, nil)
			req.Header.Set("Authorization", "Bearer valid-key")
			rec = httptest.NewRecorder()
			middleware(nextWithUser).ServeHTTP(rec, req)
			if !called || rec.Code != http.StatusOK {
				t.Fatalf("expected next handler to be called with token, got code %d", rec.Code)
			}
			if injectedUser == nil || injectedUser.ID != "pub-user" {
				t.Fatalf("expected injected user in context, got %+v", injectedUser)
			}
		})
	}
}

func TestAuthMiddlewarePrivateEndpoints(t *testing.T) {
	future := time.Now().Add(24 * time.Hour)
	past := time.Now().Add(-24 * time.Hour)

	stub := &finderStub{
		find: func(ctx context.Context, key string) (*service.CredentialRecord, error) {
			switch key {
			case "valid":
				return &service.CredentialRecord{ID: "valid-user", Role: "user", IsActive: true, ExpiresAt: &future}, nil
			case "inactive":
				return &service.CredentialRecord{ID: "inactive-user", Role: "user", IsActive: false}, nil
			case "expired":
				return &service.CredentialRecord{ID: "expired-user", Role: "user", IsActive: true, ExpiresAt: &past}, nil
			default:
				return nil, errors.New("not found")
			}
		},
	}

	middleware := AuthMiddleware(stub)

	tests := []struct {
		name       string
		authHeader string
		wantStatus int
		wantMsg    string
	}{
		{"missing auth header", "", http.StatusUnauthorized, "Autentikasi Diperlukan"},
		{"empty bearer token", "Bearer ", http.StatusUnauthorized, "Kredensial Tidak Valid"},
		{"not found token", "Bearer unknown", http.StatusUnauthorized, "Kredensial Tidak Ditemukan"},
		{"inactive token", "Bearer inactive", http.StatusForbidden, "Akses Dinonaktifkan"},
		{"expired token", "Bearer expired", http.StatusForbidden, "Masa Aktif Kedaluwarsa"},
		{"valid token", "Bearer valid", http.StatusOK, ""},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			called := false
			next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				called = true
				w.WriteHeader(http.StatusOK)
			})

			req := httptest.NewRequest(http.MethodGet, "/api/projects", nil)
			if test.authHeader != "" {
				req.Header.Set("Authorization", test.authHeader)
			}
			rec := httptest.NewRecorder()
			middleware(next).ServeHTTP(rec, req)

			if rec.Code != test.wantStatus {
				t.Fatalf("expected status %d, got %d. Body: %s", test.wantStatus, rec.Code, rec.Body.String())
			}
			if test.wantMsg != "" && !strings.Contains(rec.Body.String(), test.wantMsg) {
				t.Fatalf("expected body to contain %q, got %s", test.wantMsg, rec.Body.String())
			}
			if test.wantStatus == http.StatusOK && !called {
				t.Fatalf("expected next handler to be called")
			}
		})
	}
}

func TestAuthMiddlewareContextPropagation(t *testing.T) {
	type testCtxKey string
	const key testCtxKey = "custom_test_context_val"

	contextVerified := false
	stub := &finderStub{
		find: func(ctx context.Context, token string) (*service.CredentialRecord, error) {
			if val, ok := ctx.Value(key).(string); ok && val == "injected_context" {
				contextVerified = true
			}
			return &service.CredentialRecord{ID: "user-1", IsActive: true}, nil
		},
	}

	middleware := AuthMiddleware(stub)
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/api/projects", nil)
	req.Header.Set("Authorization", "Bearer valid")
	req = req.WithContext(context.WithValue(req.Context(), key, "injected_context"))
	rec := httptest.NewRecorder()

	middleware(next).ServeHTTP(rec, req)
	if !contextVerified {
		t.Fatalf("request context was not properly forwarded to CredentialFinder")
	}
}

func TestRequireAdmin(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`ok`))
	})

	guarded := RequireAdmin(next)

	// No user in context
	req := httptest.NewRequest(http.MethodGet, "/api/admin/credentials", nil)
	rec := httptest.NewRecorder()
	guarded(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 Forbidden without user, got %d", rec.Code)
	}

	// User role
	user := &service.CredentialRecord{ID: "usr", Role: "user"}
	req = httptest.NewRequest(http.MethodGet, "/api/admin/credentials", nil)
	req = req.WithContext(authcontext.WithUser(req.Context(), user))
	rec = httptest.NewRecorder()
	guarded(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 Forbidden for non-admin, got %d", rec.Code)
	}

	// Admin role
	admin := &service.CredentialRecord{ID: "adm", Role: "admin"}
	req = httptest.NewRequest(http.MethodGet, "/api/admin/credentials", nil)
	req = req.WithContext(authcontext.WithUser(req.Context(), admin))
	rec = httptest.NewRecorder()
	guarded(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for admin, got %d", rec.Code)
	}
}

func TestOptionalAuthenticationRejectsInvalidIdentityWithoutBlockingPublicRoute(t *testing.T) {
	past := time.Now().Add(-time.Hour)
	for _, test := range []struct {
		name string
		user *service.CredentialRecord
		err  error
	}{
		{"invalid", nil, errors.New("invalid")},
		{"inactive", &service.CredentialRecord{}, nil},
		{"expired", &service.CredentialRecord{IsActive: true, ExpiresAt: &past}, nil},
	} {
		t.Run(test.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
			defer cancel()
			cancel()
			called := false
			stub := &finderStub{find: func(actual context.Context, token string) (*service.CredentialRecord, error) {
				if actual != ctx || !errors.Is(actual.Err(), context.Canceled) || token != "token" {
					t.Fatal("optional auth lost request context or bearer parsing")
				}
				called = true
				return test.user, test.err
			}}
			next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if authcontext.UserFromContext(r.Context()) != nil {
					t.Fatal("invalid optional identity was injected")
				}
				w.WriteHeader(http.StatusNoContent)
			})
			request := httptest.NewRequest(http.MethodGet, "/api/shared/token", nil).WithContext(ctx)
			request.Header.Set("Authorization", "Bearer token")
			response := httptest.NewRecorder()
			AuthMiddleware(stub)(next).ServeHTTP(response, request)
			if !called || response.Code != http.StatusNoContent {
				t.Fatalf("public route blocked: %d", response.Code)
			}
		})
	}
}

func TestPublicPathsAreMethodSpecific(t *testing.T) {
	for _, test := range []struct {
		method, path string
	}{
		{http.MethodPost, "/api/shared/token/fork"},
		{http.MethodPut, "/api/shared/token"},
		{http.MethodGet, "/api/auth/verify"},
		{http.MethodGet, "/api/auth/me"},
	} {
		request := httptest.NewRequest(test.method, test.path, nil)
		response := httptest.NewRecorder()
		AuthMiddleware(nil)(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
			t.Fatal("protected route was treated as public")
		})).ServeHTTP(response, request)
		if response.Code != http.StatusUnauthorized {
			t.Fatalf("%s %s: %d", test.method, test.path, response.Code)
		}
	}
}
