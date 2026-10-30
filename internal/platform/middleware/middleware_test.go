package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCORSMiddleware(t *testing.T) {
	handler := CORSMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	}), "http://example.com,http://localhost:3000")

	t.Run("Options preflight request", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodOptions, "/api/projects", nil)
		req.Header.Set("Origin", "http://example.com")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusNoContent {
			t.Errorf("expected 204 No Content for preflight, got %d", rec.Code)
		}
		if rec.Header().Get("Access-Control-Allow-Origin") != "http://example.com" {
			t.Errorf("expected origin http://example.com, got %s", rec.Header().Get("Access-Control-Allow-Origin"))
		}
	})

	t.Run("Unlisted localhost origin rejected", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/projects", nil)
		req.Header.Set("Origin", "http://localhost:5173")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", rec.Code)
		}
		if rec.Header().Get("Access-Control-Allow-Origin") != "" {
			t.Errorf("expected unlisted localhost origin rejected, got %s", rec.Header().Get("Access-Control-Allow-Origin"))
		}
	})

	t.Run("Non-allowed origin", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/projects", nil)
		req.Header.Set("Origin", "http://malicious.com")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Header().Get("Access-Control-Allow-Origin") != "" {
			t.Errorf("expected no allow origin header, got %s", rec.Header().Get("Access-Control-Allow-Origin"))
		}
	})
}

func TestLoggerMiddleware(t *testing.T) {
	called := false
	handler := LoggerMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if !called || rec.Code != http.StatusOK {
		t.Fatalf("expected handler to be called with 200, got called=%v code=%d", called, rec.Code)
	}
}

func TestInternalAuth(t *testing.T) {
	secret := "my-secret-key"

	t.Run("ValidInternalKey matches correctly", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/internal", nil)
		if ValidInternalKey(secret, req) {
			t.Errorf("expected false for missing header")
		}

		req.Header.Set("X-Internal-Service-Key", "wrong")
		if ValidInternalKey(secret, req) {
			t.Errorf("expected false for wrong key")
		}

		req.Header.Set("X-Internal-Service-Key", secret)
		if !ValidInternalKey(secret, req) {
			t.Errorf("expected true for correct key")
		}

		if ValidInternalKey("", req) {
			t.Errorf("expected false for empty configured secret")
		}
	})

	t.Run("RequireInternalKey middleware", func(t *testing.T) {
		called := false
		guarded := RequireInternalKey(secret, func(w http.ResponseWriter, r *http.Request) {
			called = true
			w.WriteHeader(http.StatusOK)
		})

		// Unauthorized
		req := httptest.NewRequest(http.MethodPost, "/internal", nil)
		rec := httptest.NewRecorder()
		guarded(rec, req)
		if rec.Code != http.StatusUnauthorized || called {
			t.Errorf("expected 401 Unauthorized, got %d", rec.Code)
		}

		// Authorized
		req.Header.Set("X-Internal-Service-Key", secret)
		rec = httptest.NewRecorder()
		guarded(rec, req)
		if rec.Code != http.StatusOK || !called {
			t.Errorf("expected 200 OK, got %d", rec.Code)
		}
	})
}
