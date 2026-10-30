package service

import (
	"context"
	"errors"
	"testing"

	"github.com/Bobby-P-dev/go-diagram.git/internal/modules/generation"
)

type fakeRunRepo struct {
	findAcceptedFn func(context.Context, generation.Scope, generation.RunWrite) (*generation.Run, error)
	createFn       func(context.Context, generation.Scope, generation.StartInput, *generation.Manifest, generation.RunWrite) (*generation.Run, error)
	getFn          func(context.Context, generation.Scope, string) (*generation.Run, error)
	claimFn        func(context.Context, generation.Scope, string) (*generation.Run, bool, error)
	cancelFn       func(context.Context, generation.Scope, string) (*generation.Run, error)
	finishFn       func(context.Context, generation.Scope, string, string, string, string) error
	commitFn       func(context.Context, generation.Scope, string, *generation.ArtifactCandidate) (*generation.Run, error)
	recoverFn      func(context.Context) error
	queuedFn       func(context.Context) ([]generation.Run, error)
}

func (fake *fakeRunRepo) FindAccepted(ctx context.Context, scope generation.Scope, write generation.RunWrite) (*generation.Run, error) {
	if fake.findAcceptedFn != nil {
		return fake.findAcceptedFn(ctx, scope, write)
	}
	return nil, nil
}

func (fake *fakeRunRepo) Create(ctx context.Context, scope generation.Scope, input generation.StartInput, manifest *generation.Manifest, write generation.RunWrite) (*generation.Run, error) {
	if fake.createFn != nil {
		return fake.createFn(ctx, scope, input, manifest, write)
	}
	return &generation.Run{ID: "run-created", Status: "queued"}, nil
}

func (fake *fakeRunRepo) Get(ctx context.Context, scope generation.Scope, id string) (*generation.Run, error) {
	if fake.getFn != nil {
		return fake.getFn(ctx, scope, id)
	}
	return &generation.Run{ID: id, Status: "running"}, nil
}

func (fake *fakeRunRepo) Claim(ctx context.Context, scope generation.Scope, id string) (*generation.Run, bool, error) {
	if fake.claimFn != nil {
		return fake.claimFn(ctx, scope, id)
	}
	return &generation.Run{ID: id, Status: "running"}, true, nil
}

func (fake *fakeRunRepo) Cancel(ctx context.Context, scope generation.Scope, id string) (*generation.Run, error) {
	if fake.cancelFn != nil {
		return fake.cancelFn(ctx, scope, id)
	}
	return &generation.Run{ID: id, Status: "canceled"}, nil
}

func (fake *fakeRunRepo) Finish(ctx context.Context, scope generation.Scope, id, status, code, msg string) error {
	if fake.finishFn != nil {
		return fake.finishFn(ctx, scope, id, status, code, msg)
	}
	return nil
}

func (fake *fakeRunRepo) Commit(ctx context.Context, scope generation.Scope, id string, candidate *generation.ArtifactCandidate) (*generation.Run, error) {
	if fake.commitFn != nil {
		return fake.commitFn(ctx, scope, id, candidate)
	}
	return &generation.Run{ID: id, Status: "succeeded"}, nil
}

func (fake *fakeRunRepo) RecoverInterrupted(ctx context.Context) error {
	if fake.recoverFn != nil {
		return fake.recoverFn(ctx)
	}
	return nil
}

func (fake *fakeRunRepo) Queued(ctx context.Context) ([]generation.Run, error) {
	if fake.queuedFn != nil {
		return fake.queuedFn(ctx)
	}
	return nil, nil
}

type fakeEngine struct {
	generateFn func(context.Context, generation.DocumentInput, generation.ProgressSink) (*generation.DocumentCandidate, error)
}

func (fake *fakeEngine) GenerateDocument(ctx context.Context, input generation.DocumentInput, sink generation.ProgressSink) (*generation.DocumentCandidate, error) {
	if fake.generateFn != nil {
		return fake.generateFn(ctx, input, sink)
	}
	return &generation.DocumentCandidate{
		Type:             input.Type,
		Title:            input.Title,
		DocumentContent:  "# Generated",
		ChangeSummary:    "Initial draft",
		AssistantMessage: "Here is your document.",
		Format:           "markdown",
		SchemaVersion:    1,
	}, nil
}

type testResolver struct {
	resolveFn func(context.Context, generation.Scope, generation.ResolveInput) (*generation.Manifest, error)
}

func (record *testResolver) Resolve(ctx context.Context, scope generation.Scope, input generation.ResolveInput) (*generation.Manifest, error) {
	if record.resolveFn != nil {
		return record.resolveFn(ctx, scope, input)
	}
	return &generation.Manifest{
		SchemaVersion: 1,
		Mode:          "explicit",
		Project:       generation.ProjectContext{ID: scope.ProjectID, Title: "P"},
		ThreadID:      scope.ThreadID,
	}, nil
}

func TestRunCoordinator_Start(test *testing.T) {
	validScope := generation.Scope{
		UserID:    "11111111-1111-1111-1111-111111111111",
		ProjectID: "22222222-2222-2222-2222-222222222222",
		ThreadID:  "33333333-3333-3333-3333-333333333333",
	}
	validInput := generation.StartInput{
		TriggerMessageID: "44444444-4444-4444-4444-444444444444",
		OutputType:       "prd",
		OutputTitle:      "PRD Title",
		Context:          generation.ResolveInput{Mode: "explicit"},
	}

	test.Run("unauthenticated", func(test *testing.T) {
		coord := NewRunCoordinator(&fakeRunRepo{}, &testResolver{}, &fakeEngine{}, nil)
		_, err := coord.Start(context.Background(), generation.Scope{}, validInput, "key-1")
		if !errors.Is(err, generation.ErrUnauthenticated) {
			test.Fatalf("expected ErrUnauthenticated, got %v", err)
		}
	})

	test.Run("missing idempotency key", func(test *testing.T) {
		coord := NewRunCoordinator(&fakeRunRepo{}, &testResolver{}, &fakeEngine{}, nil)
		_, err := coord.Start(context.Background(), validScope, validInput, "")
		if !errors.Is(err, generation.ErrIdempotencyRequired) {
			test.Fatalf("expected ErrIdempotencyRequired, got %v", err)
		}
	})

	test.Run("unsupported output type", func(test *testing.T) {
		coord := NewRunCoordinator(&fakeRunRepo{}, &testResolver{}, &fakeEngine{}, nil)
		input := validInput
		input.OutputType = "invalid-type"
		_, err := coord.Start(context.Background(), validScope, input, "key-1")
		if !errors.Is(err, generation.ErrUnsupportedType) {
			test.Fatalf("expected ErrUnsupportedType, got %v", err)
		}
	})

	test.Run("target without expected revision", func(test *testing.T) {
		coord := NewRunCoordinator(&fakeRunRepo{}, &testResolver{}, &fakeEngine{}, nil)
		input := validInput
		input.TargetArtifactID = "55555555-5555-5555-5555-555555555555"
		input.TargetExpectedRevisionID = ""
		_, err := coord.Start(context.Background(), validScope, input, "key-1")
		if !errors.Is(err, generation.ErrInvalidInput) {
			test.Fatalf("expected ErrInvalidInput, got %v", err)
		}
	})

	test.Run("cached idempotency replay skips resolve", func(test *testing.T) {
		resolveCalled := false
		res := &testResolver{
			resolveFn: func(_ context.Context, _ generation.Scope, _ generation.ResolveInput) (*generation.Manifest, error) {
				resolveCalled = true
				return nil, nil
			},
		}
		repo := &fakeRunRepo{
			findAcceptedFn: func(_ context.Context, _ generation.Scope, _ generation.RunWrite) (*generation.Run, error) {
				return &generation.Run{ID: "run-cached", Status: "running"}, nil
			},
		}
		coord := NewRunCoordinator(repo, res, &fakeEngine{}, nil)
		run, err := coord.Start(context.Background(), validScope, validInput, "key-1")
		if err != nil {
			test.Fatalf("unexpected error: %v", err)
		}
		if run.ID != "run-cached" {
			test.Fatalf("expected run-cached, got %s", run.ID)
		}
		if resolveCalled {
			test.Fatal("resolver should not be called when idempotency hit")
		}
	})

	test.Run("context budget exceeded propagates", func(test *testing.T) {
		res := &testResolver{
			resolveFn: func(_ context.Context, _ generation.Scope, _ generation.ResolveInput) (*generation.Manifest, error) {
				return nil, generation.ErrBudgetExceeded
			},
		}
		coord := NewRunCoordinator(&fakeRunRepo{}, res, &fakeEngine{}, nil)
		_, err := coord.Start(context.Background(), validScope, validInput, "key-1")
		if !errors.Is(err, generation.ErrBudgetExceeded) {
			test.Fatalf("expected ErrBudgetExceeded, got %v", err)
		}
	})

	test.Run("successful start creates run", func(test *testing.T) {
		created := false
		repo := &fakeRunRepo{
			createFn: func(_ context.Context, scope generation.Scope, input generation.StartInput, manifest *generation.Manifest, write generation.RunWrite) (*generation.Run, error) {
				created = true
				return &generation.Run{ID: "run-new", Status: "queued", Manifest: manifest}, nil
			},
		}
		coord := NewRunCoordinator(repo, &testResolver{}, &fakeEngine{}, nil)
		run, err := coord.Start(context.Background(), validScope, validInput, "key-1")
		if err != nil {
			test.Fatalf("unexpected error: %v", err)
		}
		if !created || run.ID != "run-new" {
			test.Fatalf("expected run-new, got %v", run)
		}
	})
}

func TestRunCoordinator_GetAndCancel(test *testing.T) {
	validScope := generation.Scope{
		UserID:    "11111111-1111-1111-1111-111111111111",
		ProjectID: "22222222-2222-2222-2222-222222222222",
	}
	test.Run("get valid", func(test *testing.T) {
		repo := &fakeRunRepo{
			getFn: func(_ context.Context, _ generation.Scope, id string) (*generation.Run, error) {
				return &generation.Run{ID: id, Status: "running"}, nil
			},
		}
		coord := NewRunCoordinator(repo, &testResolver{}, &fakeEngine{}, nil)
		run, err := coord.Get(context.Background(), validScope, "33333333-3333-3333-3333-333333333333")
		if err != nil || run.ID != "33333333-3333-3333-3333-333333333333" {
			test.Fatalf("expected run, got %v, %v", run, err)
		}
	})

	test.Run("get invalid id", func(test *testing.T) {
		coord := NewRunCoordinator(&fakeRunRepo{}, &testResolver{}, &fakeEngine{}, nil)
		_, err := coord.Get(context.Background(), validScope, "invalid-uuid")
		if !errors.Is(err, generation.ErrInvalidInput) {
			test.Fatalf("expected ErrInvalidInput, got %v", err)
		}
	})

	test.Run("cancel terminal returns conflict", func(test *testing.T) {
		repo := &fakeRunRepo{
			cancelFn: func(_ context.Context, _ generation.Scope, _ string) (*generation.Run, error) {
				return nil, generation.ErrRunTerminal
			},
		}
		coord := NewRunCoordinator(repo, &testResolver{}, &fakeEngine{}, nil)
		_, err := coord.Cancel(context.Background(), validScope, "33333333-3333-3333-3333-333333333333")
		if !errors.Is(err, generation.ErrRunTerminal) {
			test.Fatalf("expected ErrRunTerminal, got %v", err)
		}
	})
}

func TestRunCoordinator_ExecuteWorker(test *testing.T) {
	queuedRun := generation.Run{
		ID:          "run-100",
		ProjectID:   "22222222-2222-2222-2222-222222222222",
		ThreadID:    "33333333-3333-3333-3333-333333333333",
		UserID:      "11111111-1111-1111-1111-111111111111",
		OutputType:  "prd",
		OutputTitle: "Title",
		Prompt:      "Write a PRD",
		Manifest:    &generation.Manifest{SchemaVersion: 1},
	}

	test.Run("successful claim, generate and commit", func(test *testing.T) {
		claimed := false
		committed := false
		repo := &fakeRunRepo{
			claimFn: func(_ context.Context, _ generation.Scope, id string) (*generation.Run, bool, error) {
				claimed = true
				record := queuedRun
				record.Status = "running"
				return &record, true, nil
			},
			commitFn: func(_ context.Context, _ generation.Scope, id string, candidate *generation.ArtifactCandidate) (*generation.Run, error) {
				committed = true
				record := queuedRun
				record.Status = "succeeded"
				return &record, nil
			},
		}
		coord := NewRunCoordinator(repo, &testResolver{}, &fakeEngine{}, nil)
		coord.execute(context.Background(), queuedRun)
		if !claimed || !committed {
			test.Fatalf("expected claimed=%v committed=%v", claimed, committed)
		}
	})

	test.Run("engine failure finishes run as failed", func(test *testing.T) {
		finishedCode := ""
		repo := &fakeRunRepo{
			claimFn: func(_ context.Context, _ generation.Scope, id string) (*generation.Run, bool, error) {
				record := queuedRun
				record.Status = "running"
				return &record, true, nil
			},
			finishFn: func(_ context.Context, _ generation.Scope, id, status, code, msg string) error {
				finishedCode = code
				return nil
			},
		}
		eng := &fakeEngine{
			generateFn: func(_ context.Context, _ generation.DocumentInput, _ generation.ProgressSink) (*generation.DocumentCandidate, error) {
				return nil, generation.ErrRepairFailed
			},
		}
		coord := NewRunCoordinator(repo, &testResolver{}, eng, nil)
		coord.execute(context.Background(), queuedRun)
		if finishedCode != "REPAIR_FAILED" {
			test.Fatalf("expected finish with REPAIR_FAILED, got %s", finishedCode)
		}
	})
}
