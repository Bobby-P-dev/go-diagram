package integration

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"
)

type HTTPSidecarDispatcher struct {
	url        string
	serviceKey string
	client     *http.Client
}

func NewHTTPSidecarDispatcher(baseURL, serviceKey string, client *http.Client) *HTTPSidecarDispatcher {
	if client == nil {
		client = &http.Client{Timeout: 300 * time.Second}
	}
	return &HTTPSidecarDispatcher{
		url:        strings.TrimSuffix(baseURL, "/") + "/api/v1/crews/ui-design",
		serviceKey: serviceKey,
		client:     client,
	}
}

func (dispatcher *HTTPSidecarDispatcher) Dispatch(ctx context.Context, payload []byte) error {
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, dispatcher.url, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("create sidecar request: %w", err)
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-Internal-Service-Key", dispatcher.serviceKey)
	response, err := dispatcher.client.Do(request)
	if err != nil {
		return fmt.Errorf("dispatch sidecar request: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return fmt.Errorf("sidecar responded with status %d", response.StatusCode)
	}
	return nil
}
