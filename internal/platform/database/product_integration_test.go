package database

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"testing"

	"github.com/Bobby-P-dev/go-diagram.git/internal/modules/product"
	productrepo "github.com/Bobby-P-dev/go-diagram.git/internal/modules/product/repository"
)

func hashPayload(v interface{}) string {
	b, _ := json.Marshal(v)
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}

func TestProductRepositoryLiveIntegration(test *testing.T) {
	newFixture := workspaceTestCluster(test)
	ctx := context.Background()

	test.Run("projects_threads_messages_lifecycle_and_isolation", func(test *testing.T) {
		db := newFixture()
		if err := RunMigrations(ctx, db); err != nil {
			test.Fatal(err)
		}

		userA := "11111111-1111-1111-1111-111111111111"
		userB := "22222222-2222-2222-2222-222222222222"
		executeFixture(test, db, `INSERT INTO access_credentials(id, name, credential_key) VALUES
			($1, 'Alice', 'hashA'), ($2, 'Bob', 'hashB')`, userA, userB)

		repo := productrepo.NewPostgresRepository(db)

		// 1. Create Project with Idempotency
		projInput := product.ProjectInput{Title: "Alice System Architecture", Description: "End-to-End Design"}
		writeReq := product.WriteRequest{
			Key:         "key-proj-1",
			RequestHash: hashPayload(projInput),
			Resource:    "/api/v2/projects",
		}

		projA, err := repo.CreateProject(ctx, userA, projInput, writeReq)
		if err != nil {
			test.Fatalf("create project failed: %v", err)
		}
		if projA.Title != projInput.Title || projA.UserID != userA {
			test.Fatalf("unexpected project: %+v", projA)
		}

		// Replay same idempotency key and hash -> must return cached project
		replayProj, err := repo.CreateProject(ctx, userA, projInput, writeReq)
		if err != nil {
			test.Fatalf("replay failed: %v", err)
		}
		if replayProj.ID != projA.ID {
			test.Fatalf("replay returned different ID: %s vs %s", replayProj.ID, projA.ID)
		}

		// Replay same key with different hash -> ErrIdempotencyConflict
		conflictInput := product.ProjectInput{Title: "Different Title"}
		conflictReq := product.WriteRequest{
			Key:         "key-proj-1",
			RequestHash: hashPayload(conflictInput),
			Resource:    "/api/v2/projects",
		}
		_, err = repo.CreateProject(ctx, userA, conflictInput, conflictReq)
		if !errors.Is(err, product.ErrIdempotencyConflict) {
			test.Fatalf("expected ErrIdempotencyConflict, got %v", err)
		}

		// 2. Owner Isolation on ListProjects
		pageA, err := repo.ListProjects(ctx, userA, product.PageQuery{Limit: 10})
		if err != nil {
			test.Fatal(err)
		}
		if len(pageA.Items) != 1 || pageA.Items[0].ID != projA.ID {
			test.Fatalf("expected 1 project for Alice, got %d", len(pageA.Items))
		}

		pageB, err := repo.ListProjects(ctx, userB, product.PageQuery{Limit: 10})
		if err != nil {
			test.Fatal(err)
		}
		if len(pageB.Items) != 0 {
			test.Fatalf("expected 0 projects for Bob, got %d", len(pageB.Items))
		}

		// 3. Create Thread within Project
		threadInput := product.ThreadInput{
			Title:    "PRD Discussion",
			ModeHint: "prd",
		}
		tWrite := product.WriteRequest{
			Key:         "key-thread-1",
			RequestHash: hashPayload(threadInput),
			Resource:    fmt.Sprintf("/api/v2/projects/%s/threads", projA.ID),
		}

		// Bob cannot create a thread in Alice's project
		_, err = repo.CreateThread(ctx, userB, projA.ID, threadInput, tWrite)
		if !errors.Is(err, product.ErrNotFound) {
			test.Fatalf("expected ErrNotFound for cross-owner thread creation, got %v", err)
		}

		threadA, err := repo.CreateThread(ctx, userA, projA.ID, threadInput, tWrite)
		if err != nil {
			test.Fatalf("failed to create thread: %v", err)
		}
		if threadA.Title != "PRD Discussion" || threadA.ModeHint != "prd" {
			test.Fatalf("unexpected thread: %+v", threadA)
		}

		// Cross-project active artifact isolation check
		projBInput := product.ProjectInput{Title: "Bob Project"}
		projB, err := repo.CreateProject(ctx, userB, projBInput, product.WriteRequest{
			Key:         "key-proj-b",
			RequestHash: hashPayload(projBInput),
			Resource:    "/api/v2/projects",
		})
		if err != nil {
			test.Fatal(err)
		}
		bobArtifactID, _ := createFixtureArtifact(test, db, projB.ID, "bob-art")

		badThreadInput := product.ThreadInput{
			Title:            "Sneaky Thread",
			ActiveArtifactID: &bobArtifactID,
		}
		_, err = repo.CreateThread(ctx, userA, projA.ID, badThreadInput, product.WriteRequest{
			Key:         "key-bad-thread",
			RequestHash: hashPayload(badThreadInput),
			Resource:    fmt.Sprintf("/api/v2/projects/%s/threads", projA.ID),
		})
		if !errors.Is(err, product.ErrNotFound) {
			test.Fatalf("expected ErrNotFound for cross-project active artifact, got %v", err)
		}

		// 4. List Threads Isolation
		threadsA, err := repo.ListThreads(ctx, userA, projA.ID, product.PageQuery{Limit: 10})
		if err != nil {
			test.Fatal(err)
		}
		if len(threadsA.Items) != 1 || threadsA.Items[0].ID != threadA.ID {
			test.Fatalf("expected 1 thread for Alice, got %d", len(threadsA.Items))
		}

		_, err = repo.ListThreads(ctx, userB, projA.ID, product.PageQuery{Limit: 10})
		if !errors.Is(err, product.ErrNotFound) {
			test.Fatalf("expected ErrNotFound for Bob listing Alice's threads, got %v", err)
		}

		// 5. Post Messages & Sequence Numbering
		msgInput1 := product.MessageInput{
			Content:         "We need a payment processing module.",
			ClientMessageID: "c-msg-1",
		}
		mWrite1 := product.WriteRequest{
			Key:         "key-m-1",
			RequestHash: hashPayload(msgInput1),
			Resource:    fmt.Sprintf("/api/v2/projects/%s/threads/%s/messages", projA.ID, threadA.ID),
		}

		// Bob cannot post to Alice's thread
		_, err = repo.PostUserMessage(ctx, userB, projA.ID, threadA.ID, msgInput1, mWrite1)
		if !errors.Is(err, product.ErrNotFound) {
			test.Fatalf("expected ErrNotFound for Bob posting to Alice's thread, got %v", err)
		}

		msg1, err := repo.PostUserMessage(ctx, userA, projA.ID, threadA.ID, msgInput1, mWrite1)
		if err != nil {
			test.Fatalf("failed to post message 1: %v", err)
		}
		if msg1.Role != "user" || msg1.SequenceNumber != 1 {
			test.Fatalf("unexpected message 1: %+v", msg1)
		}

		// Replay same client_message_id with same content -> returns existing message
		replayMsg1, err := repo.PostUserMessage(ctx, userA, projA.ID, threadA.ID, msgInput1, product.WriteRequest{
			Key:         "key-m-1-replay",
			RequestHash: hashPayload(msgInput1),
			Resource:    fmt.Sprintf("/api/v2/projects/%s/threads/%s/messages", projA.ID, threadA.ID),
		})
		if err != nil {
			test.Fatalf("replay message failed: %v", err)
		}
		if replayMsg1.ID != msg1.ID {
			test.Fatalf("expected same message ID on replay, got %s vs %s", replayMsg1.ID, msg1.ID)
		}

		// Replay same client_message_id with DIFFERENT content -> ErrMessageConflict
		conflictMsgInput := product.MessageInput{
			Content:         "A completely different request body",
			ClientMessageID: "c-msg-1",
		}
		_, err = repo.PostUserMessage(ctx, userA, projA.ID, threadA.ID, conflictMsgInput, product.WriteRequest{
			Key:         "key-m-1-conflict",
			RequestHash: hashPayload(conflictMsgInput),
			Resource:    fmt.Sprintf("/api/v2/projects/%s/threads/%s/messages", projA.ID, threadA.ID),
		})
		if !errors.Is(err, product.ErrMessageConflict) {
			test.Fatalf("expected ErrMessageConflict, got %v", err)
		}

		// Post message 2 -> sequence_number must be 2
		msgInput2 := product.MessageInput{
			Content:         "Let's also add Stripe webhooks.",
			ClientMessageID: "c-msg-2",
		}
		msg2, err := repo.PostUserMessage(ctx, userA, projA.ID, threadA.ID, msgInput2, product.WriteRequest{
			Key:         "key-m-2",
			RequestHash: hashPayload(msgInput2),
			Resource:    fmt.Sprintf("/api/v2/projects/%s/threads/%s/messages", projA.ID, threadA.ID),
		})
		if err != nil {
			test.Fatal(err)
		}
		if msg2.SequenceNumber != 2 {
			test.Fatalf("expected sequence 2, got %d", msg2.SequenceNumber)
		}

		// 6. List Messages with Sequence Ordering & Cursors
		msgList, err := repo.ListMessages(ctx, userA, projA.ID, threadA.ID, product.PageQuery{Limit: 1})
		if err != nil {
			test.Fatal(err)
		}
		if len(msgList.Items) != 1 || msgList.Items[0].SequenceNumber != 1 || msgList.NextCursor == nil {
			test.Fatalf("unexpected first page of messages: %+v", msgList)
		}

		msgList2, err := repo.ListMessages(ctx, userA, projA.ID, threadA.ID, product.PageQuery{Limit: 1, Cursor: *msgList.NextCursor})
		if err != nil {
			test.Fatal(err)
		}
		if len(msgList2.Items) != 1 || msgList2.Items[0].SequenceNumber != 2 {
			test.Fatalf("unexpected second page of messages: %+v", msgList2)
		}

		// Bob cannot list messages in Alice's thread
		_, err = repo.ListMessages(ctx, userB, projA.ID, threadA.ID, product.PageQuery{Limit: 10})
		if !errors.Is(err, product.ErrNotFound) {
			test.Fatalf("expected ErrNotFound for Bob listing messages, got %v", err)
		}
	})

	test.Run("concurrent_idempotent_creates", func(test *testing.T) {
		db := newFixture()
		if err := RunMigrations(ctx, db); err != nil {
			test.Fatal(err)
		}
		user := "33333333-3333-3333-3333-333333333333"
		executeFixture(test, db, `INSERT INTO access_credentials(id, name, credential_key) VALUES ($1, 'Charlie', 'hashC')`, user)

		repo := productrepo.NewPostgresRepository(db)
		input := product.ProjectInput{Title: "Concurrent Project"}
		write := product.WriteRequest{
			Key:         "concurrent-key-1",
			RequestHash: hashPayload(input),
			Resource:    "/api/v2/projects",
		}

		const concurrency = 6
		results := make([]*product.Project, concurrency)
		errs := make([]error, concurrency)

		var wg sync.WaitGroup
		wg.Add(concurrency)
		for i := 0; i < concurrency; i++ {
			idx := i
			go func() {
				defer wg.Done()
				results[idx], errs[idx] = repo.CreateProject(ctx, user, input, write)
			}()
		}
		wg.Wait()

		var successID string
		for i := 0; i < concurrency; i++ {
			if errs[i] == nil {
				if successID == "" {
					successID = results[i].ID
				} else if results[i].ID != successID {
					test.Fatalf("concurrent creates produced divergent IDs: %s vs %s", results[i].ID, successID)
				}
			} else if !errors.Is(errs[i], product.ErrIdempotencyInFlight) {
				test.Fatalf("unexpected concurrent error: %v", errs[i])
			}
		}

		// Total projects created in DB must be exactly 1
		assertCount(test, db, 1, "SELECT count(*) FROM projects WHERE user_id = $1", user)
	})
}
