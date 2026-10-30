# Sprint 12 Step 3: Generation Run Lifecycle

Date: 2026-10-29
Status: Implemented; offline unit, race, API contract and disposable PostgreSQL checks pass.

## Scope

Implements asynchronous `brainstorm` and `prd` generation in the primary checkout
`/var/www/project-diagram/backend-diagram`. No frontend file, production database,
running server, real API credential or deployed container was changed.
The React integration remains Step 4 and must preserve the user's own design,
not copy the legacy Vue project or introduce an unrelated design template.

## API

Bearer authentication and project ownership are required for all four routes:

- `POST /api/v2/projects/{projectId}/threads/{threadId}/generations`
- `GET /api/v2/projects/{projectId}/generations/{runId}`
- `POST /api/v2/projects/{projectId}/generations/{runId}/cancel`
- `GET /api/v2/projects/{projectId}/generations/{runId}/events`

Start requires `Idempotency-Key` (at most 128 bytes) and returns HTTP 202.
Example request, after persisting the user's prompt through the message API:

```json
{
  "trigger_message_id": "40000000-0000-4000-8000-000000000001",
  "output_type": "prd",
  "context": {"mode": "project_relevant"}
}
```

`output_title` is optional; omit it to let the engine produce a title.
To revise, supply both `target_artifact_id` and `target_expected_revision_id`.
The target must be an unarchived Markdown artifact with the same output type.
Its exact base document is frozen at acceptance; existing history is immutable.

Idempotency is scoped to user + project + key, including the thread in conflict
checks. Replays return the same run at its current status without resolving
new context or rerunning inference. Different payloads/threads return HTTP 409.
Keys are retained with the run, not expired after 24 hours; retry a failed run
using a new key. Unowned projects/runs and foreign trigger messages return 404.
Only a user message in the specified project and thread can trigger generation.

## Persistence And Execution

Additive checksum-tracked migration `05_generation_runs.sql` stores the accepted
manifest, prompt, base document, request hash, run status and committed IDs.
Composite foreign keys enforce project/thread boundaries. Migration 04 is unchanged.
The application will apply migration 05 on its next normal startup; this step
applied it only to disposable test databases.

The coordinator consumes consumer-owned context, engine and repository ports.
The repository writes the transaction across run, artifact and chat tables; it
does not import other domain modules or call their services independently.

- Two bounded workers per application process poll the durable PostgreSQL queue.
- Compare-and-set `queued -> running` prevents duplicate claims across workers.
- Request disconnect does not cancel an accepted job. Explicit cancel does.
- Inference has a four-minute timeout; **no database transaction is open while waiting for the LLM**.
- A six-minute lease lets another worker mark interrupted runs failed; it does
  not automatically rerun a possibly billed upstream request. Queued work survives restart.
- Cancellation is persisted before being observed by the worker (250 ms poll).
  Queued runs cancel immediately; running runs enter `cancel_requested`.
- Terminal states are `succeeded`, `failed`, and `canceled`. Canceling an already
  canceled run is idempotent; canceling succeeded/failed runs returns 409.

On a valid candidate, one short transaction locks the run, arbitrates cancel
versus commit, creates/appends the artifact revision, inserts the assistant
message with trigger/run/revision metadata, updates the thread's active artifact,
writes exact source links from the frozen manifest, and marks the run succeeded.
The revision head is checked under row lock again at commit. A concurrent edit
causes `REVISION_CONFLICT`, never a silent overwrite or partial assistant message.
Errors exposed to clients are bounded public messages, not raw upstream errors.

## SSE Contract

The stream polls persisted status, so reconnects and different server replicas
do not depend on process-local event buffers. Use authenticated `fetch`, not a
native EventSource URL containing the API key.

- `status`: `{ "status": "running" }` initially and whenever status changes.
- `progress`: five-second heartbeat with status and phase, not a completion percentage.
- `complete`: `{ "run": ... }` when succeeded or canceled, then close.
- `error`: public code/message and terminal run on failure, then close.

Raw LLM JSON/repair drafts are deliberately not streamed as committed content.
Each connection closes after four minutes; reconnect or GET the run to resume.
There is no token-event replay/Last-Event-ID history; reconnect returns current state.

## Validation

- `make verify`
- `make test-race`
- `make lint-spec`: 56 operations match the Go router.
- `RUN_DATABASE_INTEGRATION_TESTS=1 go test ./internal/platform/database -v -count=1`

Coverage includes replay/conflict, ownership and trigger isolation, pinned revision
lineage, optimistic conflict, cancel-before-commit, injected transactional rollback,
expired lease recovery, and real worker cooperative cancel with no idle transaction.
No real provider was contacted. Output quality with the user's actual prompts is
still a manual check after Step 4, not established by these offline tests.
