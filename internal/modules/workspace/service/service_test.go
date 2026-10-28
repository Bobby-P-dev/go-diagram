package service

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/Bobby-P-dev/go-diagram.git/internal/modules/workspace"
	"github.com/Bobby-P-dev/go-diagram.git/internal/modules/workspace/dto"
)

type accessReaderFunc func(context.Context, string, string, string) error

func (reader accessReaderFunc) CheckProjectAccess(ctx context.Context, projectID, userID, role string) error {
	return reader(ctx, projectID, userID, role)
}

type commentRepo struct {
	CommentRepository
	comment   workspace.DesignComment
	input     dto.CreateCommentRequest
	status    string
	calls     int
	err       error
	lookupErr error
}

func (repo *commentRepo) Create(projectID, nodeID, author, content string, posX, posY float64, metadata []byte) (*workspace.DesignComment, error) {
	repo.calls++
	repo.input = dto.CreateCommentRequest{NodeID: nodeID, Author: author, Content: content, PosX: posX, PosY: posY, Metadata: metadata}
	return &workspace.DesignComment{ProjectID: projectID, Author: author, Content: content}, repo.err
}

func (repo *commentRepo) GetByID(id string) (*workspace.DesignComment, error) {
	return &repo.comment, repo.lookupErr
}

func (repo *commentRepo) GetByProjectID(projectID string) ([]workspace.DesignComment, error) {
	repo.calls++
	return []workspace.DesignComment{repo.comment}, repo.err
}

func (repo *commentRepo) UpdateStatus(id, status string) error {
	repo.calls++
	repo.status = status
	return repo.err
}

func (repo *commentRepo) Delete(id string) error {
	repo.calls++
	return repo.err
}

type exportRepo struct {
	calls int
	err   error
}

func (repo *exportRepo) GetByProjectID(projectID string) ([]workspace.ProjectExport, error) {
	repo.calls++
	return []workspace.ProjectExport{{ProjectID: projectID}}, repo.err
}

func TestProjectOperationsEnforceAccess(t *testing.T) {
	operations := []struct {
		name string
		run  func(*WorkspaceService, *workspace.Actor) error
	}{
		{"list comments", func(svc *WorkspaceService, actor *workspace.Actor) error {
			_, err := svc.GetComments(context.Background(), actor, "project-a")
			return err
		}},
		{"create comment", func(svc *WorkspaceService, actor *workspace.Actor) error {
			_, err := svc.CreateComment(context.Background(), actor, "project-a", dto.CreateCommentRequest{Content: "Fix spacing"})
			return err
		}},
		{"update comment", func(svc *WorkspaceService, actor *workspace.Actor) error {
			_, err := svc.UpdateCommentStatus(context.Background(), actor, "comment-1", dto.UpdateCommentStatusRequest{})
			return err
		}},
		{"delete comment", func(svc *WorkspaceService, actor *workspace.Actor) error {
			return svc.DeleteComment(context.Background(), actor, "comment-1")
		}},
		{"list exports", func(svc *WorkspaceService, actor *workspace.Actor) error {
			_, err := svc.GetExports(context.Background(), actor, "project-a")
			return err
		}},
	}
	for _, operation := range operations {
		for _, scenario := range []struct {
			name          string
			actor         *workspace.Actor
			missingReader bool
			accessErr     error
			want          error
		}{
			{name: "anonymous", want: ErrUnauthenticated},
			{name: "foreign", actor: &workspace.Actor{ID: "user-b"}, accessErr: errors.New("denied"), want: ErrProjectNotFound},
			{name: "missing access reader", actor: &workspace.Actor{ID: "user-a"}, missingReader: true, want: ErrAccessUnavailable},
			{name: "owner", actor: &workspace.Actor{ID: "user-a", Role: "member"}},
		} {
			t.Run(operation.name+"/"+scenario.name, func(t *testing.T) {
				comments := &commentRepo{comment: workspace.DesignComment{ID: "comment-1", ProjectID: "project-a"}}
				exports := &exportRepo{}
				var reader workspace.ProjectAccessReader
				if !scenario.missingReader {
					reader = accessReaderFunc(func(ctx context.Context, projectID, userID, role string) error {
						if projectID != "project-a" || userID != scenario.actor.ID || role != scenario.actor.Role {
							t.Fatalf("unexpected access input: %s %s %s", projectID, userID, role)
						}
						return scenario.accessErr
					})
				}
				svc := NewWorkspaceService(reader, comments, nil, nil, exports)
				if err := operation.run(svc, scenario.actor); !errors.Is(err, scenario.want) {
					t.Fatalf("got %v, want %v", err, scenario.want)
				}
				calls := comments.calls + exports.calls
				if scenario.want != nil && calls != 0 {
					t.Fatalf("denied operation reached persistence %d times", calls)
				}
				if scenario.want == nil && calls != 1 {
					t.Fatalf("owner operation calls = %d", calls)
				}
			})
		}
	}
}

func TestCommentDefaultsValidationAndErrors(t *testing.T) {
	ctx := context.Background()
	actor := &workspace.Actor{ID: "user-a"}
	comments := &commentRepo{comment: workspace.DesignComment{ProjectID: "project-a"}}
	svc := NewWorkspaceService(accessReaderFunc(func(context.Context, string, string, string) error { return nil }), comments, nil, nil, nil)
	if _, err := svc.CreateComment(ctx, actor, "project-a", dto.CreateCommentRequest{}); !errors.Is(err, ErrContentRequired) || comments.calls != 0 {
		t.Fatalf("empty content: %v, calls %d", err, comments.calls)
	}
	input := dto.CreateCommentRequest{Content: "Fix spacing", NodeID: "node-1", PosX: 3, PosY: 4, Metadata: json.RawMessage(`{"tag":"a"}`)}
	if _, err := svc.CreateComment(ctx, actor, "project-a", input); err != nil {
		t.Fatal(err)
	}
	if comments.input.Author != "Designer" || comments.input.NodeID != input.NodeID || comments.input.PosX != input.PosX || comments.input.PosY != input.PosY || string(comments.input.Metadata) != string(input.Metadata) {
		t.Fatalf("input changed: %+v", comments.input)
	}
	status, err := svc.UpdateCommentStatus(ctx, actor, "comment-1", dto.UpdateCommentStatusRequest{})
	if err != nil || status != "resolved" || comments.status != status {
		t.Fatalf("status %s: %v", status, err)
	}
	status, err = svc.UpdateCommentStatus(ctx, actor, "comment-1", dto.UpdateCommentStatusRequest{Status: "open"})
	if err != nil || status != "open" {
		t.Fatalf("explicit status %s: %v", status, err)
	}
	storageErr := errors.New("storage unavailable")
	comments.err = storageErr
	if _, err := svc.CreateComment(ctx, actor, "project-a", input); !errors.Is(err, storageErr) {
		t.Fatal(err)
	}
	if _, err := svc.GetComments(ctx, actor, "project-a"); !errors.Is(err, storageErr) {
		t.Fatal(err)
	}
	if _, err := svc.UpdateCommentStatus(ctx, actor, "comment-1", dto.UpdateCommentStatusRequest{}); !errors.Is(err, storageErr) {
		t.Fatal(err)
	}
	if err := svc.DeleteComment(ctx, actor, "comment-1"); !errors.Is(err, storageErr) {
		t.Fatal(err)
	}
	comments.lookupErr = errors.New("not found")
	before := comments.calls
	if _, err := svc.UpdateCommentStatus(ctx, actor, "missing", dto.UpdateCommentStatusRequest{}); !errors.Is(err, ErrCommentNotFound) {
		t.Fatal(err)
	}
	if err := svc.DeleteComment(ctx, actor, "missing"); !errors.Is(err, ErrCommentNotFound) {
		t.Fatal(err)
	}
	if comments.calls != before {
		t.Fatal("missing comment mutated persistence")
	}
}

type preferencesRepo struct {
	err   error
	input dto.UpdateSettingsRequest
}

func (repo *preferencesRepo) GetAll() ([]workspace.DesignFoundation, error) {
	return []workspace.DesignFoundation{{ID: "foundation-1"}}, repo.err
}

func (repo *preferencesRepo) Get() (*workspace.WorkspaceSetting, error) {
	return &workspace.WorkspaceSetting{ID: "default"}, repo.err
}

func (repo *preferencesRepo) Update(mode, theme, model string, canvas json.RawMessage) (*workspace.WorkspaceSetting, error) {
	repo.input = dto.UpdateSettingsRequest{DefaultMode: mode, Theme: theme, PreferredAIModel: model, CanvasPreferences: canvas}
	return &workspace.WorkspaceSetting{ID: "default", Theme: theme}, repo.err
}

func TestPreferencesAndExportsDelegateWithoutChangingInputs(t *testing.T) {
	repo := &preferencesRepo{}
	exports := &exportRepo{}
	svc := NewWorkspaceService(accessReaderFunc(func(context.Context, string, string, string) error { return nil }), nil, repo, repo, exports)
	if found, err := svc.GetFoundations(); err != nil || len(found) != 1 {
		t.Fatalf("foundations: %v, %v", found, err)
	}
	if settings, err := svc.GetSettings(); err != nil || settings.ID != "default" {
		t.Fatalf("settings: %v, %v", settings, err)
	}
	input := dto.UpdateSettingsRequest{DefaultMode: "diagram", Theme: "light", PreferredAIModel: "custom", CanvasPreferences: json.RawMessage(`{"grid_size":8}`)}
	if _, err := svc.UpdateSettings(input); err != nil {
		t.Fatal(err)
	}
	if repo.input.DefaultMode != input.DefaultMode || repo.input.Theme != input.Theme || repo.input.PreferredAIModel != input.PreferredAIModel || string(repo.input.CanvasPreferences) != string(input.CanvasPreferences) {
		t.Fatalf("input changed: %+v", repo.input)
	}
	storageErr := errors.New("storage unavailable")
	repo.err, exports.err = storageErr, storageErr
	if _, err := svc.GetFoundations(); !errors.Is(err, storageErr) {
		t.Fatal(err)
	}
	if _, err := svc.GetSettings(); !errors.Is(err, storageErr) {
		t.Fatal(err)
	}
	if _, err := svc.UpdateSettings(input); !errors.Is(err, storageErr) {
		t.Fatal(err)
	}
	if _, err := svc.GetExports(context.Background(), &workspace.Actor{ID: "owner"}, "project-a"); !errors.Is(err, storageErr) {
		t.Fatal(err)
	}
}
