# Sprint 11 Step 3: Artifact & Revision Domain Implementation

Date: 2026-10-28
Status: Implemented, verified against Go unit tests, database concurrency tests, and disposable PostgreSQL 16 cluster.

## 1. Overview & Delivered Scope

Step 3 delivers the immutable artifact, versioning, context lineage, and restore domain (`internal/modules/artifact`) in the Go modular monolith.

### Delivered Endpoints under `/api/v2`

| Method | Endpoint | Description |
| --- | --- | --- |
| `POST` | `/api/v2/projects/{projectId}/artifacts` | Creates an artifact and its initial immutable revision ($V=1$) atomically with idempotency protection. |
| `GET` | `/api/v2/projects/{projectId}/artifacts` | Lists project artifacts newest first with cursor pagination and optional `type` filter. |
| `GET` | `/api/v2/projects/{projectId}/artifacts/{artifactId}` | Retrieves artifact metadata along with its current revision detail and pinned context lineage. |
| `POST` | `/api/v2/projects/{projectId}/artifacts/{artifactId}/revisions` | Appends a new immutable revision ($V=N+1$) with optimistic concurrency verification (`expected_revision_id`). |
| `GET` | `/api/v2/projects/{projectId}/artifacts/{artifactId}/revisions` | Lists immutable revision summaries in descending version order with opaque version cursors. |
| `GET` | `/api/v2/projects/{projectId}/artifacts/{artifactId}/revisions/{revisionId}` | Retrieves a specific immutable revision by UUID. |
| `POST` | `/api/v2/projects/{projectId}/artifacts/{artifactId}/restores` | Appends a new revision ($N+1$) with the exact content and pinned context copied from a target revision. |

---

## 2. Invariants & Architecture Enforcement

1. **Owner Isolation & Boundary Defense**:
   - Every read and write validates that `projects.user_id = $userID`.
   - Accessing an artifact or revision belonging to another owner or outside the specified project returns `404 RESOURCE_NOT_FOUND`.
   - Referencing trigger messages (`trigger_message_id`) or context sources (`sources`) from another project fails with `404 RESOURCE_NOT_FOUND` and rolls back the transaction atomically.

2. **Artifact Immutability**:
   - Revisions are strictly append-only.
   - Database-level triggers enforce immutability: direct `UPDATE` or `DELETE` statements on `artifact_revisions` raise SQLSTATE `23514` (`check_violation`).

3. **Optimistic Concurrency Control**:
   - Appending a revision requires providing `expected_revision_id` matching the current head pointer of the artifact.
   - Concurrent appends or stale requests collide safely and return `409 REVISION_CONFLICT`.
   - Tested under multi-worker concurrent race conditions: exactly one write succeeds while concurrent writes on the same base revision are rejected with `409 REVISION_CONFLICT`.

4. **Restore Semantics (Append-Only History)**:
   - Restoring a prior revision does not alter or roll back historical rows.
   - A new revision ($N+1$) is created copying the target content and pinned context links.
   - `author_type` is tagged as `'restore'`, and provenance metadata is recorded (`restored_from_revision_id`, `restored_from_version_number`).

5. **Type & Payload Format Tagging**:
   - Format is strictly resolved from artifact type: `brainstorm` and `prd` resolve to `markdown`; `wireframe` and `ui_design` resolve to `frame`; all other types resolve to `graph`.
   - Payloads are validated against format constraints. Mismatched or invalid shapes return `422 CONTENT_INVALID`.

6. **Modular Monolith Separation**:
   - Packaged in `internal/modules/artifact/` with clear layers: `records.go`, `ports.go`, `validation.go`, `dto/`, `service/`, `controller/`, and `repository/`.
   - Architectural boundary test (`architecture_test.go`) enforces that `artifact` does not import `diagram`, `uidesign`, `workspace`, `product`, or infrastructure packages into service/controller layers.

---

## 3. Verification & Quality Gates

All checks pass offline and against live test databases:
1. `make verify`: Go vet and all package unit tests pass.
2. `make test-race`: Zero data races across all modules.
3. `make lint-spec`: OpenAPI 3.0.3 contract synchronized (51 operations registered and matching `internal/app/routes.go`).
4. Database Integration Tests (`RUN_DATABASE_INTEGRATION_TESTS=1 go test ./internal/platform/database -v -count=1`):
   - `TestArtifactRepositoryLiveIntegration`: Verified artifact and revision lifecycle, optimistic concurrency collision, idempotency replay, slug conflict, context link lineage, immutability trigger defense, restore provenance, and pagination.
   - `TestArtifactTriggerMessageIsolation`: Verified cross-project trigger message rejection.
   - `TestProductRepositoryLiveIntegration`: Verified project/thread/message domain isolation.
   - `TestConnectedWorkspaceIntegration`: Verified migration idempotent execution and backfill.

---

## 4. Next Step: Sprint 11 Step 4

Next is **Sprint 11 Step 4: React Workspace Client Migration**:
- Initialize clean React + TypeScript workspace.
- Connect client to `/api/v2` endpoints for project containers, multi-thread chats, and artifact revisions.
