package database

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Bobby-P-dev/go-diagram.git/internal/integration/contextgateway"
	"github.com/Bobby-P-dev/go-diagram.git/internal/modules/generation"
	runcontroller "github.com/Bobby-P-dev/go-diagram.git/internal/modules/generation/controller"
	runrepository "github.com/Bobby-P-dev/go-diagram.git/internal/modules/generation/repository"
	runservice "github.com/Bobby-P-dev/go-diagram.git/internal/modules/generation/service"
	"github.com/Bobby-P-dev/go-diagram.git/internal/modules/identity/authcontext"
	identityservice "github.com/Bobby-P-dev/go-diagram.git/internal/modules/identity/service"
)

type diagramFixtureClient struct{ inputs chan generation.DiagramInput }

func integrationGraph(kind string) *generation.DiagramGraph {
	nodeType := "process"
	if kind == "erd" {
		nodeType = "entity"
	}
	if kind == "sequence" {
		nodeType = "participant"
	}
	graph := &generation.DiagramGraph{
		Nodes:    []generation.DiagramNode{{ID: "reader", Type: nodeType, Data: generation.DiagramNodeData{Label: "Reader"}}, {ID: "loan", Type: nodeType, Data: generation.DiagramNodeData{Label: "Loan"}}},
		Edges:    []generation.DiagramEdge{{ID: "lending", Source: "reader", Target: "loan", Label: "Borrow"}},
		Metadata: generation.DiagramMetadata{DiagramType: kind},
	}
	if kind == "erd" {
		for index := range graph.Nodes {
			graph.Nodes[index].Data.Fields = []generation.DiagramField{{Name: "id", DataType: "uuid", PrimaryKey: true}}
		}
		graph.Edges[0].Cardinality = "1:N"
	}
	if kind == "sequence" {
		graph.Edges[0].SequenceNumber = 1
		graph.Edges[0].MessageType = "call"
	}
	return graph
}

func (client *diagramFixtureClient) CallLLMStreamContext(ctx context.Context, _ string, _ []runservice.LLMMessage, prompt string, _ generation.ProgressSink) (string, error) {
	var input generation.DiagramInput
	if err := json.Unmarshal([]byte(prompt), &input); err != nil {
		return "", err
	}
	select {
	case client.inputs <- input:
	case <-ctx.Done():
		return "", ctx.Err()
	}
	graph := integrationGraph(input.Type)
	if input.BaseGraph != nil {
		graph = input.BaseGraph
	}
	payload := map[string]any{"title": input.Title, "graph_nodes": graph.Nodes, "graph_edges": graph.Edges, "metadata": graph.Metadata, "change_summary": "Drafted diagram", "assistant_message": "Diagram ready for review."}
	encoded, err := json.Marshal(payload)
	return string(encoded), err
}

func TestDiagramGenerationIntegration(test *testing.T) {
	newFixture := workspaceTestCluster(test)
	db := newFixture()
	ctx := context.Background()
	if err := RunMigrations(ctx, db); err != nil {
		test.Fatal(err)
	}
	scope := generation.Scope{UserID: "10000000-0000-4000-8000-000000000001", ProjectID: "10000000-0000-4000-8000-000000000002", ThreadID: "10000000-0000-4000-8000-000000000003"}
	trigger := "10000000-0000-4000-8000-000000000004"
	executeFixture(test, db, `INSERT INTO access_credentials(id,credential_key,name,role,is_active) VALUES($1,'diagram-test','Test','user',true)`, scope.UserID)
	executeFixture(test, db, `INSERT INTO projects(id,user_id,title) VALUES($1,$2,'Library')`, scope.ProjectID, scope.UserID)
	executeFixture(test, db, `INSERT INTO chat_threads(id,project_id,title) VALUES($1,$2,'Planning')`, scope.ThreadID, scope.ProjectID)
	executeFixture(test, db, `INSERT INTO chat_messages_v2(id,project_id,thread_id,role,content) VALUES($1,$2,$3,'user','Plan lending from the PRD')`, trigger, scope.ProjectID, scope.ThreadID)
	repository := runrepository.NewPostgresRunRepository(db)
	client := &diagramFixtureClient{inputs: make(chan generation.DiagramInput, 10)}
	coordinator := runservice.NewRunCoordinator(repository, runservice.NewContextResolver(contextgateway.NewPostgresReader(db)), nil, runservice.NewDiagramEngine(client))
	controller := runcontroller.NewRunController(coordinator)
	start := func(key string, input generation.StartInput) *generation.Run {
		test.Helper()
		payload, err := json.Marshal(input)
		if err != nil {
			test.Fatal(err)
		}
		request := httptest.NewRequest(http.MethodPost, "/generations", strings.NewReader(string(payload)))
		request.Header.Set("Idempotency-Key", key)
		request.SetPathValue("projectId", scope.ProjectID)
		request.SetPathValue("threadId", scope.ThreadID)
		request = request.WithContext(authcontext.WithUser(request.Context(), &identityservice.CredentialRecord{ID: scope.UserID, Role: "user"}))
		response := httptest.NewRecorder()
		controller.Start(response, request)
		if response.Code != http.StatusAccepted {
			test.Fatalf("accept: %d %s", response.Code, response.Body.String())
		}
		var envelope struct {
			Data generation.Run `json:"data"`
		}
		if err := json.Unmarshal(response.Body.Bytes(), &envelope); err != nil {
			test.Fatal(err)
		}
		run, err := repository.Get(ctx, scope, envelope.Data.ID)
		if err != nil {
			test.Fatal(err)
		}
		return run
	}
	claim := func(run *generation.Run) {
		test.Helper()
		if _, ok, err := repository.Claim(ctx, scope, run.ID); err != nil || !ok {
			test.Fatalf("claim: %v %v", ok, err)
		}
	}
	input := generation.StartInput{TriggerMessageID: trigger, OutputType: "prd", OutputTitle: "Library PRD"}
	source := start("source", input)
	claim(source)
	source, err := repository.Commit(ctx, scope, source.ID, &generation.ArtifactCandidate{Type: "prd", Title: "Library PRD", DocumentContent: "# Requirements\nReaders borrow books.", ChangeSummary: "Drafted PRD", AssistantMessage: "PRD ready.", Format: "markdown", SchemaVersion: 1})
	if err != nil {
		test.Fatal(err)
	}
	input.Context = generation.ResolveInput{Mode: "explicit", Sources: []generation.RevisionRef{{ArtifactID: source.CommittedArtifactID, RevisionID: source.CommittedRevisionID}}}
	drive := func(id string) *generation.Run {
		test.Helper()
		workerCtx, cancel := context.WithCancel(ctx)
		done := make(chan struct{})
		go func() { defer close(done); coordinator.Run(workerCtx) }()
		defer func() { cancel(); <-done }()
		deadline := time.Now().Add(5 * time.Second)
		for time.Now().Before(deadline) {
			run, err := repository.Get(ctx, scope, id)
			if err != nil {
				test.Fatal(err)
			}
			if run.Terminal() {
				if run.Status != "succeeded" {
					test.Fatalf("worker: %+v", run)
				}
				return run
			}
			time.Sleep(20 * time.Millisecond)
		}
		test.Fatal("worker timeout")
		return nil
	}
	var first *generation.Run
	for _, kind := range []string{"erd", "flowchart", "sequence"} {
		test.Run("http_worker_commit_"+kind, func(test *testing.T) {
			input.OutputType, input.OutputTitle = kind, "Library "+kind
			run := start(kind, input)
			if replay := start(kind, input); replay.ID != run.ID {
				test.Fatal("idempotency duplicated run")
			}
			result := drive(run.ID)
			received := <-client.inputs
			if received.Type != kind || len(received.Manifest.Sources) != 1 || received.Manifest.Sources[0].RevisionID != source.CommittedRevisionID || received.Manifest.PreviewOnly {
				test.Fatalf("lost context: %+v", received)
			}
			assertCount(test, db, 1, `SELECT count(*) FROM artifact_revisions WHERE id=$1 AND content_format='graph' AND document_content IS NULL AND jsonb_array_length(graph_nodes)=2 AND jsonb_array_length(graph_edges)=1 AND metadata->>'diagram_type'=$2`, result.CommittedRevisionID, kind)
			assertCount(test, db, 1, `SELECT count(*) FROM artifact_context_links WHERE downstream_artifact_id=$1 AND upstream_revision_id=$2`, result.CommittedArtifactID, source.CommittedRevisionID)
			assertCount(test, db, 1, `SELECT count(*) FROM chat_messages_v2 WHERE id=$1 AND metadata->>'revision_id'=$2`, result.CommittedMessageID, result.CommittedRevisionID)
			if first == nil {
				first = result
			}
		})
	}
	input.OutputType, input.OutputTitle = "erd", "Library erd"
	input.TargetArtifactID, input.TargetExpectedRevisionID = first.CommittedArtifactID, first.CommittedRevisionID
	candidate := &generation.ArtifactCandidate{Type: "erd", Title: input.OutputTitle, Graph: integrationGraph("erd"), ChangeSummary: "Revised", AssistantMessage: "Updated diagram.", Format: "graph", SchemaVersion: 1}
	test.Run("frozen_base_revision_and_conflict", func(test *testing.T) {
		left, right := start("left", input), start("right", input)
		if left.BaseDocument != nil || left.BaseGraph == nil || left.BaseGraph.Nodes[0].Data.Label != "Reader" {
			test.Fatal("base graph not frozen")
		}
		claim(left)
		claim(right)
		candidate.Graph.Nodes[0].Data.Label = "Member"
		updated, err := repository.Commit(ctx, scope, left.ID, candidate)
		if err != nil {
			test.Fatal(err)
		}
		if _, err := repository.Commit(ctx, scope, right.ID, candidate); !errors.Is(err, generation.ErrRevisionConflict) {
			test.Fatalf("conflict: %v", err)
		}
		frozen, err := repository.Get(ctx, scope, right.ID)
		if err != nil || frozen.BaseGraph.Nodes[0].Data.Label != "Reader" {
			test.Fatal("frozen graph changed")
		}
		assertCount(test, db, 1, `SELECT count(*) FROM artifacts WHERE id=$1 AND current_revision_id=$2 AND current_version_number=2`, first.CommittedArtifactID, updated.CommittedRevisionID)
		if err := repository.Finish(ctx, scope, right.ID, "failed", "REVISION_CONFLICT", "Conflict"); err != nil {
			test.Fatal(err)
		}
	})
	test.Run("cancel_graph_before_commit", func(test *testing.T) {
		request := input
		request.TargetArtifactID = ""
		request.TargetExpectedRevisionID = ""
		run := start("cancel", request)
		claim(run)
		if _, err := repository.Cancel(ctx, scope, run.ID); err != nil {
			test.Fatal(err)
		}
		result, err := repository.Commit(ctx, scope, run.ID, candidate)
		if err != nil || result.Status != "canceled" {
			test.Fatalf("cancel: %+v %v", result, err)
		}
		assertCount(test, db, 0, `SELECT count(*) FROM chat_messages_v2 WHERE metadata->>'generation_run_id'=$1`, run.ID)
	})
	test.Run("revision_worker_receives_frozen_graph", func(test *testing.T) {
		request := input
		if err := db.QueryRow(`SELECT current_revision_id FROM artifacts WHERE id=$1`, first.CommittedArtifactID).Scan(&request.TargetExpectedRevisionID); err != nil {
			test.Fatal(err)
		}
		run := start("worker-revision", request)
		result := drive(run.ID)
		received := <-client.inputs
		if received.BaseGraph == nil || received.BaseGraph.Nodes[0].Data.Label != "Member" || received.BaseGraph.Metadata.DiagramType != "erd" {
			test.Fatal("worker lost revision snapshot")
		}
		assertCount(test, db, 1, `SELECT count(*) FROM artifact_revisions WHERE id=$1 AND version_number=3 AND graph_nodes->0->'data'->>'label'='Member'`, result.CommittedRevisionID)
	})
	test.Run("graph_lineage_error_rolls_back_everything", func(test *testing.T) {
		request := input
		request.TargetArtifactID = ""
		request.TargetExpectedRevisionID = ""
		run := start("rollback", request)
		claim(run)
		var artifacts, revisions, messages, links int
		for query, target := range map[string]*int{"SELECT count(*) FROM artifacts": &artifacts, "SELECT count(*) FROM artifact_revisions": &revisions, "SELECT count(*) FROM chat_messages_v2": &messages, "SELECT count(*) FROM artifact_context_links": &links} {
			if err := db.QueryRow(query).Scan(target); err != nil {
				test.Fatal(err)
			}
		}
		executeFixture(test, db, `CREATE FUNCTION fail_diagram_lineage() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'fixture failure'; END $$; CREATE TRIGGER fail_diagram_lineage BEFORE INSERT ON artifact_context_links FOR EACH ROW EXECUTE FUNCTION fail_diagram_lineage()`)
		if _, err := repository.Commit(ctx, scope, run.ID, candidate); err == nil {
			test.Fatal("expected rollback")
		}
		executeFixture(test, db, `DROP TRIGGER fail_diagram_lineage ON artifact_context_links; DROP FUNCTION fail_diagram_lineage()`)
		assertCount(test, db, artifacts, `SELECT count(*) FROM artifacts`)
		assertCount(test, db, revisions, `SELECT count(*) FROM artifact_revisions`)
		assertCount(test, db, messages, `SELECT count(*) FROM chat_messages_v2`)
		assertCount(test, db, links, `SELECT count(*) FROM artifact_context_links`)
		assertCount(test, db, 1, `SELECT count(*) FROM generation_runs WHERE id=$1 AND status='running' AND committed_revision_id IS NULL`, run.ID)
	})
	test.Run("revision_lineage_failure_preserves_head", func(test *testing.T) {
		request := input
		if err := db.QueryRow(`SELECT current_revision_id FROM artifacts WHERE id=$1`, first.CommittedArtifactID).Scan(&request.TargetExpectedRevisionID); err != nil {
			test.Fatal(err)
		}
		run := start("rollback-revision", request)
		claim(run)
		executeFixture(test, db, `CREATE FUNCTION fail_revision_lineage() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'fixture failure'; END $$; CREATE TRIGGER fail_revision_lineage BEFORE INSERT ON artifact_context_links FOR EACH ROW EXECUTE FUNCTION fail_revision_lineage()`)
		if _, err := repository.Commit(ctx, scope, run.ID, candidate); err == nil {
			test.Fatal("expected rollback")
		}
		executeFixture(test, db, `DROP TRIGGER fail_revision_lineage ON artifact_context_links; DROP FUNCTION fail_revision_lineage()`)
		assertCount(test, db, 1, `SELECT count(*) FROM artifacts WHERE id=$1 AND current_revision_id=$2 AND current_version_number=3`, first.CommittedArtifactID, request.TargetExpectedRevisionID)
		assertCount(test, db, 3, `SELECT count(*) FROM artifact_revisions WHERE artifact_id=$1`, first.CommittedArtifactID)
		assertCount(test, db, 0, `SELECT count(*) FROM chat_messages_v2 WHERE metadata->>'generation_run_id'=$1`, run.ID)
	})
}

func TestDiagramMigrationUpgradeIntegration(test *testing.T) {
	newFixture := workspaceTestCluster(test)
	db := newFixture()
	ctx := context.Background()
	executeFixture(test, db, initSQL)
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		test.Fatal(err)
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `CREATE TABLE schema_migrations(version TEXT PRIMARY KEY, checksum TEXT NOT NULL, applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW())`); err != nil {
		test.Fatal(err)
	}
	for _, version := range []string{"04_connected_workspace", "05_generation_runs"} {
		if err := applyWorkspaceMigration(ctx, tx, version); err != nil {
			test.Fatal(err)
		}
	}
	if err := tx.Commit(); err != nil {
		test.Fatal(err)
	}
	executeFixture(test, db, `INSERT INTO access_credentials(id,credential_key,name,role,is_active) VALUES('10000000-0000-4000-8000-000000000001','upgrade-test','Test','user',true);
INSERT INTO projects(id,user_id,title) VALUES('10000000-0000-4000-8000-000000000002','10000000-0000-4000-8000-000000000001','Upgrade');
INSERT INTO chat_threads(id,project_id,title) VALUES('10000000-0000-4000-8000-000000000003','10000000-0000-4000-8000-000000000002','Planning');
INSERT INTO chat_messages_v2(id,project_id,thread_id,role,content) VALUES('10000000-0000-4000-8000-000000000004','10000000-0000-4000-8000-000000000002','10000000-0000-4000-8000-000000000003','user','Plan');
INSERT INTO generation_runs(project_id,thread_id,user_id,trigger_message_id,output_type,frozen_manifest,idempotency_key,request_hash) VALUES('10000000-0000-4000-8000-000000000002','10000000-0000-4000-8000-000000000003','10000000-0000-4000-8000-000000000001','10000000-0000-4000-8000-000000000004','prd','{}','existing',repeat('a',64))`)
	if err := RunMigrations(ctx, db); err != nil {
		test.Fatal(err)
	}
	if err := RunMigrations(ctx, db); err != nil {
		test.Fatal(err)
	}
	assertCount(test, db, 5, `SELECT count(*) FROM schema_migrations`)
	assertCount(test, db, 1, `SELECT count(*) FROM generation_runs WHERE output_type='prd' AND status='queued' AND base_graph IS NULL`)
	executeFixture(test, db, `UPDATE generation_runs SET output_type='erd' WHERE idempotency_key='existing'`)
	if _, err := db.Exec(`UPDATE generation_runs SET output_type='sequence_diagram'`); err == nil {
		test.Fatal("database accepted unsupported output type")
	}
	if _, err := db.Exec(`UPDATE generation_runs SET base_document='wrong format'`); err == nil {
		test.Fatal("graph run accepted document snapshot")
	}
	if _, err := db.Exec(`UPDATE generation_runs SET base_graph='[]'::jsonb`); err == nil {
		test.Fatal("graph run accepted non-object snapshot")
	}
}
