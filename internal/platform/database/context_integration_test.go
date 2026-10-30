package database

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/Bobby-P-dev/go-diagram.git/internal/integration/contextgateway"
	"github.com/Bobby-P-dev/go-diagram.git/internal/modules/generation"
	generationservice "github.com/Bobby-P-dev/go-diagram.git/internal/modules/generation/service"
)

func TestContextResolverLiveIntegration(test *testing.T) {
	newFixture := workspaceTestCluster(test)
	ctx := context.Background()

	db := newFixture()
	if err := RunMigrations(ctx, db); err != nil {
		test.Fatal(err)
	}

	userAlice := "11111111-1111-4111-8111-111111111111"
	userBob := "22222222-2222-4222-8222-222222222222"
	executeFixture(test, db, `INSERT INTO access_credentials(id, name, credential_key) VALUES
		($1, 'Alice', 'hashA'), ($2, 'Bob', 'hashB')`, userAlice, userBob)

	projAlice := "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"
	projBob := "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb"
	executeFixture(test, db, `INSERT INTO projects(id, user_id, title, description, project_mode) VALUES
		($1, $2, 'Alice Project', 'System architecture and diagrams', 'connected'),
		($3, $4, 'Bob Project', 'Confidential financial designs', 'connected')`, projAlice, userAlice, projBob, userBob)

	threadAlice := "33333333-3333-4333-8333-333333333333"
	threadAlice2 := "44444444-4444-4444-8444-444444444444"
	threadBob := "55555555-5555-4555-8555-555555555555"
	executeFixture(test, db, `INSERT INTO chat_threads(id, project_id, title, stage_hint) VALUES
		($1, $2, 'Brainstorming Thread', 'brainstorm'),
		($3, $2, 'PRD Thread', 'prd'),
		($4, $5, 'Bob Private Thread', 'brainstorm')`, threadAlice, projAlice, threadAlice2, threadBob, projBob)

	executeFixture(test, db, `INSERT INTO chat_messages_v2(id, project_id, thread_id, role, content) VALUES
		('60000000-0000-4000-8000-000000000001', $1, $2, 'user', 'Need an order payment workflow'),
		('60000000-0000-4000-8000-000000000002', $1, $2, 'assistant', 'Understood, let us plan the order schema and state machine'),
		('60000000-0000-4000-8000-000000000003', $1, $3, 'user', 'PRD Section 1: Non-functional constraints')`,
		projAlice, threadAlice, threadAlice2)

	artAlice1 := "70000000-0000-4000-8000-000000000001"
	revAlice1v1 := "80000000-0000-4000-8000-000000000001"
	revAlice1v2 := "80000000-0000-4000-8000-000000000002"
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		test.Fatal(err)
	}
	defer tx.Rollback()
	executeArtifact := func(query string, arguments ...any) {
		test.Helper()
		if _, err := tx.ExecContext(ctx, query, arguments...); err != nil {
			test.Fatal(err)
		}
	}
	executeArtifact(`INSERT INTO artifacts(id, project_id, type, title, slug, format, current_version_number, current_revision_id)
		VALUES ($1, $2, 'prd', 'Order Service PRD', 'order-prd', 'markdown', 2, $3)`, artAlice1, projAlice, revAlice1v2)
	executeArtifact(`INSERT INTO artifact_revisions(id, project_id, artifact_id, version_number, change_summary, content_format, document_content, author_id, author_type)
		VALUES ($1, $2, $3, 1, 'Initial PRD', 'markdown', '# PRD v1 draft', $4, 'manual'),
		       ($5, $2, $3, 2, 'Added SLA rules', 'markdown', '# PRD v2 with 99.99% availability rules', $4, 'manual')`,
		revAlice1v1, projAlice, artAlice1, userAlice, revAlice1v2)

	artAlice2 := "70000000-0000-4000-8000-000000000002"
	revAlice2v1 := "80000000-0000-4000-8000-000000000003"
	executeArtifact(`INSERT INTO artifacts(id, project_id, type, title, slug, format, current_version_number, current_revision_id)
		VALUES ($1, $2, 'erd', 'Order ERD', 'order-erd', 'graph', 1, $3)`, artAlice2, projAlice, revAlice2v1)
	executeArtifact(`INSERT INTO artifact_revisions(id, project_id, artifact_id, version_number, change_summary, content_format, graph_nodes, graph_edges, author_id, author_type)
		VALUES ($1, $2, $3, 1, 'Initial ERD', 'graph', '[{"id":"orders","label":"orders_tbl"}]'::jsonb, '[]'::jsonb, $4, 'manual')`,
		revAlice2v1, projAlice, artAlice2, userAlice)
	if err := tx.Commit(); err != nil {
		test.Fatal(err)
	}
	projAliceOther := "cccccccc-cccc-4ccc-8ccc-cccccccccccc"
	threadAliceOther := "dddddddd-dddd-4ddd-8ddd-dddddddddddd"
	executeFixture(test, db, `INSERT INTO projects(id, user_id, title, project_mode) VALUES ($1,$2,'Unrelated project','connected')`, projAliceOther, userAlice)
	executeFixture(test, db, `INSERT INTO chat_threads(id, project_id, title) VALUES ($1,$2,'Unrelated thread')`, threadAliceOther, projAliceOther)
	otherArtifact, otherRevision := createFixtureArtifact(test, db, projAliceOther, "unrelated-graph")

	resolver := generationservice.NewContextResolver(contextgateway.NewPostgresReader(db))
	test.Run("same_owner_does_not_grant_cross_project_context", func(test *testing.T) {
		for _, input := range []generation.ResolveInput{
			{ThreadIDs: []string{threadAliceOther}},
			{Sources: []generation.RevisionRef{{ArtifactID: otherArtifact, RevisionID: otherRevision}}},
			{Sources: []generation.RevisionRef{{ArtifactID: artAlice1, RevisionID: revAlice2v1}}},
		} {
			_, err := resolver.Resolve(ctx, generation.Scope{UserID: userAlice, ProjectID: projAlice, ThreadID: threadAlice}, input)
			if !errors.Is(err, generation.ErrNotFound) {
				test.Fatalf("cross-scope reference accepted: %v", err)
			}
		}
	})
	test.Run("automatic_context_without_exclusions_and_latest_selection", func(test *testing.T) {
		manifest, err := resolver.Resolve(ctx, generation.Scope{UserID: userAlice, ProjectID: projAlice, ThreadID: threadAlice}, generation.ResolveInput{Mode: "project_relevant"})
		if err != nil {
			test.Fatal(err)
		}
		if len(manifest.Sources) != 2 {
			test.Fatalf("expected both artifacts, got %+v", manifest.Sources)
		}
		manifest, err = resolver.Resolve(ctx, generation.Scope{UserID: userAlice, ProjectID: projAlice, ThreadID: threadAlice}, generation.ResolveInput{Sources: []generation.RevisionRef{{ArtifactID: artAlice1}}})
		if err != nil || manifest.Sources[0].RevisionID != revAlice1v2 {
			test.Fatalf("latest head not pinned: %+v %v", manifest, err)
		}
	})

	test.Run("user_isolation_blocks_unauthorized_project", func(test *testing.T) {
		_, err := resolver.Resolve(ctx, generation.Scope{UserID: userBob, ProjectID: projAlice, ThreadID: threadAlice}, generation.ResolveInput{})
		if !errors.Is(err, generation.ErrNotFound) {
			test.Fatalf("expected ErrNotFound, got %v", err)
		}
	})

	test.Run("cross_project_thread_rejected", func(test *testing.T) {
		_, err := resolver.Resolve(ctx, generation.Scope{UserID: userAlice, ProjectID: projAlice, ThreadID: threadAlice}, generation.ResolveInput{
			ThreadIDs: []string{threadBob},
		})
		if !errors.Is(err, generation.ErrNotFound) {
			test.Fatalf("expected ErrNotFound for Bob's thread, got %v", err)
		}
	})

	test.Run("explicit_mode_resolves_and_pins_requested_revision", func(test *testing.T) {
		manifest, err := resolver.Resolve(ctx, generation.Scope{UserID: userAlice, ProjectID: projAlice, ThreadID: threadAlice}, generation.ResolveInput{
			Mode: "explicit",
			Sources: []generation.RevisionRef{
				{ArtifactID: artAlice1, RevisionID: revAlice1v1},
			},
			ThreadIDs: []string{threadAlice2},
		})
		if err != nil {
			test.Fatal(err)
		}
		if len(manifest.Sources) != 1 {
			test.Fatalf("expected 1 source, got %d", len(manifest.Sources))
		}
		if manifest.Sources[0].RevisionID != revAlice1v1 || manifest.Sources[0].VersionNumber != 1 {
			test.Fatalf("pinned v1 mismatch: got %+v", manifest.Sources[0])
		}
		if len(manifest.Messages) != 3 {
			test.Fatalf("expected 3 messages across 2 threads, got %d", len(manifest.Messages))
		}
		for i := 1; i < len(manifest.Messages); i++ {
			if manifest.Messages[i].SequenceNumber < manifest.Messages[i-1].SequenceNumber {
				test.Fatalf("messages not sorted chronologically: %+v", manifest.Messages)
			}
		}
	})

	test.Run("project_relevant_mode_selects_latest_and_respects_exclusions", func(test *testing.T) {
		manifest, err := resolver.Resolve(ctx, generation.Scope{UserID: userAlice, ProjectID: projAlice, ThreadID: threadAlice}, generation.ResolveInput{
			Mode:                "project_relevant",
			ExcludedArtifactIDs: []string{artAlice2},
		})
		if err != nil {
			test.Fatal(err)
		}
		if len(manifest.Sources) != 1 {
			test.Fatalf("expected 1 source (artAlice2 excluded), got %d", len(manifest.Sources))
		}
		if manifest.Sources[0].ArtifactID != artAlice1 || manifest.Sources[0].RevisionID != revAlice1v2 {
			test.Fatalf("expected head of artAlice1 (v2), got %+v", manifest.Sources[0])
		}
	})

	test.Run("budget_omissions_recorded_cleanly", func(test *testing.T) {
		executeFixture(test, db, `INSERT INTO chat_messages_v2(project_id, thread_id, role, content) VALUES ($1,$2,'user',$3)`, projAlice, threadAlice, strings.Repeat("x", 2048))
		manifest, err := resolver.Resolve(ctx, generation.Scope{UserID: userAlice, ProjectID: projAlice, ThreadID: threadAlice}, generation.ResolveInput{
			Mode: "explicit",
			Sources: []generation.RevisionRef{
				{ArtifactID: artAlice1, RevisionID: revAlice1v2},
			},
			MaxContentBytes: 1024,
		})
		if err != nil {
			test.Fatal(err)
		}
		if manifest.Budget.MaxContentBytes != 1024 {
			test.Fatalf("expected budget 1024, got %d", manifest.Budget.MaxContentBytes)
		}
		if manifest.Budget.UsedContentBytes > 1024 {
			test.Fatalf("budget exceeded: %d > 1024", manifest.Budget.UsedContentBytes)
		}
		if len(manifest.Omitted) != 1 || manifest.Omitted[0].MessageID == "" || manifest.Omitted[0].Reason != "content_budget" {
			test.Fatalf("expected oversized message omission: %+v", manifest.Omitted)
		}
	})
}
