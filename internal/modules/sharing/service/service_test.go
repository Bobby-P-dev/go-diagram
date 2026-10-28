package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Bobby-P-dev/go-diagram.git/internal/modules/identity"
	"github.com/Bobby-P-dev/go-diagram.git/internal/modules/sharing"
)

type mockRepo struct {
	findByProjectIDFunc    func(ctx context.Context, projectID string) (*sharing.Share, error)
	findActiveByTokenFunc  func(ctx context.Context, token string) (*sharing.Share, error)
	createFunc             func(ctx context.Context, input sharing.NewShare) (*sharing.Share, error)
	reactivateFunc         func(ctx context.Context, id, title string) (*sharing.Share, error)
	revokeFunc             func(ctx context.Context, projectID string) error
	incrementViewCountFunc func(ctx context.Context, shareID string) error
}

func (m *mockRepo) FindByProjectID(ctx context.Context, projectID string) (*sharing.Share, error) {
	if m.findByProjectIDFunc != nil {
		return m.findByProjectIDFunc(ctx, projectID)
	}
	return nil, nil
}

func (m *mockRepo) FindActiveByToken(ctx context.Context, token string) (*sharing.Share, error) {
	if m.findActiveByTokenFunc != nil {
		return m.findActiveByTokenFunc(ctx, token)
	}
	return nil, errors.New("not found")
}

func (m *mockRepo) Create(ctx context.Context, input sharing.NewShare) (*sharing.Share, error) {
	if m.createFunc != nil {
		return m.createFunc(ctx, input)
	}
	return nil, nil
}

func (m *mockRepo) Reactivate(ctx context.Context, id, title string) (*sharing.Share, error) {
	if m.reactivateFunc != nil {
		return m.reactivateFunc(ctx, id, title)
	}
	return nil, nil
}

func (m *mockRepo) Revoke(ctx context.Context, projectID string) error {
	if m.revokeFunc != nil {
		return m.revokeFunc(ctx, projectID)
	}
	return nil
}

func (m *mockRepo) IncrementViewCount(ctx context.Context, shareID string) error {
	if m.incrementViewCountFunc != nil {
		return m.incrementViewCountFunc(ctx, shareID)
	}
	return nil
}

type mockGateway struct {
	loadFunc  func(ctx context.Context, projectID string) (*sharing.Project, []sharing.Message, error)
	cloneFunc func(ctx context.Context, input sharing.CloneInput) (*sharing.Project, error)
}

func (m *mockGateway) LoadSharedProject(ctx context.Context, projectID string) (*sharing.Project, []sharing.Message, error) {
	if m.loadFunc != nil {
		return m.loadFunc(ctx, projectID)
	}
	return nil, nil, nil
}

func (m *mockGateway) CloneSharedProject(ctx context.Context, input sharing.CloneInput) (*sharing.Project, error) {
	if m.cloneFunc != nil {
		return m.cloneFunc(ctx, input)
	}
	return nil, nil
}

func TestShareService_CreateOrGetShare(t *testing.T) {
	now := time.Now()
	repo := &mockRepo{
		findByProjectIDFunc: func(ctx context.Context, projectID string) (*sharing.Share, error) {
			if projectID == "existing-proj" {
				return &sharing.Share{ID: "share-1", ProjectID: projectID, Title: "Old Title", IsActive: false}, nil
			}
			return nil, nil
		},
		reactivateFunc: func(ctx context.Context, id, title string) (*sharing.Share, error) {
			return &sharing.Share{ID: id, Title: title, IsActive: true, UpdatedAt: now}, nil
		},
		createFunc: func(ctx context.Context, input sharing.NewShare) (*sharing.Share, error) {
			return &sharing.Share{ID: "new-share", ShareToken: input.Token, Title: input.Title, IsActive: true}, nil
		},
	}
	svc := NewShareService(repo, nil, identity.MasterAdminID)

	share, err := svc.CreateOrGetShare(context.Background(), "existing-proj", "New Title")
	if err != nil || share.ID != "share-1" || share.Title != "New Title" || !share.IsActive {
		t.Fatalf("reactivate failed: %+v, %v", share, err)
	}

	share, err = svc.CreateOrGetShare(context.Background(), "brand-new-proj", "Brand New")
	if err != nil || share.ID != "new-share" || share.Title != "Brand New" {
		t.Fatalf("create failed: %+v, %v", share, err)
	}
}

func TestShareService_GetShareByToken(t *testing.T) {
	repo := &mockRepo{
		findActiveByTokenFunc: func(ctx context.Context, token string) (*sharing.Share, error) {
			if token == "valid-token" {
				return &sharing.Share{ID: "share-1", ShareToken: token, ProjectID: "proj-1", IsActive: true}, nil
			}
			return nil, errors.New("not found")
		},
	}
	gw := &mockGateway{
		loadFunc: func(ctx context.Context, projectID string) (*sharing.Project, []sharing.Message, error) {
			return &sharing.Project{ID: projectID, Title: "Flow"}, []sharing.Message{{ID: "msg-1"}}, nil
		},
	}
	svc := NewShareService(repo, gw, identity.MasterAdminID)

	share, proj, msgs, err := svc.GetShareByToken(context.Background(), "valid-token")
	if err != nil || share == nil || proj == nil || len(msgs) != 1 {
		t.Fatalf("failed: share=%+v, proj=%+v, msgs=%d, err=%v", share, proj, len(msgs), err)
	}

	_, _, _, err = svc.GetShareByToken(context.Background(), "invalid")
	if err == nil {
		t.Fatal("expected error for invalid token")
	}
}

func TestShareService_ForkSharedProjectWithUser(t *testing.T) {
	repo := &mockRepo{
		findActiveByTokenFunc: func(ctx context.Context, token string) (*sharing.Share, error) {
			return &sharing.Share{ID: "share-1", ShareToken: token, ProjectID: "proj-1", IsActive: true}, nil
		},
	}
	var capturedClone sharing.CloneInput
	gw := &mockGateway{
		loadFunc: func(ctx context.Context, projectID string) (*sharing.Project, []sharing.Message, error) {
			return &sharing.Project{ID: projectID, Title: "Original"}, []sharing.Message{{ID: "m1"}}, nil
		},
		cloneFunc: func(ctx context.Context, input sharing.CloneInput) (*sharing.Project, error) {
			capturedClone = input
			return &sharing.Project{ID: "cloned-1", UserID: input.UserID, Title: input.Title}, nil
		},
	}
	svc := NewShareService(repo, gw, identity.MasterAdminID)

	cloned, err := svc.ForkSharedProjectWithUser(context.Background(), "token", "Custom Title", "user-123")
	if err != nil || cloned.ID != "cloned-1" {
		t.Fatalf("fork error: %v", err)
	}
	if capturedClone.Title != "Custom Title" || capturedClone.UserID != "user-123" {
		t.Fatalf("unexpected captured clone input: %+v", capturedClone)
	}

	cloned, err = svc.ForkSharedProjectWithUser(context.Background(), "token", "", "")
	if err != nil {
		t.Fatalf("fork error: %v", err)
	}
	if capturedClone.Title != "Original (Shared Copy)" || capturedClone.UserID != identity.MasterAdminID {
		t.Fatalf("unexpected fallback: title=%q, user=%q", capturedClone.Title, capturedClone.UserID)
	}
}

func TestSharingServicePreservesContextAndBoundsBackgroundViews(t *testing.T) {
	type traceKey struct{}
	ctx, cancel := context.WithTimeout(context.WithValue(context.Background(), traceKey{}, "trace-1"), time.Minute)
	defer cancel()
	cancel()
	check := func(actual context.Context) {
		t.Helper()
		if actual != ctx || !errors.Is(actual.Err(), context.Canceled) {
			t.Fatal("request context lost")
		}
	}
	views := make(chan bool, 2)
	var calls []string
	repo := &mockRepo{
		findByProjectIDFunc: func(actual context.Context, id string) (*sharing.Share, error) {
			check(actual)
			calls = append(calls, "find")
			return nil, nil
		},
		createFunc: func(actual context.Context, input sharing.NewShare) (*sharing.Share, error) {
			check(actual)
			calls = append(calls, "create")
			if len(input.Token) != 32 || input.Title != "Shared Project" {
				t.Errorf("invalid defaults or token: %+v", input)
			}
			return &sharing.Share{ID: "share"}, nil
		},
		findActiveByTokenFunc: func(actual context.Context, token string) (*sharing.Share, error) {
			check(actual)
			calls = append(calls, "token")
			if token != "token" {
				t.Error(token)
			}
			return &sharing.Share{ID: "share", ProjectID: "source"}, nil
		},
		revokeFunc: func(actual context.Context, id string) error {
			check(actual)
			calls = append(calls, "revoke")
			return nil
		},
		incrementViewCountFunc: func(actual context.Context, id string) error {
			deadline, bounded := actual.Deadline()
			views <- actual.Err() == nil && bounded && time.Until(deadline) <= 5*time.Second && actual.Value(traceKey{}) == "trace-1" && id == "share"
			return errors.New("best effort failure")
		},
	}
	gateway := &mockGateway{
		loadFunc: func(actual context.Context, id string) (*sharing.Project, []sharing.Message, error) {
			check(actual)
			calls = append(calls, "load")
			return &sharing.Project{ID: id, Title: "Source"}, []sharing.Message{{Content: "Keep history"}}, nil
		},
		cloneFunc: func(actual context.Context, input sharing.CloneInput) (*sharing.Project, error) {
			check(actual)
			calls = append(calls, "clone")
			if input.Title != "Copy" || input.UserID != "owner" || len(input.Messages) != 1 {
				t.Error(input)
			}
			return &sharing.Project{ID: "clone"}, nil
		},
	}
	svc := NewShareService(repo, gateway, identity.MasterAdminID)
	if _, err := svc.CreateOrGetShare(ctx, "source", " "); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.GetShareByProjectID(ctx, "source"); err != nil {
		t.Fatal(err)
	}
	if _, _, _, err := svc.GetShareByToken(ctx, " token "); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.ForkSharedProjectWithUser(ctx, "token", " Copy ", "owner"); err != nil {
		t.Fatal(err)
	}
	if err := svc.RevokeShare(ctx, "source"); err != nil {
		t.Fatal(err)
	}
	if len(calls) != 9 {
		t.Fatal(calls)
	}
	for counter := 0; counter < 2; counter++ {
		select {
		case valid := <-views:
			if !valid {
				t.Fatal("background counter lost trace, deadline, or isolation from request cancellation")
			}
		case <-time.After(time.Second):
			t.Fatal("background counter not completed")
		}
	}
}
