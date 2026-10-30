package service

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/Bobby-P-dev/go-diagram.git/internal/modules/generation"
)

type RunCoordinator struct {
	repository    generation.RunRepository
	resolver      generation.ContextResolver
	engine        generation.DocumentEngine
	diagramEngine generation.DiagramEngine
}

var _ generation.RunService = (*RunCoordinator)(nil)

func NewRunCoordinator(repository generation.RunRepository, resolver generation.ContextResolver, engine generation.DocumentEngine, diagramEngine generation.DiagramEngine) *RunCoordinator {
	return &RunCoordinator{repository: repository, resolver: resolver, engine: engine, diagramEngine: diagramEngine}
}

func (coordinator *RunCoordinator) Start(ctx context.Context, scope generation.Scope, input generation.StartInput, key string) (*generation.Run, error) {
	if scope.UserID == "" {
		return nil, generation.ErrUnauthenticated
	}
	if strings.TrimSpace(key) == "" {
		return nil, generation.ErrIdempotencyRequired
	}
	if len(key) > 128 || !utf8.ValidString(key) || strings.ContainsAny(key, "\x00\r\n") {
		return nil, generation.ErrInvalidInput
	}
	input.Context.Sources = append([]generation.RevisionRef(nil), input.Context.Sources...)
	input.Context.ThreadIDs = append([]string(nil), input.Context.ThreadIDs...)
	input.Context.ExcludedArtifactIDs = append([]string(nil), input.Context.ExcludedArtifactIDs...)
	if input.Mutation != nil {
		copyMutation := *input.Mutation
		copyMutation.NodeIDs = append([]string{}, input.Mutation.NodeIDs...)
		copyMutation.EdgeIDs = append([]string{}, input.Mutation.EdgeIDs...)
		if input.Mutation.Validate() != nil || input.TargetArtifactID == "" || (input.OutputType != "erd" && input.OutputType != "flowchart" && input.OutputType != "sequence") {
			return nil, generation.ErrInvalidInput
		}
		input.Mutation = &copyMutation
	}
	if err := normalize(&scope, &input.Context); err != nil {
		return nil, err
	}
	input.TriggerMessageID = strings.ToLower(input.TriggerMessageID)
	input.TargetArtifactID = strings.ToLower(input.TargetArtifactID)
	input.TargetExpectedRevisionID = strings.ToLower(input.TargetExpectedRevisionID)
	input.OutputTitle = strings.TrimSpace(input.OutputTitle)
	if !validID(input.TriggerMessageID) || !utf8.ValidString(input.OutputTitle) || strings.ContainsRune(input.OutputTitle, '\x00') || utf8.RuneCountInString(input.OutputTitle) > 255 {
		return nil, generation.ErrInvalidInput
	}
	if _, err := generation.OutputFormat(input.OutputType); err != nil {
		return nil, err
	}
	if (input.TargetArtifactID == "") != (input.TargetExpectedRevisionID == "") {
		return nil, generation.ErrInvalidInput
	}
	if input.TargetArtifactID != "" && (!validID(input.TargetArtifactID) || !validID(input.TargetExpectedRevisionID)) {
		return nil, generation.ErrInvalidInput
	}
	data, err := json.Marshal(input)
	if err != nil {
		return nil, generation.ErrInvalidInput
	}
	write := generation.RunWrite{Key: key, RequestHash: fmt.Sprintf("%x", sha256.Sum256(data))}
	cached, err := coordinator.repository.FindAccepted(ctx, scope, write)
	if err != nil || cached != nil {
		return cached, err
	}
	manifest, err := coordinator.resolver.Resolve(ctx, scope, input.Context)
	if err != nil {
		return nil, err
	}
	if manifest == nil || manifest.Project.ID != scope.ProjectID || manifest.ThreadID != scope.ThreadID {
		return nil, generation.ErrInvalidInput
	}
	manifest.PreviewOnly = false
	return coordinator.repository.Create(ctx, scope, input, manifest, write)
}

func normalizeRunScope(scope *generation.Scope, id *string) error {
	if scope.UserID == "" {
		return generation.ErrUnauthenticated
	}
	scope.UserID = strings.ToLower(scope.UserID)
	scope.ProjectID = strings.ToLower(scope.ProjectID)
	*id = strings.ToLower(*id)
	if !validID(scope.UserID) || !validID(scope.ProjectID) || !validID(*id) {
		return generation.ErrInvalidInput
	}
	return nil
}

func (coordinator *RunCoordinator) Get(ctx context.Context, scope generation.Scope, id string) (*generation.Run, error) {
	if err := normalizeRunScope(&scope, &id); err != nil {
		return nil, err
	}
	return coordinator.repository.Get(ctx, scope, id)
}

func (coordinator *RunCoordinator) Cancel(ctx context.Context, scope generation.Scope, id string) (*generation.Run, error) {
	if err := normalizeRunScope(&scope, &id); err != nil {
		return nil, err
	}
	return coordinator.repository.Cancel(ctx, scope, id)
}

func (coordinator *RunCoordinator) Run(ctx context.Context) {
	var workers sync.WaitGroup
	for index := 0; index < 2; index++ {
		workers.Add(1)
		go func() { defer workers.Done(); coordinator.work(ctx) }()
	}
	workers.Wait()
}

func (coordinator *RunCoordinator) work(ctx context.Context) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for ctx.Err() == nil {
		queryCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		err := coordinator.repository.RecoverInterrupted(queryCtx)
		var queued []generation.Run
		if err == nil {
			queued, err = coordinator.repository.Queued(queryCtx)
		}
		cancel()
		if err != nil && ctx.Err() == nil {
			log.Printf("generation queue poll failed: %v", err)
		}
		for _, run := range queued {
			if ctx.Err() != nil {
				return
			}
			coordinator.execute(ctx, run)
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (coordinator *RunCoordinator) execute(workerCtx context.Context, queued generation.Run) {
	scope := generation.Scope{UserID: queued.UserID, ProjectID: queued.ProjectID, ThreadID: queued.ThreadID}
	claimCtx, claimCancel := context.WithTimeout(workerCtx, 10*time.Second)
	run, claimed, err := coordinator.repository.Claim(claimCtx, scope, queued.ID)
	claimCancel()
	if err != nil {
		log.Printf("generation claim %s failed: %v", queued.ID, err)
		return
	}
	if !claimed {
		return
	}
	execCtx, cancel := context.WithTimeout(workerCtx, 4*time.Minute)
	monitorDone := make(chan struct{})
	go coordinator.monitorCancellation(execCtx, cancel, scope, run.ID, monitorDone)
	defer func() { cancel(); <-monitorDone }()
	defer func() {
		if panicValue := recover(); panicValue != nil {
			log.Printf("generation run %s panicked", run.ID)
			coordinator.fail(scope, run.ID, "WORKER_FAILED", "Generation worker failed")
		}
	}()
	candidate, err := coordinator.generate(execCtx, run)
	if execCtx.Err() != nil {
		coordinator.fail(scope, run.ID, "GENERATION_TIMEOUT", "Generation interrupted or exceeded its time limit")
		return
	}
	if err != nil {
		code, message := "GENERATION_FAILED", "Provider failed to produce an artifact"
		switch {
		case errors.Is(err, generation.ErrRepairFailed), errors.Is(err, generation.ErrDiagramRepairFailed):
			code, message = "REPAIR_FAILED", "Provider output remained invalid after repair"
		case errors.Is(err, generation.ErrDocumentTooLarge):
			code, message = "DOCUMENT_TOO_LARGE", "Document exceeded the size limit"
		case errors.Is(err, generation.ErrDiagramTooLarge):
			code, message = "DIAGRAM_TOO_LARGE", "Diagram exceeded the size limit"
		case errors.Is(err, generation.ErrEngineUnavailable):
			code, message = "ENGINE_UNAVAILABLE", "Generation engine is unavailable"
		}
		log.Printf("generation run %s failed: %v", run.ID, err)
		coordinator.fail(scope, run.ID, code, message)
		return
	}
	commitCtx, commitCancel := context.WithTimeout(workerCtx, 15*time.Second)
	_, err = coordinator.repository.Commit(commitCtx, scope, run.ID, candidate)
	commitCancel()
	if err != nil {
		code, message := "COMMIT_FAILED", "Unable to commit generated artifact"
		if errors.Is(err, generation.ErrRevisionConflict) {
			code, message = "REVISION_CONFLICT", "Artifact changed; retry against the latest revision"
		}
		log.Printf("generation commit %s failed: %v", run.ID, err)
		coordinator.fail(scope, run.ID, code, message)
	}
}

func (coordinator *RunCoordinator) fail(scope generation.Scope, id, code, message string) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := coordinator.repository.Finish(ctx, scope, id, "failed", code, message); err != nil {
		log.Printf("generation finish %s failed: %v", id, err)
	}
}

func (coordinator *RunCoordinator) monitorCancellation(ctx context.Context, cancel context.CancelFunc, scope generation.Scope, id string, done chan<- struct{}) {
	defer close(done)
	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			readCtx, readCancel := context.WithTimeout(ctx, 5*time.Second)
			run, err := coordinator.repository.Get(readCtx, scope, id)
			readCancel()
			if errors.Is(err, generation.ErrNotFound) || (err == nil && (run.Status == "cancel_requested" || run.Terminal())) {
				cancel()
				return
			}
		}
	}
}
