# Sprint 11 Step 2: Product & Thread Domain Implementation

Date: 2026-10-28
Status: Implemented and 100% verified against unit tests and disposable PostgreSQL 16.

## 1. Overview & Delivered Scope

Step 2 delivers the core container and multi-thread conversation domain (`internal/modules/product`) in the Go modular monolith.

### Delivered Endpoints under `/api/v2`

| Method | Endpoint | Description |
| --- | --- | --- |
| `POST` | `/api/v2/projects` | Creates an isolated project container with safe replay protection (`Idempotency-Key`). |
| `GET` | `/api/v2/projects` | Lists projects owned by the authenticated user with opaque cursor pagination. |
| `POST` | `/api/v2/projects/{projectId}/threads` | Creates a focused chat thread (e.g. Brainstorming, PRD, Architecture) within the project. |
| `GET` | `/api/v2/projects/{projectId}/threads` | Lists all chat threads in the specified project. |
| `POST` | `/api/v2/projects/{projectId}/threads/{threadId}/messages` | Posts a user message. Replay protection via `client_message_id` and `Idempotency-Key`. |
| `GET` | `/api/v2/projects/{projectId}/threads/{threadId}/messages` | Retrieves messages chronologically sorted by `sequence_number ASC` with sequence cursors. |

---

## 2. Strict Domain Invariants Enforced

1. **Owner Isolation & Boundary Defense**:
   - Every read and write query checks `projects.user_id = $userID`.
   - Accessing a thread or project owned by another user yields `404 RESOURCE_NOT_FOUND` (no authorization or existence leak).
   - Threads referencing an `active_artifact_id` verify that the artifact belongs to the same project (`FOR SHARE` check), preventing cross-project context leaks.

2. **Role Forgery Prevention**:
   - `PostUserMessage` hardcodes `role = 'user'`.
   - Clients cannot spoof `assistant` or `system` messages through the API.

3. **Replay Protection & Idempotency**:
   - Post endpoints require the `Idempotency-Key` header (1..128 printable ASCII characters).
   - Replays with identical request hash return the cached 201 response with zero duplicated database rows.
   - Replays with a different payload return `409 IDEMPOTENCY_KEY_REUSED`.
   - Concurrent requests on the same key are serialized using transactional advisory locks (`pg_advisory_xact_lock`), returning `409 RESOURCE_CONFLICT` (in-flight) if concurrent execution attempts collide.

4. **Message De-duplication**:
   - Posts require `client_message_id`. Re-sending the identical content returns the existing message.
   - Re-sending with different content yields `409 RESOURCE_CONFLICT`.

5. **Opaque Scoped Cursor Pagination**:
   - Standard URL-safe base64 encoded cursor tokens incorporating scope, timestamp/sequence, and UUID.
   - Cross-scope cursor tampering is rejected with `400 INVALID_REQUEST`.

---

## 3. Verification & Quality Gates

All checks pass offline and against live test databases:
1. `make verify`: 100% Go unit tests clean across all modules.
2. `make test-race`: Zero data races across all packages under concurrent load.
3. `make lint-spec`: OpenAPI 3.0.3 contract synchronized (44 operations matched against `internal/app/routes.go`).
4. Live Integration Test (`RUN_DATABASE_INTEGRATION_TESTS=1 go test ./internal/platform/database -run '^TestProductRepositoryLiveIntegration$' -v -count=1`):
   - Verified project lifecycle, owner isolation between Alice and Bob, thread creation and isolation, message sequence generation, replay protection, and concurrent creates under 6 concurrent goroutines.

---

## 4. Next Step: Sprint 11 Step 3

Next is **Sprint 11 Step 3: Artifact & Revision Domain (`internal/modules/artifact`)**:
- Domain package for `artifacts`, `artifact_revisions`, and `artifact_context_links`.
- Append revision with optimistic concurrency check (`expected_revision_id`).
- Rollback implementation: appending version $N+1$ with content of version $V$.
- HTTP endpoints: `POST /api/v2/projects/{projectId}/artifacts`, `GET /api/v2/projects/{projectId}/artifacts`, `GET /api/v2/projects/{projectId}/artifacts/{id}/revisions`, `POST /api/v2/projects/{projectId}/artifacts/{id}/revisions`, and restore/rollback.
