package redis

import (
	"context"
	"testing"
)

func TestConnectInvalidURL(test *testing.T) {
	_, err := Connect(context.Background(), "invalid-url-schema://localhost")
	if err == nil {
		test.Errorf("expected error for invalid redis url, got nil")
	}
}

func TestConnectUnreachable(test *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Pre-canceled context to ensure immediate failure without waiting for timeout
	_, err := Connect(ctx, "redis://127.0.0.1:59999/0")
	if err == nil {
		test.Errorf("expected connection error for unreachable redis, got nil")
	}
}

func TestClose(test *testing.T) {
	Close(nil) // Should not panic
}
