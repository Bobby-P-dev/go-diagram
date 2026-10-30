package database

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Bobby-P-dev/go-diagram.git/internal/integration/contextgateway"
	"github.com/Bobby-P-dev/go-diagram.git/internal/modules/generation"
	runrepository "github.com/Bobby-P-dev/go-diagram.git/internal/modules/generation/repository"
	runservice "github.com/Bobby-P-dev/go-diagram.git/internal/modules/generation/service"
)

type blockingDocumentEngine struct {
	started chan struct{}
	once    sync.Once
}

func (engine *blockingDocumentEngine) GenerateDocument(ctx context.Context, input generation.DocumentInput, emit generation.ProgressSink) (*generation.DocumentCandidate, error) {
	engine.once.Do(func() { close(engine.started) })
	<-ctx.Done()
	return nil, ctx.Err()
}

func TestGenerationRunsIntegration(test *testing.T) {
	newFixture := workspaceTestCluster(test)
	db := newFixture()
	ctx := context.Background()
	if err := RunMigrations(ctx, db); err != nil {
		test.Fatal(err)
	}
	scope := generation.Scope{
		UserID:    "10000000-0000-4000-8000-000000000001",
		ProjectID: "20000000-0000-4000-8000-000000000001",
		ThreadID:  "30000000-0000-4000-8000-000000000001",
	}
	const trigger = "40000000-0000-4000-8000-000000000001"
	const otherProject = "20000000-0000-4000-8000-000000000002"
	const otherThread = "30000000-0000-4000-8000-000000000002"
	executeFixture(test, db, `INSERT INTO access_credentials(id,credential_key,name) VALUES ($1,'generation-fixture','Test')`, scope.UserID)
	executeFixture(test, db, `INSERT INTO projects(id,title,user_id,project_mode) VALUES ($1,'First',$3,'workspace'),($2,'Second',$3,'workspace')`, scope.ProjectID, otherProject, scope.UserID)
	executeFixture(test, db, `INSERT INTO chat_threads(id,project_id,title) VALUES ($1,$2,'First'),($3,$4,'Second')`, scope.ThreadID, scope.ProjectID, otherThread, otherProject)
	executeFixture(test, db, `INSERT INTO chat_messages_v2(id,project_id,thread_id,role,content) VALUES ($1,$2,$3,'user','Create a PRD for a library lending app')`, trigger, scope.ProjectID, scope.ThreadID)
	repository := runrepository.NewPostgresRunRepository(db)
	resolver := runservice.NewContextResolver(contextgateway.NewPostgresReader(db))
	coordinator := runservice.NewRunCoordinator(repository, resolver, nil, nil)
	input := generation.StartInput{TriggerMessageID: trigger, OutputType: "prd", OutputTitle: "Library PRD"}
	candidate := &generation.ArtifactCandidate{Type: "prd", Title: "Library PRD", DocumentContent: "# Library\nLending and returns.", ChangeSummary: "First draft", AssistantMessage: "Prepared the PRD.", Format: "markdown", SchemaVersion: 1}
	var first *generation.Run
	start := func(key string, request generation.StartInput) *generation.Run {
		test.Helper()
		run, err := coordinator.Start(ctx, scope, request, key)
		if err != nil {
			test.Fatal(err)
		}
		return run
	}
	claim := func(run *generation.Run) *generation.Run {
		test.Helper()
		claimed, ok, err := repository.Claim(ctx, scope, run.ID)
		if err != nil || !ok {
			test.Fatalf("claim: %v %v", ok, err)
		}
		return claimed
	}
	test.Run("atomic_creation_and_idempotency", func(test *testing.T) {
		run := start("create", input)
		if run.Status != "queued" || run.Manifest.PreviewOnly || run.Prompt == "" {
			test.Fatalf("bad acceptance: %+v", run)
		}
		replay := start("create", input)
		if replay.ID != run.ID {
			test.Fatal("replay created another run")
		}
		changed := input
		changed.OutputTitle = "Different"
		if _, err := coordinator.Start(ctx, scope, changed, "create"); !errors.Is(err, generation.ErrIdempotencyConflict) {
			test.Fatalf("hash conflict: %v", err)
		}
		claim(run)
		if _, ok, err := repository.Claim(ctx, scope, run.ID); err != nil || ok {
			test.Fatalf("duplicate claim: %v %v", ok, err)
		}
		var err error
		first, err = repository.Commit(ctx, scope, run.ID, candidate)
		if err != nil {
			test.Fatal(err)
		}
		if first.Status != "succeeded" || first.CommittedArtifactID == "" || first.CommittedRevisionID == "" || first.CommittedMessageID == "" {
			test.Fatalf("bad commit: %+v", first)
		}
		assertCount(test, db, 1, `SELECT count(*) FROM artifacts WHERE id=$1 AND current_revision_id=$2 AND current_version_number=1`, first.CommittedArtifactID, first.CommittedRevisionID)
		assertCount(test, db, 1, `SELECT count(*) FROM chat_messages_v2 WHERE id=$1 AND role='assistant' AND metadata->>'trigger_message_id'=$2`, first.CommittedMessageID, trigger)
		if _, err := repository.Commit(ctx, scope, run.ID, candidate); !errors.Is(err, generation.ErrRunTerminal) {
			test.Fatalf("double commit: %v", err)
		}
		if _, err := repository.Cancel(ctx, scope, run.ID); !errors.Is(err, generation.ErrRunTerminal) {
			test.Fatalf("cancel succeeded: %v", err)
		}
	})
	test.Run("authorization_and_trigger_scope", func(test *testing.T) {
		foreign := scope
		foreign.ProjectID = otherProject
		if _, err := repository.Get(ctx, foreign, first.ID); !errors.Is(err, generation.ErrNotFound) {
			test.Fatalf("project leak: %v", err)
		}
		if _, err := repository.Cancel(ctx, foreign, first.ID); !errors.Is(err, generation.ErrNotFound) {
			test.Fatalf("foreign cancel: %v", err)
		}
		foreign.UserID = "10000000-0000-4000-8000-000000000099"
		if _, err := repository.Get(ctx, foreign, first.ID); !errors.Is(err, generation.ErrNotFound) {
			test.Fatalf("owner leak: %v", err)
		}
		foreign = scope
		foreign.ProjectID = otherProject
		foreign.ThreadID = otherThread
		if _, err := coordinator.Start(ctx, foreign, input, "foreign-trigger"); !errors.Is(err, generation.ErrNotFound) {
			test.Fatalf("foreign trigger: %v", err)
		}
		var assistant string
		if err := db.QueryRow(`INSERT INTO chat_messages_v2(project_id,thread_id,role,content) VALUES ($1,$2,'assistant','Not a trigger') RETURNING id`, scope.ProjectID, scope.ThreadID).Scan(&assistant); err != nil {
			test.Fatal(err)
		}
		invalid := input
		invalid.TriggerMessageID = assistant
		if _, err := coordinator.Start(ctx, scope, invalid, "assistant-trigger"); !errors.Is(err, generation.ErrNotFound) {
			test.Fatalf("non-user trigger: %v", err)
		}
	})
	test.Run("revision_conflict_and_pinned_lineage", func(test *testing.T) {
		request := input
		request.TargetArtifactID = first.CommittedArtifactID
		request.TargetExpectedRevisionID = first.CommittedRevisionID
		request.Context = generation.ResolveInput{Sources: []generation.RevisionRef{{ArtifactID: first.CommittedArtifactID}}}
		left, right := start("revision-left", request), start("revision-right", request)
		if left.BaseDocument == nil || !strings.Contains(*left.BaseDocument, "Lending") {
			test.Fatal("base not frozen")
		}
		claim(left)
		claim(right)
		updated, err := repository.Commit(ctx, scope, left.ID, candidate)
		if err != nil {
			test.Fatal(err)
		}
		assertCount(test, db, 1, `SELECT count(*) FROM artifact_context_links WHERE downstream_artifact_id=$1 AND downstream_revision_number=2 AND upstream_revision_id=$2`, first.CommittedArtifactID, first.CommittedRevisionID)
		if _, err := repository.Commit(ctx, scope, right.ID, candidate); !errors.Is(err, generation.ErrRevisionConflict) {
			test.Fatalf("stale revision: %v", err)
		}
		if err := repository.Finish(ctx, scope, right.ID, "failed", "REVISION_CONFLICT", "Artifact changed"); err != nil {
			test.Fatal(err)
		}
		assertCount(test, db, 2, `SELECT count(*) FROM artifact_revisions WHERE artifact_id=$1`, first.CommittedArtifactID)
		assertCount(test, db, 0, `SELECT count(*) FROM chat_messages_v2 WHERE metadata->>'generation_run_id'=$1`, right.ID)
		assertCount(test, db, 1, `SELECT count(*) FROM artifacts WHERE id=$1 AND current_revision_id=$2`, updated.CommittedArtifactID, updated.CommittedRevisionID)
	})
	test.Run("cancel_queued_running_and_commit_race", func(test *testing.T) {
		queued := start("cancel-queued", input)
		canceled, err := repository.Cancel(ctx, scope, queued.ID)
		if err != nil || canceled.Status != "canceled" {
			test.Fatalf("cancel queued: %v %v", canceled, err)
		}
		if _, err := repository.Cancel(ctx, scope, queued.ID); err != nil {
			test.Fatal(err)
		}
		running := claim(start("cancel-running", input))
		requested, err := repository.Cancel(ctx, scope, running.ID)
		if err != nil || requested.Status != "cancel_requested" {
			test.Fatalf("cancel running: %v %v", requested, err)
		}
		committed, err := repository.Commit(ctx, scope, running.ID, candidate)
		if err != nil || committed.Status != "canceled" {
			test.Fatalf("commit after cancel: %v %v", committed, err)
		}
		assertCount(test, db, 0, `SELECT count(*) FROM chat_messages_v2 WHERE metadata->>'generation_run_id'=$1`, running.ID)
	})
	test.Run("rollback_all_writes_on_commit_error", func(test *testing.T) {
		run := claim(start("rollback", input))
		executeFixture(test, db, `CREATE FUNCTION fail_generated_assistant() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN IF NEW.role='assistant' THEN RAISE EXCEPTION 'fixture failure'; END IF; RETURN NEW; END $$;
 CREATE TRIGGER fail_generated_assistant BEFORE INSERT ON chat_messages_v2 FOR EACH ROW EXECUTE FUNCTION fail_generated_assistant()`)
		if _, err := repository.Commit(ctx, scope, run.ID, candidate); err == nil {
			test.Fatal("expected failure")
		}
		executeFixture(test, db, `DROP TRIGGER fail_generated_assistant ON chat_messages_v2; DROP FUNCTION fail_generated_assistant()`)
		assertCount(test, db, 1, `SELECT count(*) FROM artifacts WHERE project_id=$1`, scope.ProjectID)
		if err := repository.Finish(ctx, scope, run.ID, "failed", "COMMIT_FAILED", "Commit failed"); err != nil {
			test.Fatal(err)
		}
	})
	test.Run("expired_worker_lease_recovers_without_duplicate_execution", func(test *testing.T) {
		run := claim(start("interrupted", input))
		executeFixture(test, db, `UPDATE generation_runs SET lease_expires_at=NOW()-INTERVAL '1 second' WHERE id=$1`, run.ID)
		if err := repository.RecoverInterrupted(ctx); err != nil {
			test.Fatal(err)
		}
		failed, err := repository.Get(ctx, scope, run.ID)
		if err != nil || failed.Status != "failed" || failed.ErrorCode != "WORKER_INTERRUPTED" {
			test.Fatalf("recovery: %v %v", failed, err)
		}
		if _, err := repository.Commit(ctx, scope, run.ID, candidate); !errors.Is(err, generation.ErrRunTerminal) {
			test.Fatalf("late worker commit: %v", err)
		}
	})
	test.Run("real_worker_cooperative_cancel_without_open_llm_transaction", func(test *testing.T) {
		engine := &blockingDocumentEngine{started: make(chan struct{})}
		worker := runservice.NewRunCoordinator(repository, resolver, engine, nil)
		run := start("worker-cancel", input)
		workerCtx, stop := context.WithCancel(ctx)
		done := make(chan struct{})
		go func() { defer close(done); worker.Run(workerCtx) }()
		defer func() { stop(); <-done }()
		select {
		case <-engine.started:
		case <-time.After(5 * time.Second):
			test.Fatal("worker did not start")
		}
		assertCount(test, db, 0, `SELECT count(*) FROM pg_stat_activity WHERE datname=current_database() AND state='idle in transaction'`)
		if _, err := repository.Cancel(ctx, scope, run.ID); err != nil {
			test.Fatal(err)
		}
		deadline := time.Now().Add(5 * time.Second)
		for time.Now().Before(deadline) {
			result, err := repository.Get(ctx, scope, run.ID)
			if err != nil {
				test.Fatal(err)
			}
			if result.Status == "canceled" {
				return
			}
			time.Sleep(25 * time.Millisecond)
		}
		test.Fatal("worker did not honor cancellation")
	})
}
