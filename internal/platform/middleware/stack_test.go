package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCORSCharacterization(test *testing.T) {
	for _, scenario := range []struct {
		name, origins, origin, expected string
	}{
		{"whitespace list", " https://first.test, https://second.test ", "https://second.test", "https://second.test"},
		{"wildcard echoes", "*", "https://arbitrary.test", "https://arbitrary.test"},
		{"empty origin", "*", "", ""},
		{"loopback legacy exception", "https://app.test", "http://127.0.0.1:5173", "http://127.0.0.1:5173"},
		{"localhost empty allowlist", "", "http://localhost:5173", "http://localhost:5173"},
		{"https localhost not implicit", "https://app.test", "https://localhost:5173", ""},
		{"near match not allowed", "https://app.test", "https://app.test.attacker.test", ""},
	} {
		test.Run(scenario.name, func(test *testing.T) {
			called := false
			handler := CORSMiddleware(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
				called = true
				response.WriteHeader(http.StatusAccepted)
			}), scenario.origins)
			request := httptest.NewRequest(http.MethodGet, "/api/projects", nil)
			request.Header.Set("Origin", scenario.origin)
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)
			if !called || response.Code != http.StatusAccepted || response.Header().Get("Access-Control-Allow-Origin") != scenario.expected {
				test.Fatalf("CORS changed: status=%d headers=%v called=%v", response.Code, response.Header(), called)
			}
			for name, expected := range map[string]string{
				"Access-Control-Allow-Methods":     "GET, POST, PUT, DELETE, PATCH, OPTIONS",
				"Access-Control-Allow-Headers":     "Content-Type, Authorization, X-Requested-With, X-Internal-Service-Key",
				"Access-Control-Allow-Credentials": "true",
				"Access-Control-Max-Age":           "86400",
			} {
				if response.Header().Get(name) != expected {
					test.Fatalf("%s=%q, want %q", name, response.Header().Get(name), expected)
				}
			}
		})
	}
}

func TestMiddlewareStackPreflightAndStreaming(test *testing.T) {
	called := false
	callback := RequireInternalKey("worker-secret", func(response http.ResponseWriter, request *http.Request) {
		called = true
		response.Header().Set("Content-Type", "text/event-stream")
		_, _ = response.Write([]byte("event: progress\ndata: {\"status\":\"working\"}\n\n"))
		flusher, ok := response.(http.Flusher)
		if !ok {
			test.Fatal("logger must preserve SSE Flusher")
		}
		flusher.Flush()
	})
	handler := CORSMiddleware(LoggerMiddleware(callback), "https://app.test")
	request := httptest.NewRequest(http.MethodOptions, "/internal/v1/jobs/callback", nil)
	request.Header.Set("Origin", "https://app.test")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if called || response.Code != http.StatusNoContent || response.Body.Len() != 0 {
		test.Fatalf("preflight reached callback: called=%v status=%d body=%s", called, response.Code, response.Body.String())
	}

	request = httptest.NewRequest(http.MethodPost, "/internal/v1/jobs/callback", nil)
	request.Header.Set("Origin", "https://app.test")
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if called || response.Code != http.StatusUnauthorized || response.Body.String() != "{\"status\":\"error\",\"message\":\"Unauthorized\",\"error\":\"Invalid or missing internal service key\"}\n" {
		test.Fatalf("callback rejection contract changed: called=%v status=%d body=%s", called, response.Code, response.Body.String())
	}

	request.Header.Set("X-Internal-Service-Key", " worker-secret ")
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if !called || !response.Flushed || response.Code != http.StatusOK || response.Header().Get("Content-Type") != "text/event-stream" || response.Body.String() != "event: progress\ndata: {\"status\":\"working\"}\n\n" {
		test.Fatalf("SSE changed: called=%v flushed=%v status=%d body=%s", called, response.Flushed, response.Code, response.Body.String())
	}
}

type basicResponseWriter struct {
	header http.Header
	status int
}

func (writer *basicResponseWriter) Header() http.Header            { return writer.header }
func (writer *basicResponseWriter) WriteHeader(status int)         { writer.status = status }
func (writer *basicResponseWriter) Write(body []byte) (int, error) { return len(body), nil }

func TestLoggerWithoutUnderlyingFlusher(test *testing.T) {
	writer := &basicResponseWriter{header: make(http.Header)}
	handler := LoggerMiddleware(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		response.WriteHeader(http.StatusCreated)
		response.(http.Flusher).Flush()
	}))
	handler.ServeHTTP(writer, httptest.NewRequest(http.MethodPost, "/api/projects", nil))
	if writer.status != http.StatusCreated {
		test.Fatalf("status=%d, want 201", writer.status)
	}
}

func TestInternalKeyIsolation(test *testing.T) {
	if ValidInternalKey("secret", nil) {
		test.Fatal("nil request must fail closed")
	}
	for _, secret := range []string{"secret-one", "secret-two", " \t "} {
		test.Run(secret, func(test *testing.T) {
			test.Parallel()
			handler := RequireInternalKey(secret, func(response http.ResponseWriter, request *http.Request) {
				response.WriteHeader(http.StatusNoContent)
			})
			for _, provided := range []string{"secret-one", "secret-two", ""} {
				request := httptest.NewRequest(http.MethodPost, "/internal/v1/jobs/callback", nil)
				request.Header.Set("X-Internal-Service-Key", provided)
				response := httptest.NewRecorder()
				handler(response, request)
				expected := http.StatusUnauthorized
				if secret == provided && provided != "" {
					expected = http.StatusNoContent
				}
				if response.Code != expected {
					test.Fatalf("status=%d, want %d", response.Code, expected)
				}
			}
		})
	}
}
