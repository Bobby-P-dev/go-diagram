package integration

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

type mockRoundTripper struct {
	fn func(req *http.Request) (*http.Response, error)
}

func (transport *mockRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	return transport.fn(req)
}

func TestHTTPSidecarDispatcher_Success(t *testing.T) {
	var capturedPath string
	var capturedKey string
	var capturedContentType string
	var capturedBody []byte
	bodyClosed := false

	transport := &mockRoundTripper{
		fn: func(req *http.Request) (*http.Response, error) {
			if req.Method != http.MethodPost {
				t.Fatalf("expected POST, got %s", req.Method)
			}
			capturedPath = req.URL.Path
			capturedKey = req.Header.Get("X-Internal-Service-Key")
			capturedContentType = req.Header.Get("Content-Type")
			capturedBody, _ = io.ReadAll(req.Body)

			closer := io.NopCloser(bytes.NewReader([]byte(`{"status":"ok"}`)))
			return &http.Response{
				StatusCode: http.StatusOK,
				Body: &trackingReadCloser{
					ReadCloser: closer,
					onClose: func() {
						bodyClosed = true
					},
				},
				Header: make(http.Header),
			}, nil
		},
	}

	client := &http.Client{Transport: transport}
	dispatcher := NewHTTPSidecarDispatcher("http://orchestration-service:8000/", "secret-token", client)

	payload := []byte(`{"job_id":"test-1"}`)
	err := dispatcher.Dispatch(context.Background(), payload)
	if err != nil {
		t.Fatalf("unexpected dispatch error: %v", err)
	}

	if capturedPath != "/api/v1/crews/ui-design" {
		t.Fatalf("expected path /api/v1/crews/ui-design, got %q", capturedPath)
	}
	if capturedKey != "secret-token" {
		t.Fatalf("expected service key 'secret-token', got %q", capturedKey)
	}
	if capturedContentType != "application/json" {
		t.Fatalf("expected Content-Type application/json, got %q", capturedContentType)
	}
	if string(capturedBody) != string(payload) {
		t.Fatalf("expected body %s, got %s", string(payload), string(capturedBody))
	}
	if !bodyClosed {
		t.Fatal("expected response body to be closed")
	}
}

func TestHTTPSidecarDispatcher_Non2xxReturnsError(t *testing.T) {
	transport := &mockRoundTripper{
		fn: func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusInternalServerError,
				Body:       io.NopCloser(strings.NewReader("server failure")),
				Header:     make(http.Header),
			}, nil
		},
	}

	client := &http.Client{Transport: transport}
	dispatcher := NewHTTPSidecarDispatcher("http://orchestrator", "key", client)

	err := dispatcher.Dispatch(context.Background(), []byte(`{}`))
	if err == nil {
		t.Fatal("expected error on status 500, got nil")
	}
	if !strings.Contains(err.Error(), "500") {
		t.Fatalf("expected error to mention 500, got %v", err)
	}
}

func TestHTTPSidecarDispatcher_ContextCanceled(t *testing.T) {
	transport := &mockRoundTripper{
		fn: func(req *http.Request) (*http.Response, error) {
			<-req.Context().Done()
			return nil, req.Context().Err()
		},
	}

	client := &http.Client{Transport: transport}
	dispatcher := NewHTTPSidecarDispatcher("http://orchestrator", "key", client)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	cancel()

	err := dispatcher.Dispatch(ctx, []byte(`{}`))
	if err == nil {
		t.Fatal("expected error on canceled context, got nil")
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context cancellation error, got %v", err)
	}
}

func TestHTTPSidecarDispatcherDefaultTimeoutAndTransportFailure(t *testing.T) {
	defaultDispatcher := NewHTTPSidecarDispatcher("http://orchestrator", "key", nil)
	if defaultDispatcher.client.Timeout != 300*time.Second {
		t.Fatalf("changed default timeout: %v", defaultDispatcher.client.Timeout)
	}
	failure := errors.New("connection refused")
	client := &http.Client{Transport: &mockRoundTripper{fn: func(*http.Request) (*http.Response, error) {
		return nil, failure
	}}}
	if err := NewHTTPSidecarDispatcher("http://orchestrator", "key", client).Dispatch(context.Background(), []byte(`{}`)); !errors.Is(err, failure) {
		t.Fatalf("lost transport error: %v", err)
	}
	if err := NewHTTPSidecarDispatcher(":invalid", "key", client).Dispatch(context.Background(), []byte(`{}`)); err == nil {
		t.Fatal("invalid sidecar URL must fail")
	}
}

type trackingReadCloser struct {
	io.ReadCloser
	onClose func()
}

func (body *trackingReadCloser) Close() error {
	if body.onClose != nil {
		body.onClose()
	}
	return body.ReadCloser.Close()
}
