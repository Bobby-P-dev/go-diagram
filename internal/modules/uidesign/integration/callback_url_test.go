package integration

import (
	"errors"
	"testing"
)

func TestResolveCallbackURL(t *testing.T) {
	// 1. BACKEND_INTERNAL_URL takes precedence
	url := resolveCallbackURL("http://internal-svc:8080/", "http://go-backend:8080", "8080", nil)
	if url != "http://internal-svc:8080/internal/v1/jobs/callback" {
		t.Fatalf("expected internal URL precedence, got %s", url)
	}

	// 2. GO_BACKEND_URL fallback
	url = resolveCallbackURL("", "http://go-backend:9000", "8080", nil)
	if url != "http://go-backend:9000/internal/v1/jobs/callback" {
		t.Fatalf("expected go backend URL, got %s", url)
	}

	// 3. DNS lookup "backend"
	lookupBackend := func(host string) ([]string, error) {
		if host == "backend" {
			return []string{"172.18.0.2"}, nil
		}
		return nil, errors.New("not found")
	}
	url = resolveCallbackURL("", "", "8080", lookupBackend)
	if url != "http://backend:8080/internal/v1/jobs/callback" {
		t.Fatalf("expected docker backend hostname, got %s", url)
	}

	// 4. DNS lookup "diagram_backend_prod"
	lookupProd := func(host string) ([]string, error) {
		if host == "diagram_backend_prod" {
			return []string{"172.18.0.3"}, nil
		}
		return nil, errors.New("not found")
	}
	url = resolveCallbackURL("", "", "8080", lookupProd)
	if url != "http://diagram_backend_prod:8080/internal/v1/jobs/callback" {
		t.Fatalf("expected docker prod hostname, got %s", url)
	}

	// 5. Localhost fallback
	lookupFail := func(host string) ([]string, error) {
		return nil, errors.New("dns failure")
	}
	url = resolveCallbackURL("", "", "3000", lookupFail)
	if url != "http://localhost:3000/internal/v1/jobs/callback" {
		t.Fatalf("expected localhost with port 3000, got %s", url)
	}

	url = resolveCallbackURL("", "", "", lookupFail)
	if url != "http://localhost:8080/internal/v1/jobs/callback" {
		t.Fatalf("expected localhost default port 8080, got %s", url)
	}
}
