package database

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"testing"

	"github.com/Bobby-P-dev/go-diagram.git/internal/modules/artifact"
	artifactrepo "github.com/Bobby-P-dev/go-diagram.git/internal/modules/artifact/repository"
)

func TestArtifactRepositoryLiveIntegration(test *testing.T) {
	newFixture := workspaceTestCluster(test)
	ctx := context.Background()

	test.Run("artifact_revisions_restores_and_lineage_lifecycle", func(test *testing.T) {
		db := newFixture()
		if err := RunMigrations(ctx, db); err != nil {
			test.Fatal(err)
		}

		userA := "11111111-1111-1111-1111-111111111111"
		userB := "22222222-2222-2222-2222-222222222222"
		executeFixture(test, db, `INSERT INTO access_credentials(id, name, credential_key) VALUES
			($1, 'Alice', 'hashA'), ($2, 'Bob', 'hashB')`, userA, userB)

		projAlice := "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
		projBob := "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"
		executeFixture(test, db, `INSERT INTO projects(id, user_id, title, project_mode) VALUES
			($1, $2, 'Alice Project', 'connected'),
			($3, $4, 'Bob Project', 'connected')`, projAlice, userA, projBob, userB)

		repo := artifactrepo.NewPostgresRepository(db)

		doc1 := "# System Architecture Brainstorm\nCore invariants defined."
		createInput := artifact.CreateArtifactInput{
			Type:            "brainstorm",
			Title:           "Architecture Brainstorm",
			Slug:            "arch-brainstorm",
			DocumentContent: &doc1,
			ChangeSummary:   "Initial brain dump",
		}
		writeReq := artifact.WriteRequest{
			Key:         "key-art-1",
			RequestHash: hashPayload(createInput),
			Resource:    fmt.Sprintf("/api/v2/projects/%s/artifacts", projAlice),
		}

		artA, err := repo.CreateArtifact(ctx, userA, projAlice, createInput, writeReq)
		if err != nil {
			test.Fatalf("create artifact failed: %v", err)
		}
		if artA.Title != createInput.Title || artA.Format != "markdown" || artA.CurrentVersionNumber != 1 {
			test.Fatalf("unexpected created artifact: %+v", artA)
		}
		if artA.CurrentRevision == nil || artA.CurrentRevision.DocumentContent == nil || *artA.CurrentRevision.DocumentContent != doc1 {
			test.Fatalf("unexpected initial revision: %+v", artA.CurrentRevision)
		}

		replayArt, err := repo.CreateArtifact(ctx, userA, projAlice, createInput, writeReq)
		if err != nil {
			test.Fatalf("replay failed: %v", err)
		}
		if replayArt.ID != artA.ID || replayArt.CurrentRevisionID != artA.CurrentRevisionID {
			test.Fatalf("replay returned mismatched artifact: %+v vs %+v", replayArt, artA)
		}

		conflictInput := createInput
		conflictInput.Title = "Different Title"
		conflictReq := artifact.WriteRequest{
			Key:         "key-art-1",
			RequestHash: hashPayload(conflictInput),
			Resource:    fmt.Sprintf("/api/v2/projects/%s/artifacts", projAlice),
		}
		_, err = repo.CreateArtifact(ctx, userA, projAlice, conflictInput, conflictReq)
		if !errors.Is(err, artifact.ErrIdempotencyConflict) {
			test.Fatalf("expected ErrIdempotencyConflict, got %v", err)
		}

		dupSlugInput := artifact.CreateArtifactInput{
			Type:            "prd",
			Title:           "Another PRD",
			Slug:            "arch-brainstorm",
			DocumentContent: &doc1,
		}
		_, err = repo.CreateArtifact(ctx, userA, projAlice, dupSlugInput, artifact.WriteRequest{
			Key:         "key-dup-slug",
			RequestHash: hashPayload(dupSlugInput),
			Resource:    fmt.Sprintf("/api/v2/projects/%s/artifacts", projAlice),
		})
		if !errors.Is(err, artifact.ErrSlugConflict) {
			test.Fatalf("expected ErrSlugConflict, got %v", err)
		}

		_, err = repo.GetArtifact(ctx, userB, projAlice, artA.ID)
		if !errors.Is(err, artifact.ErrNotFound) {
			test.Fatalf("expected ErrNotFound for Bob getting Alice's artifact, got %v", err)
		}

		bobList, err := repo.ListArtifacts(ctx, userB, projAlice, artifact.PageQuery{Limit: 10})
		if !errors.Is(err, artifact.ErrNotFound) {
			test.Fatalf("expected ErrNotFound for Bob listing Alice's project artifacts, got %v", err)
		}
		if bobList != nil {
			test.Fatalf("expected nil list for unauthorized user")
		}

		nodesJSON := json.RawMessage(`[{"id":"user_table","label":"users"}]`)
		edgesJSON := json.RawMessage(`[]`)
		erdInput := artifact.CreateArtifactInput{
			Type:          "erd",
			Title:         "Database Schema",
			Slug:          "erd-model",
			GraphNodes:    nodesJSON,
			GraphEdges:    edgesJSON,
			ChangeSummary: "Initial ERD nodes",
		}
		erdArt, err := repo.CreateArtifact(ctx, userA, projAlice, erdInput, artifact.WriteRequest{
			Key:         "key-erd-1",
			RequestHash: hashPayload(erdInput),
			Resource:    fmt.Sprintf("/api/v2/projects/%s/artifacts", projAlice),
		})
		if err != nil {
			test.Fatalf("create ERD failed: %v", err)
		}
		if erdArt.Format != "graph" || erdArt.CurrentRevision == nil || len(erdArt.CurrentRevision.GraphNodes) == 0 {
			test.Fatalf("unexpected ERD artifact: %+v", erdArt)
		}

		frameJSON := json.RawMessage(`{"nodes":[{"id":"screen-1","title":"Dashboard"}],"edges":[]}`)
		uiInput := artifact.CreateArtifactInput{
			Type:          "ui_design",
			Title:         "App Dashboard",
			Slug:          "app-dashboard",
			FrameData:     frameJSON,
			ChangeSummary: "Initial UI frame",
			Sources: []artifact.RevisionRef{
				{ArtifactID: artA.ID, RevisionID: artA.CurrentRevisionID, LinkType: "context_source"},
				{ArtifactID: erdArt.ID, RevisionID: erdArt.CurrentRevisionID, LinkType: "derived_from"},
			},
		}
		uiArt, err := repo.CreateArtifact(ctx, userA, projAlice, uiInput, artifact.WriteRequest{
			Key:         "key-ui-1",
			RequestHash: hashPayload(uiInput),
			Resource:    fmt.Sprintf("/api/v2/projects/%s/artifacts", projAlice),
		})
		if err != nil {
			test.Fatalf("create UI Design artifact failed: %v", err)
		}
		if len(uiArt.CurrentRevision.Sources) != 2 {
			test.Fatalf("expected 2 context sources, got %d", len(uiArt.CurrentRevision.Sources))
		}

		bobDoc := "Bob artifact"
		bobArt, err := repo.CreateArtifact(ctx, userB, projBob, artifact.CreateArtifactInput{
			Type:            "brainstorm",
			Title:           "Bob Idea",
			Slug:            "bob-idea",
			DocumentContent: &bobDoc,
		}, artifact.WriteRequest{
			Key:         "key-bob-art",
			RequestHash: hashPayload("bob-artifact"),
			Resource:    fmt.Sprintf("/api/v2/projects/%s/artifacts", projBob),
		})
		if err != nil {
			test.Fatal(err)
		}

		crossInput := artifact.CreateArtifactInput{
			Type:            "prd",
			Title:           "Sneaky PRD",
			Slug:            "sneaky-prd",
			DocumentContent: &doc1,
			Sources: []artifact.RevisionRef{
				{ArtifactID: bobArt.ID, RevisionID: bobArt.CurrentRevisionID, LinkType: "context_source"},
			},
		}
		_, err = repo.CreateArtifact(ctx, userA, projAlice, crossInput, artifact.WriteRequest{
			Key:         "key-cross-src",
			RequestHash: hashPayload(crossInput),
			Resource:    fmt.Sprintf("/api/v2/projects/%s/artifacts", projAlice),
		})
		if !errors.Is(err, artifact.ErrNotFound) {
			test.Fatalf("expected ErrNotFound for cross-project source link, got %v", err)
		}
		assertCount(test, db, 0, "SELECT count(*) FROM artifacts WHERE slug = 'sneaky-prd'")

		doc2 := "# System Architecture Brainstorm\nExpanded with caching and message bus."
		appendInput := artifact.AppendRevisionInput{
			ExpectedRevisionID: artA.CurrentRevisionID,
			ChangeSummary:      "Added caching details",
			DocumentContent:    &doc2,
		}
		appWrite := artifact.WriteRequest{
			Key:         "key-append-1",
			RequestHash: hashPayload(appendInput),
			Resource:    fmt.Sprintf("/api/v2/projects/%s/artifacts/%s/revisions", projAlice, artA.ID),
		}

		rev2, err := repo.AppendRevision(ctx, userA, projAlice, artA.ID, appendInput, appWrite)
		if err != nil {
			test.Fatalf("append revision failed: %v", err)
		}
		if rev2.VersionNumber != 2 || rev2.DocumentContent == nil || *rev2.DocumentContent != doc2 {
			test.Fatalf("unexpected revision 2: %+v", rev2)
		}

		replayRev2, err := repo.AppendRevision(ctx, userA, projAlice, artA.ID, appendInput, appWrite)
		if err != nil {
			test.Fatalf("replay append failed: %v", err)
		}
		if replayRev2.ID != rev2.ID {
			test.Fatalf("replay append ID mismatch: %s vs %s", replayRev2.ID, rev2.ID)
		}

		artUpdated, err := repo.GetArtifact(ctx, userA, projAlice, artA.ID)
		if err != nil {
			test.Fatal(err)
		}
		if artUpdated.CurrentVersionNumber != 2 || artUpdated.CurrentRevisionID != rev2.ID {
			test.Fatalf("artifact head not updated: %+v", artUpdated)
		}

		staleAppend := artifact.AppendRevisionInput{
			ExpectedRevisionID: artA.CurrentRevisionID,
			ChangeSummary:      "Stale modification",
			DocumentContent:    &doc1,
		}
		_, err = repo.AppendRevision(ctx, userA, projAlice, artA.ID, staleAppend, artifact.WriteRequest{
			Key:         "key-stale-app",
			RequestHash: hashPayload(staleAppend),
			Resource:    fmt.Sprintf("/api/v2/projects/%s/artifacts/%s/revisions", projAlice, artA.ID),
		})
		if !errors.Is(err, artifact.ErrRevisionConflict) {
			test.Fatalf("expected ErrRevisionConflict, got %v", err)
		}

		_, err = db.Exec("UPDATE artifact_revisions SET change_summary = 'Tampered' WHERE id = $1", rev2.ID)
		assertSQLState(test, err, "23514")

		_, err = db.Exec("DELETE FROM artifact_revisions WHERE id = $1", rev2.ID)
		assertSQLState(test, err, "23514")

		restoreInput := artifact.RestoreRevisionInput{
			TargetRevisionID:   artA.CurrentRevisionID,
			ExpectedRevisionID: rev2.ID,
			ChangeSummary:      "Rolled back to initial vision",
		}
		restWrite := artifact.WriteRequest{
			Key:         "key-restore-1",
			RequestHash: hashPayload(restoreInput),
			Resource:    fmt.Sprintf("/api/v2/projects/%s/artifacts/%s/restores", projAlice, artA.ID),
		}

		rev3, err := repo.RestoreRevision(ctx, userA, projAlice, artA.ID, restoreInput, restWrite)
		if err != nil {
			test.Fatalf("restore revision failed: %v", err)
		}
		if rev3.VersionNumber != 3 || rev3.AuthorType != "restore" {
			test.Fatalf("unexpected restored revision: %+v", rev3)
		}
		if *rev3.DocumentContent != doc1 {
			test.Fatalf("restored content does not match target: %s vs %s", *rev3.DocumentContent, doc1)
		}

		var meta struct {
			RestoredFromRevisionID    string `json:"restored_from_revision_id"`
			RestoredFromVersionNumber int    `json:"restored_from_version_number"`
		}
		if err := json.Unmarshal(rev3.Metadata, &meta); err != nil {
			test.Fatalf("failed to unmarshal metadata: %v", err)
		}
		if meta.RestoredFromRevisionID != artA.CurrentRevisionID || meta.RestoredFromVersionNumber != 1 {
			test.Fatalf("incorrect provenance metadata: %+v", meta)
		}

		artAfterRestore, err := repo.GetArtifact(ctx, userA, projAlice, artA.ID)
		if err != nil {
			test.Fatal(err)
		}
		if artAfterRestore.CurrentVersionNumber != 3 || artAfterRestore.CurrentRevisionID != rev3.ID {
			test.Fatalf("artifact head not pointing to restored revision: %+v", artAfterRestore)
		}

		artList, err := repo.ListArtifacts(ctx, userA, projAlice, artifact.PageQuery{
			Limit:      1,
			TypeFilter: "brainstorm",
		})
		if err != nil {
			test.Fatal(err)
		}
		if len(artList.Items) != 1 || artList.Items[0].Format != "markdown" {
			test.Fatalf("unexpected list artifacts response: %+v", artList)
		}

		revList, err := repo.ListRevisions(ctx, userA, projAlice, artA.ID, artifact.PageQuery{Limit: 2})
		if err != nil {
			test.Fatal(err)
		}
		if len(revList.Items) != 2 || revList.Items[0].VersionNumber != 3 || revList.Items[1].VersionNumber != 2 || revList.NextCursor == nil {
			test.Fatalf("unexpected revisions page 1: %+v", revList)
		}

		revList2, err := repo.ListRevisions(ctx, userA, projAlice, artA.ID, artifact.PageQuery{
			Limit:  2,
			Cursor: *revList.NextCursor,
		})
		if err != nil {
			test.Fatal(err)
		}
		if len(revList2.Items) != 1 || revList2.Items[0].VersionNumber != 1 {
			test.Fatalf("unexpected revisions page 2: %+v", revList2)
		}
	})

	test.Run("concurrent_appends_optimistic_conflict", func(test *testing.T) {
		db := newFixture()
		if err := RunMigrations(ctx, db); err != nil {
			test.Fatal(err)
		}
		user := "44444444-4444-4444-4444-444444444444"
		executeFixture(test, db, `INSERT INTO access_credentials(id, name, credential_key) VALUES ($1, 'Dave', 'hashD')`, user)
		projID := "dddddddd-dddd-dddd-dddd-dddddddddddd"
		executeFixture(test, db, `INSERT INTO projects(id, user_id, title, project_mode) VALUES ($1, $2, 'Dave Project', 'connected')`, projID, user)

		repo := artifactrepo.NewPostgresRepository(db)
		initialDoc := "Version 1"
		art, err := repo.CreateArtifact(ctx, user, projID, artifact.CreateArtifactInput{
			Type:            "brainstorm",
			Title:           "Race Test",
			Slug:            "race-test",
			DocumentContent: &initialDoc,
		}, artifact.WriteRequest{
			Key:         "race-init",
			RequestHash: hashPayload("initial-artifact"),
			Resource:    fmt.Sprintf("/api/v2/projects/%s/artifacts", projID),
		})
		if err != nil {
			test.Fatal(err)
		}

		const workers = 4
		var wg sync.WaitGroup
		wg.Add(workers)

		results := make([]*artifact.ArtifactRevision, workers)
		errs := make([]error, workers)

		for i := 0; i < workers; i++ {
			workerIdx := i
			go func() {
				defer wg.Done()
				doc := fmt.Sprintf("Revision 2 from worker %d", workerIdx)
				input := artifact.AppendRevisionInput{
					ExpectedRevisionID: art.CurrentRevisionID,
					ChangeSummary:      fmt.Sprintf("Worker %d", workerIdx),
					DocumentContent:    &doc,
				}
				results[workerIdx], errs[workerIdx] = repo.AppendRevision(ctx, user, projID, art.ID, input, artifact.WriteRequest{
					Key:         fmt.Sprintf("worker-key-%d", workerIdx),
					RequestHash: hashPayload(input),
					Resource:    fmt.Sprintf("/api/v2/projects/%s/artifacts/%s/revisions", projID, art.ID),
				})
			}()
		}
		wg.Wait()

		successCount := 0
		conflictCount := 0
		for i := 0; i < workers; i++ {
			if errs[i] == nil {
				successCount++
			} else if errors.Is(errs[i], artifact.ErrRevisionConflict) {
				conflictCount++
			} else {
				test.Fatalf("unexpected error from worker %d: %v", i, errs[i])
			}
		}

		if successCount != 1 || conflictCount != workers-1 {
			test.Fatalf("expected 1 success and %d conflicts, got %d successes and %d conflicts", workers-1, successCount, conflictCount)
		}

		assertCount(test, db, 2, "SELECT count(*) FROM artifact_revisions WHERE artifact_id = $1", art.ID)
	})
}

func TestArtifactTriggerMessageIsolation(test *testing.T) {
	newFixture := workspaceTestCluster(test)
	ctx := context.Background()
	test.Run("trigger_message_cross_project_isolation", func(test *testing.T) {
		db := newFixture()
		if err := RunMigrations(ctx, db); err != nil {
			test.Fatal(err)
		}
		userA := "11111111-1111-1111-1111-111111111111"
		userB := "22222222-2222-2222-2222-222222222222"
		executeFixture(test, db, `INSERT INTO access_credentials(id, name, credential_key) VALUES ($1, 'Alice', 'hashA'), ($2, 'Bob', 'hashB')`, userA, userB)
		projA := "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
		projB := "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"
		executeFixture(test, db, `INSERT INTO projects(id, user_id, title) VALUES ($1, $2, 'Proj A'), ($3, $4, 'Proj B')`, projA, userA, projB, userB)

		threadA := "10101010-1010-1010-1010-101010101010"
		threadB := "20202020-2020-2020-2020-202020202020"
		executeFixture(test, db, `INSERT INTO chat_threads(id, project_id, title) VALUES ($1, $2, 'Thread A'), ($3, $4, 'Thread B')`, threadA, projA, threadB, projB)

		msgA := "30303030-3030-3030-3030-303030303030"
		msgB := "40404040-4040-4040-4040-404040404040"
		executeFixture(test, db, `INSERT INTO chat_messages_v2(id, project_id, thread_id, role, content) VALUES
			($1, $2, $3, 'user', 'Prompt in Alice project'),
			($4, $5, $6, 'user', 'Prompt in Bob project')`, msgA, projA, threadA, msgB, projB, threadB)

		repo := artifactrepo.NewPostgresRepository(db)
		doc := "Content"

		_, err := repo.CreateArtifact(ctx, userA, projA, artifact.CreateArtifactInput{
			Type:             "brainstorm",
			Title:            "Sneaky Trigger",
			Slug:             "sneaky-trigger",
			DocumentContent:  &doc,
			TriggerMessageID: &msgB,
		}, artifact.WriteRequest{
			Key:         "key-trig-cross",
			RequestHash: hashPayload("cross-trig"),
			Resource:    fmt.Sprintf("/api/v2/projects/%s/artifacts", projA),
		})
		if !errors.Is(err, artifact.ErrNotFound) {
			test.Fatalf("expected ErrNotFound for cross-project trigger message, got %v", err)
		}

		art, err := repo.CreateArtifact(ctx, userA, projA, artifact.CreateArtifactInput{
			Type:             "brainstorm",
			Title:            "Legit Trigger",
			Slug:             "legit-trigger",
			DocumentContent:  &doc,
			TriggerMessageID: &msgA,
		}, artifact.WriteRequest{
			Key:         "key-trig-legit",
			RequestHash: hashPayload("legit-trig"),
			Resource:    fmt.Sprintf("/api/v2/projects/%s/artifacts", projA),
		})
		if err != nil {
			test.Fatalf("create artifact with trigger message failed: %v", err)
		}
		if art.CurrentRevision == nil || art.CurrentRevision.TriggerMessageID == nil || *art.CurrentRevision.TriggerMessageID != msgA {
			test.Fatalf("expected trigger message ID %s, got %+v", msgA, art.CurrentRevision)
		}
	})
}
