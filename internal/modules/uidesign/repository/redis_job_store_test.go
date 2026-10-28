package repository

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func TestRedisJobStoreOperations(t *testing.T) {
	server, err := miniredis.Run()
	if err != nil {
		t.Fatalf("failed to run miniredis: %v", err)
	}
	defer server.Close()

	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	defer client.Close()

	store := NewRedisJobStore(client)
	ctx := context.Background()

	status, err := store.GetStatus(ctx, "nonexistent-job")
	if err != nil {
		t.Fatalf("expected nil error on missing key, got %v", err)
	}
	if status != "" {
		t.Fatalf("expected empty status, got %q", status)
	}

	project, err := store.GetJobProject(ctx, "nonexistent-job")
	if err != nil {
		t.Fatalf("expected nil error on missing project key, got %v", err)
	}
	if project != "" {
		t.Fatalf("expected empty project, got %q", project)
	}

	if err := store.SetStatus(ctx, "job-123", "processing"); err != nil {
		t.Fatalf("failed to set status: %v", err)
	}
	status, err = store.GetStatus(ctx, "job-123")
	if err != nil {
		t.Fatalf("failed to get status: %v", err)
	}
	if status != "processing" {
		t.Fatalf("expected 'processing', got %q", status)
	}

	if !server.Exists("job:job-123:status") {
		t.Fatalf("expected key 'job:job-123:status' to exist")
	}
	if ttl := server.TTL("job:job-123:status"); ttl != 2*time.Hour {
		t.Fatalf("expected 2 hour TTL, got %v", ttl)
	}

	server.Set("job:job-123:project", "proj-abc")
	project, err = store.GetJobProject(ctx, "job-123")
	if err != nil {
		t.Fatalf("failed to get project: %v", err)
	}
	if project != "proj-abc" {
		t.Fatalf("expected 'proj-abc', got %q", project)
	}

	payload := []byte(`{"job_id":"job-123"}`)
	if err := store.Enqueue(ctx, payload); err != nil {
		t.Fatalf("failed to enqueue: %v", err)
	}

	items, err := client.LRange(ctx, "queue:ai_orchestration", 0, -1).Result()
	if err != nil {
		t.Fatalf("failed to inspect queue: %v", err)
	}
	if len(items) != 1 || items[0] != string(payload) {
		t.Fatalf("expected queued item %s, got %+v", string(payload), items)
	}

	canceledCtx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := store.SetStatus(canceledCtx, "job-cancel", "queued"); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled on SetStatus, got %v", err)
	}
	if _, err := store.GetStatus(canceledCtx, "job-cancel"); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled on GetStatus, got %v", err)
	}
	if err := store.Enqueue(canceledCtx, payload); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled on Enqueue, got %v", err)
	}
	if _, err := store.GetJobProject(canceledCtx, "job-cancel"); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled on GetJobProject, got %v", err)
	}
	if server.Exists("job:job-cancel:status") || !server.Exists("job:job-123:project") {
		t.Fatal("canceled operations must not change Redis state")
	}
	if err := client.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := store.GetStatus(ctx, "job-123"); !errors.Is(err, redis.ErrClosed) {
		t.Fatalf("storage error must not be normalized to missing status: %v", err)
	}
}
