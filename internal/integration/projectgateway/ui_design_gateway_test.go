package projectgateway_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/Bobby-P-dev/go-diagram.git/internal/integration/projectgateway"
	"github.com/Bobby-P-dev/go-diagram.git/internal/modules/diagram"
	"github.com/Bobby-P-dev/go-diagram.git/internal/modules/uidesign"
)

type mockUIDesignStores struct {
	mockProjectRepo
	versionNumber int
	message       *diagram.ChatMessage
}

func (m *mockUIDesignStores) AppendMessage(projectID, role, content string, targetNodeIDs json.RawMessage) (*diagram.ChatMessage, error) {
	return m.message, nil
}

func (m *mockUIDesignStores) CreateVersion(projectID string, versionNumber int, summary string, nodes, edges json.RawMessage, triggerMessageID *string) (*diagram.DiagramVersion, error) {
	return &diagram.DiagramVersion{ID: "v-1", VersionNumber: versionNumber}, nil
}

func (m *mockUIDesignStores) GetLatestVersionNumber(projectID string) (int, error) {
	return m.versionNumber, nil
}

func TestUIDesignProjectGateway(t *testing.T) {
	stores := &mockUIDesignStores{
		mockProjectRepo: mockProjectRepo{
			project: &diagram.Project{ID: "proj-1", UserID: "user-1", Title: "UI Test"},
		},
		versionNumber: 3,
		message: &diagram.ChatMessage{
			ID: "msg-1", ProjectID: "proj-1", Role: "user", Content: "Make navbar",
		},
	}
	gateway := projectgateway.NewUIDesignProjectGateway(stores, stores, stores)

	t.Run("CheckProjectAccess success and cancel", func(t *testing.T) {
		if err := gateway.CheckProjectAccess(context.Background(), "proj-1", "user-1"); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		if err := gateway.CheckProjectAccess(ctx, "proj-1", "user-1"); !errors.Is(err, context.Canceled) {
			t.Fatalf("expected context.Canceled, got %v", err)
		}
	})

	t.Run("GetProjectScoped success and cancel", func(t *testing.T) {
		project, messages, err := gateway.GetProjectScoped(context.Background(), "proj-1", "user-1", "admin")
		if err != nil || project.ID != "proj-1" {
			t.Fatalf("unexpected result: project=%+v, messages=%d, err=%v", project, len(messages), err)
		}
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		if _, _, err := gateway.GetProjectScoped(ctx, "proj-1", "user-1", "admin"); !errors.Is(err, context.Canceled) {
			t.Fatalf("expected context.Canceled, got %v", err)
		}
	})

	t.Run("AppendMessage and Version success and cancel", func(t *testing.T) {
		msg, err := gateway.AppendMessage(context.Background(), "proj-1", "user", "hi", nil)
		if err != nil || msg.ID != "msg-1" {
			t.Fatalf("unexpected message: %+v, err=%v", msg, err)
		}
		vNum, err := gateway.GetLatestVersionNumber(context.Background(), "proj-1")
		if err != nil || vNum != 3 {
			t.Fatalf("expected version 3, got %d, err=%v", vNum, err)
		}
		err = gateway.CreateVersion(context.Background(), uidesign.CreateVersionInput{
			ProjectID:     "proj-1",
			VersionNumber: 4,
			ChangeSummary: "Added button",
		})
		if err != nil {
			t.Fatalf("unexpected version error: %v", err)
		}

		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		if _, err := gateway.AppendMessage(ctx, "proj-1", "user", "hi", nil); !errors.Is(err, context.Canceled) {
			t.Fatalf("expected context.Canceled, got %v", err)
		}
		if _, err := gateway.GetLatestVersionNumber(ctx, "proj-1"); !errors.Is(err, context.Canceled) {
			t.Fatalf("expected context.Canceled, got %v", err)
		}
		if err := gateway.CreateVersion(ctx, uidesign.CreateVersionInput{}); !errors.Is(err, context.Canceled) {
			t.Fatalf("expected context.Canceled, got %v", err)
		}
	})
}
