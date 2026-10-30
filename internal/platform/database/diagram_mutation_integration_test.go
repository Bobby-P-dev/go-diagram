package database

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
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

type mutationFixtureClient struct{ inputs chan generation.DiagramInput }

func (client *mutationFixtureClient) CallLLMStreamContext(ctx context.Context, _ string, _ []runservice.LLMMessage, prompt string, _ generation.ProgressSink) (string, error) {
	var input generation.DiagramInput
	if err := json.Unmarshal([]byte(prompt), &input); err != nil {
		return "", err
	}
	select {
	case client.inputs <- input:
	case <-ctx.Done():
		return "", ctx.Err()
	}
	nodes, edges := []generation.DiagramNode{}, []generation.DiagramEdge{}
	for _, id := range input.Mutation.NodeIDs {
		for _, node := range input.BaseGraph.Nodes {
			if node.ID == id {
				node.Data.Label += " revised"
				nodes = append(nodes, node)
			}
		}
	}
	for _, id := range input.Mutation.EdgeIDs {
		for _, edge := range input.BaseGraph.Edges {
			if edge.ID == id {
				edge.Label += " revised"
				edges = append(edges, edge)
			}
		}
	}
	encoded, err := json.Marshal(map[string]any{"title": input.Title, "graph_nodes": nodes, "graph_edges": edges, "metadata": input.BaseGraph.Metadata, "change_summary": "Updated selected elements", "assistant_message": "Selected elements revised."})
	return string(encoded), err
}

func TestDiagramMutationIntegration(test *testing.T) {
	newFixture := workspaceTestCluster(test)
	db := newFixture()
	ctx := context.Background()
	if err := RunMigrations(ctx, db); err != nil {
		test.Fatal(err)
	}
	scope := generation.Scope{UserID: "10000000-0000-4000-8000-000000000001", ProjectID: "10000000-0000-4000-8000-000000000002", ThreadID: "10000000-0000-4000-8000-000000000003"}
	trigger := "10000000-0000-4000-8000-000000000004"
	executeFixture(test, db, `INSERT INTO access_credentials(id,credential_key,name) VALUES($1,'mutation-test','Test')`, scope.UserID)
	executeFixture(test, db, `INSERT INTO projects(id,user_id,title) VALUES($1,$2,'Mutation project')`, scope.ProjectID, scope.UserID)
	executeFixture(test, db, `INSERT INTO chat_threads(id,project_id,title) VALUES($1,$2,'Revisions')`, scope.ThreadID, scope.ProjectID)
	executeFixture(test, db, `INSERT INTO chat_messages_v2(id,project_id,thread_id,role,content) VALUES($1,$2,$3,'user','Revise only the selected elements')`, trigger, scope.ProjectID, scope.ThreadID)
	repository := runrepository.NewPostgresRunRepository(db)
	client := &mutationFixtureClient{inputs: make(chan generation.DiagramInput, 20)}
	coordinator := runservice.NewRunCoordinator(repository, runservice.NewContextResolver(contextgateway.NewPostgresReader(db)), nil, runservice.NewDiagramEngine(client))
	controller := runcontroller.NewRunController(coordinator)
	start := func(key string, input generation.StartInput) *generation.Run {
		test.Helper()
		encoded, _ := json.Marshal(input)
		request := httptest.NewRequest(http.MethodPost, "/generations", strings.NewReader(string(encoded)))
		request.Header.Set("Idempotency-Key", key)
		request.SetPathValue("projectId", scope.ProjectID)
		request.SetPathValue("threadId", scope.ThreadID)
		request = request.WithContext(authcontext.WithUser(ctx, &identityservice.CredentialRecord{ID: scope.UserID, Role: "user"}))
		response := httptest.NewRecorder()
		controller.Start(response, request)
		if response.Code != http.StatusAccepted {
			test.Fatalf("HTTP %d: %s", response.Code, response.Body.String())
		}
		if strings.Contains(response.Body.String(), `"base_graph"`) || strings.Contains(response.Body.String(), `"base_graph_raw"`) {
			test.Fatal("private raw snapshot leaked")
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
			test.Fatalf("claim %v %v", ok, err)
		}
	}
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
					test.Fatalf("worker failed %+v", run)
				}
				return run
			}
			time.Sleep(20 * time.Millisecond)
		}
		test.Fatal("worker timeout")
		return nil
	}
	for _, kind := range []string{"erd", "flowchart", "sequence"} {
		test.Run(kind, func(test *testing.T) {
			graph := integrationGraph(kind)
			nodeJSON, _ := json.Marshal(graph.Nodes)
			edgeJSON, _ := json.Marshal(graph.Edges)
			var rawNodes, rawEdges []map[string]json.RawMessage
			_ = json.Unmarshal(nodeJSON, &rawNodes)
			_ = json.Unmarshal(edgeJSON, &rawEdges)
			rawNodes[0]["position"] = json.RawMessage(`{"x":12.75,"y":45}`)
			rawNodes[0]["extension"] = json.RawMessage(`900719925474099312345`)
			rawNodes[1]["position"] = json.RawMessage(`{"x":400,"y":20}`)
			rawEdges[0]["extension"] = json.RawMessage(`{"keep":true}`)
			nodeJSON, _ = json.Marshal(rawNodes)
			edgeJSON, _ = json.Marshal(rawEdges)
			tx, err := db.BeginTx(ctx, nil)
			if err != nil {
				test.Fatal(err)
			}
			defer tx.Rollback()
			var artifactID, revisionID string
			if err := tx.QueryRow(`INSERT INTO artifacts(project_id,type,title,slug,format) VALUES($1,$2,$3,$2,'graph') RETURNING id,current_revision_id`, scope.ProjectID, kind, "Diagram "+kind).Scan(&artifactID, &revisionID); err != nil {
				test.Fatal(err)
			}
			if _, err := tx.Exec(`INSERT INTO artifact_revisions(id,project_id,artifact_id,version_number,content_format,graph_nodes,graph_edges,metadata,author_type) VALUES($1,$2,$3,1,'graph',$4,$5,$6,'manual')`, revisionID, scope.ProjectID, artifactID, string(nodeJSON), string(edgeJSON), `{"diagram_type":"`+kind+`","extension":"keep metadata"}`); err != nil {
				test.Fatal(err)
			}
			if err := tx.Commit(); err != nil {
				test.Fatal(err)
			}
			input := generation.StartInput{TriggerMessageID: trigger, OutputType: kind, OutputTitle: "Diagram " + kind, TargetArtifactID: artifactID, TargetExpectedRevisionID: revisionID, Mutation: &generation.DiagramMutation{NodeIDs: []string{"reader"}, EdgeIDs: []string{"lending"}}, Context: generation.ResolveInput{Mode: "explicit"}}
			input.Context.Sources = []generation.RevisionRef{{ArtifactID: artifactID, RevisionID: revisionID}}
			invalid := input
			invalid.Mutation = &generation.DiagramMutation{NodeIDs: []string{"missing"}, EdgeIDs: []string{}}
			if _, err := coordinator.Start(ctx, scope, invalid, kind+"-invalid"); !errors.Is(err, generation.ErrInvalidInput) {
				test.Fatalf("unknown target accepted: %v", err)
			}
			invalid = input
			invalid.OutputTitle = "Rename outside scope"
			if _, err := coordinator.Start(ctx, scope, invalid, kind+"-rename"); !errors.Is(err, generation.ErrInvalidInput) {
				test.Fatal("renamed scoped artifact")
			}
			run := start(kind, input)
			if replay := start(kind, input); replay.ID != run.ID {
				test.Fatal("replay changed run")
			}
			invalid = input
			invalid.Mutation = &generation.DiagramMutation{NodeIDs: []string{"loan"}, EdgeIDs: []string{}}
			if _, err := coordinator.Start(ctx, scope, invalid, kind); !errors.Is(err, generation.ErrIdempotencyConflict) {
				test.Fatal("scope missing from idempotency hash")
			}
			if !strings.Contains(string(run.BaseGraphRaw), "900719925474099312345") {
				test.Fatal("raw snapshot lost extension")
			}
			otherScope := scope
			otherScope.UserID = "90000000-0000-4000-8000-000000000001"
			if _, err := repository.Get(ctx, otherScope, run.ID); !errors.Is(err, generation.ErrNotFound) {
				test.Fatal("other owner accessed run")
			}
			result := drive(run.ID)
			received := <-client.inputs
			if !reflect.DeepEqual(received.Mutation, input.Mutation) || received.BaseGraph.Nodes[0].Data.Label != "Reader" {
				test.Fatal("worker scope/snapshot mismatch")
			}
			assertCount(test, db, 1, `SELECT count(*) FROM artifact_revisions WHERE id=$1 AND version_number=2 AND graph_nodes->0->'data'->>'label'='Reader revised' AND graph_nodes->1->'data'->>'label'='Loan' AND graph_nodes->0->>'extension'='900719925474099312345' AND graph_nodes->0->'position'->>'x'='12.75' AND graph_edges->0->>'label'='Borrow revised' AND graph_edges->0->'extension'->>'keep'='true' AND metadata->>'extension'='keep metadata' AND document_content IS NULL`, result.CommittedRevisionID)
			assertCount(test, db, 1, `SELECT count(*) FROM chat_messages_v2 WHERE id=$1 AND metadata->>'generation_run_id'=$2`, result.CommittedMessageID, run.ID)
			assertCount(test, db, 1, `SELECT count(*) FROM artifact_context_links WHERE downstream_artifact_id=$1 AND downstream_revision_number=2 AND upstream_artifact_id=$1 AND upstream_revision_id=$2 AND link_type='context_source'`, artifactID, revisionID)
			assertCount(test, db, 1, `SELECT count(*) FROM artifacts WHERE id=$1 AND current_revision_id=$2 AND current_version_number=2 AND title=$3`, artifactID, result.CommittedRevisionID, input.OutputTitle)
			input.TargetExpectedRevisionID = result.CommittedRevisionID
			makeCandidate := func(run *generation.Run) *generation.ArtifactCandidate {
				copyGraph := *run.BaseGraph
				copyGraph.Nodes = append([]generation.DiagramNode{}, run.BaseGraph.Nodes...)
				copyGraph.Edges = append([]generation.DiagramEdge{}, run.BaseGraph.Edges...)
				copyGraph.Nodes[0].Data.Label += " again"
				return &generation.ArtifactCandidate{Type: kind, Title: input.OutputTitle, Graph: &copyGraph, Format: "graph", SchemaVersion: 1, ChangeSummary: "Targeted update", AssistantMessage: "Updated selected node."}
			}
			unsafe := start(kind+"-unsafe", input)
			claim(unsafe)
			candidate := makeCandidate(unsafe)
			candidate.Graph.Nodes[1].Data.Label = "Unauthorized"
			if _, err := repository.Commit(ctx, scope, unsafe.ID, candidate); !errors.Is(err, generation.ErrInvalidDiagram) {
				test.Fatalf("commit bypassed scope: %v", err)
			}
			if err := repository.Finish(ctx, scope, unsafe.ID, "failed", "SCOPE_INVALID", "Rejected"); err != nil {
				test.Fatal(err)
			}
			left, right := start(kind+"-left", input), start(kind+"-right", input)
			claim(left)
			claim(right)
			updated, err := repository.Commit(ctx, scope, left.ID, makeCandidate(left))
			if err != nil {
				test.Fatal(err)
			}
			if _, err := repository.Commit(ctx, scope, right.ID, makeCandidate(right)); !errors.Is(err, generation.ErrRevisionConflict) {
				test.Fatalf("expected conflict: %v", err)
			}
			if err := repository.Finish(ctx, scope, right.ID, "failed", "REVISION_CONFLICT", "Conflict"); err != nil {
				test.Fatal(err)
			}
			input.TargetExpectedRevisionID = updated.CommittedRevisionID
			canceled := start(kind+"-cancel", input)
			claim(canceled)
			if _, err := repository.Cancel(ctx, scope, canceled.ID); err != nil {
				test.Fatal(err)
			}
			if current, err := repository.Commit(ctx, scope, canceled.ID, makeCandidate(canceled)); err != nil || current.Status != "canceled" {
				test.Fatalf("cancel failed: %+v %v", current, err)
			}
			rollback := start(kind+"-rollback", input)
			claim(rollback)
			executeFixture(test, db, `CREATE FUNCTION fail_mutation_message() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'fixture failure'; END $$; CREATE TRIGGER fail_mutation_message BEFORE INSERT ON chat_messages_v2 FOR EACH ROW WHEN (NEW.role='assistant') EXECUTE FUNCTION fail_mutation_message()`)
			if _, err := repository.Commit(ctx, scope, rollback.ID, makeCandidate(rollback)); err == nil {
				test.Fatal("expected rollback")
			}
			executeFixture(test, db, `DROP TRIGGER fail_mutation_message ON chat_messages_v2; DROP FUNCTION fail_mutation_message()`)
			assertCount(test, db, 1, `SELECT count(*) FROM artifacts WHERE id=$1 AND current_revision_id=$2 AND current_version_number=3`, artifactID, updated.CommittedRevisionID)
			assertCount(test, db, 3, `SELECT count(*) FROM artifact_revisions WHERE artifact_id=$1`, artifactID)
			assertCount(test, db, 2, `SELECT count(*) FROM artifact_context_links WHERE downstream_artifact_id=$1`, artifactID)
			assertCount(test, db, 0, `SELECT count(*) FROM chat_messages_v2 WHERE metadata->>'generation_run_id' IN ($1,$2)`, canceled.ID, rollback.ID)
			if err := repository.Finish(ctx, scope, rollback.ID, "failed", "COMMIT_FAILED", "Rolled back"); err != nil {
				test.Fatal(err)
			}
			for _, invalidMutation := range []string{`{}`, `{"node_ids":[],"edge_ids":[]}`, `{"node_ids":null,"edge_ids":[]}`, `{"node_ids":["reader"]}`} {
				if _, err := db.Exec(`UPDATE generation_runs SET mutation=$2 WHERE id=$1`, run.ID, invalidMutation); err == nil {
					test.Fatalf("constraint accepted %s", invalidMutation)
				}
			}
		})
	}
}
