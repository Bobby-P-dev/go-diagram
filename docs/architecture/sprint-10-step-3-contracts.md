# Sprint 10 Step 3: Module Boundaries and API Contracts

Status: proposed implementation contract, ready for review; no handlers, Go
interfaces, OpenAPI operations, or database migrations have been implemented.
Date: 2026-10-28.

## 1. Scope and Development Environment Decision

The user confirms that the target database is development and authorizes a
clean schema redesign. Keeping legacy storage or Vue compatibility indefinitely
is not a design requirement. This step specifies the target boundaries and API;
Step 4 will select and verify the database transition procedure. It does not
execute SQL, reset data, start workers, or call a provider.

The application remains a Go modular monolith. `/api/v2` distinguishes the new
resource model while it is implemented; it is not a microservice boundary or a
promise of permanent dual-write compatibility. Existing `/api` handlers remain
unchanged in this step. There is no requirement to finish the complete backend
before consuming one working slice in React.

Current source anchors inspected:

- `internal/app/app.go`: existing composition root and project gateway wiring.
- `internal/app/routes.go`: current diagram, UI design, identity, sharing, and
  workspace-enhancement routes, not the proposed v2 endpoints below.
- `internal/modules/workspace/`: existing comments, foundations, settings, and
  exports. Do not silently redefine this module as the new project container.
- `internal/modules/uidesign/contracts.go`: existing project gateway coupling;
  adapting generators to artifacts still requires implementation and tests.
- `internal/platform/httpresponse/response.go`: existing success/error helpers.

The earlier Step 2 DDL is a conceptual draft, not an executable migration. The
requirements in section 9 qualify it before implementation.

## 2. Ownership and Dependency Direction

These are proposed module names, not directories created by this step:

| Module | Owns | Must not own |
| --- | --- | --- |
| `identity` | Authenticated actor and generation permissions | Artifact bodies or product decisions |
| `product` (new) | Project containers, threads, messages, product decisions and their approval history | Diagram layout or UI compilation |
| `artifact` (new) | Artifact identities, immutable revisions, revision review records, exact context lineage | Provider-specific prompting or thread history |
| `generation` (new) | Run lifecycle, context selection/resolution, provider dispatch, cancellation and result orchestration | Independent copies of canonical artifacts |
| `diagram` | Graph generation, graph validation and graph-specific patches | New project/chat persistence |
| `uidesign` | Wireframe/UI generation, targeted patches, sanitization and export contracts | New project/chat persistence |
| `workspace` (existing) | Comments, foundations, settings, exports until explicitly reassigned | A duplicate of the new product aggregate |
| `sharing` | Explicit sharing/fork authorization | Automatic cross-project AI context |

```text
HTTP controllers -> product / artifact / generation application services
generation      -> consumer-owned access, thread, artifact and context ports
generation      -> diagram / uidesign / document-generation adapters
repositories    -> PostgreSQL; services receive interfaces, not database handles
internal/app    -> wires adapters and transaction coordination
```

Dependencies flow toward small domain ports; `artifact` does not import
`generation`, and generation engines do not call HTTP controllers or each
other. Consumer modules define narrow interfaces; integration adapters convert
between contracts rather than leaking another module's repository or records.

Retain controller/service/repository separation inside each module. Keep HTTP
DTOs in `dto/`, domain records/contracts at module root, and mapping at boundaries.
Do not create a broad shared service containing every domain's methods.

Generation adapters must eventually return validated candidates, without
persisting a legacy project. Existing persistence-coupled methods are reusable
logic candidates, not drop-in implementations of that new interface.

## 3. Domain Invariants

1. A project is an access and context boundary. A thread mode is a UI hint,
   not a fixed workflow or a restriction on output type.
2. Artifacts belong to projects, not exclusively to threads. A chat may create
   or revise many artifacts; several chats may reference the same artifact.
3. Every read/write validates the actor, project and nested resource together.
   A valid artifact UUID is not authorization. Initial policy is owner-only;
   collaboration and public sharing require explicit policies, not implicit
   admin or share-token access to AI context.
4. Clients cannot set message role to `assistant`/`system`, set an owner, approve
   a decision implicitly through generation, or declare a run successful.
5. Every committed content change creates a revision. Initial content and its
   artifact pointer commit together. Restore creates latest version + 1; e.g.
   restoring v2 while current is v7 produces v8, not v3.
6. Prompt, manual edit and restore all use the same optimistic-concurrency
   contract. Selection includes the revision on which element IDs were chosen.
7. Context references identify exact immutable revisions. A later source change
   never rewrites historical provenance or triggers an automatic downstream edit.
8. Approval applies to a specific revision, not to all future versions of an
   artifact. Editing an approved artifact creates a new draft revision.

## 4. Representative Go Ports

The snippets specify intent, not compile-ready source. Domain DTO definitions,
constructors, implementations and controller mapping come in execution sprints.
`Scope` is constructed from authenticated server context, never request JSON.

```go
type Scope struct {
    ActorID   string
    ProjectID string
}

type RevisionRef struct {
    ArtifactID string
    RevisionID string
}

type AccessGate interface {
    RequireProject(ctx context.Context, scope Scope) error
}

type ContextResolver interface {
    Resolve(ctx context.Context, scope Scope, input ResolveInput) (ResolvedContext, error)
}

type RevisionWriter interface {
    Append(ctx context.Context, scope Scope, input AppendRevisionInput) (Revision, error)
    Restore(ctx context.Context, scope Scope, input RestoreRevisionInput) (Revision, error)
}

type GenerationEngine interface {
    Generate(ctx context.Context, input EngineInput, emit ProgressSink) (Candidate, error)
}
```

`ResolveInput` contains thread ID, exact revision references, decision references
and a token budget. The resolver validates membership before reading content.
`ResolvedContext` records selected, included and omitted sources with reasons;
it does not claim that an excluded or truncated source was used.

`AppendRevisionInput` includes artifact ID, expected current revision ID,
content, change summary, source references and an idempotency key. SQL transaction
coordination uses the same transaction across the participating repositories
through a unit-of-work adapter; it must not become cross-module ad hoc SQL.

Persist accepted input/run metadata before inference. Do not hold a database
transaction open during an LLM call. Commit the validated revision, current
pointer, lineage, assistant output association and successful run result in one
short transaction after rechecking access and the expected revision. Failure
may leave an honest failed run and the user's message, never a partial successful
artifact. Step 4 will settle the transaction adapter and retry implementation.

## 5. HTTP Conventions

All proposed routes are under `/api/v2` and require Bearer authentication. Actor
identity comes from middleware; there is no client `user_id` override.
Nested resources use the complete `/projects/{projectId}/...` path. Cross-owner
or cross-project resource mismatches return `404 RESOURCE_NOT_FOUND`; a known
actor lacking a generation capability receives `403 CAPABILITY_DENIED`.

Collection responses use `data.items` and `data.next_cursor`. Default `limit`
is 20, range 1..100. Opaque cursors are validated against resource scope, filters
and stable sort: created time + ID descending, messages ascending by a
server-assigned per-thread sequence. Ordering must not rely on timestamps alone.

Success retains the current wrapper shape:

```json
{"status":"success","message":"Revision created","data":{"id":"revision-id"}}
```

V2 errors add a machine-readable `code` and `request_id` to the existing wrapper;
this is a proposed v2 change, not behavior of the existing helper today:

```json
{
  "status": "error",
  "message": "Artifact changed; reload before applying this revision.",
  "code": "REVISION_CONFLICT",
  "request_id": "request-id"
}
```

| Status | Codes / behavior |
| --- | --- |
| 400 | `INVALID_REQUEST`, malformed UUID/cursor, unknown request fields |
| 401 | `UNAUTHENTICATED` |
| 403 | `CAPABILITY_DENIED` for an authenticated, otherwise visible operation |
| 404 | `RESOURCE_NOT_FOUND` for absent or inaccessible nested resources |
| 409 | `REVISION_CONFLICT`, `RESOURCE_CONFLICT`, `IDEMPOTENCY_KEY_REUSED`, `RUN_NOT_CANCELABLE` |
| 413 | `PAYLOAD_TOO_LARGE` |
| 422 | `CONTENT_INVALID`, `TARGET_NOT_FOUND`, `CONTEXT_INVALID`, `GENERATOR_UNAVAILABLE` |
| 429 | `RATE_LIMITED`, with retry guidance |
| 500/503 | `INTERNAL_ERROR` / `SERVICE_UNAVAILABLE`; never expose SQL or secrets |

Provider failures after a run is accepted are stored on that run with a safe
error code, not converted into an apparently successful generation. Request
size, rate and token limits will be explicit config in implementation, not
invented hardcoded guarantees in this document.

## 6. Endpoint Catalog

Paths below are relative to `/api/v2`. `P` means `/projects/{projectId}`.
`T` means `P/threads/{threadId}`; `A` means `P/artifacts/{artifactId}`.
All lists follow section 5 pagination; responses below describe `data`.

| Method | Path | Input / response | Success |
| --- | --- | --- | --- |
| POST | `/projects` | `{title, description?}` -> Project; no initial LLM call | 201 |
| GET | `/projects` | Project summaries | 200 |
| GET | `P` | Project identity/metadata, not all message and revision bodies | 200 |
| PATCH | `P` | Title, description, pin, `expected_updated_at` | 200 |
| POST | `P/threads` | `{title, mode_hint?, active_artifact_id?}` -> Thread | 201 |
| GET | `P/threads` | Thread summaries | 200 |
| GET | `T` | Thread details | 200 |
| PATCH | `T` | Title, mode hint, focus, `expected_updated_at`; nullable focus clears it | 200 |
| GET | `T/messages` | Message history; no generation side effect | 200 |
| POST | `T/messages` | User-only `{content, client_message_id}`; no inference | 201 |
| POST | `P/artifacts` | Type, title, initial revision body, optional source message | 201 |
| GET | `P/artifacts` | Summaries, optional type filter | 200 |
| GET | `A` | Identity + current revision + revision review state | 200 |
| PATCH | `A` | Title/archive metadata with `expected_updated_at`; not content | 200 |
| GET | `A/revisions` | Revision summaries | 200 |
| GET | `A/revisions/{revisionId}` | Immutable full snapshot | 200 |
| POST | `A/revisions` | Manual content, expected revision ID, change summary | 201 |
| POST | `A/restores` | Target revision ID + expected current revision ID | 201 |
| POST | `A/revisions/{revisionId}/reviews` | Explicit approve/revoke action with expected prior review ID | 201 |
| GET | `A/lineage` | Current revision's exact sources + derived review-needed state | 200 |
| POST | `T/context/resolve` | Exact refs or `project_relevant` selection -> budgeted context preview | 200 |
| POST | `T/generations` | User message ID + output/target/context -> GenerationRun | 202 |
| GET | `P/generations/{runId}` | Run state, included refs, safe error or committed result | 200 |
| GET | `P/generations/{runId}/events` | Authorized fetch-based SSE stream | 200 |
| POST | `P/generations/{runId}/cancel` | Cooperative cancellation request | 202 |
| POST | `P/decisions` | Proposed decision text/category and optional exact source | 201 |
| GET | `P/decisions` | Decisions, optional status filter | 200 |
| POST | `P/decisions/{decisionId}/revisions` | New text/rationale, expected decision revision ID -> proposed revision | 201 |
| POST | `P/decisions/{decisionId}/reviews` | Exact decision revision ID + explicit approve/supersede/reject action, expected prior review ID | 201 |

All creation/action POSTs require `Idempotency-Key`. Server stores the request
hash, authenticated actor, resource scope and operation with the result. A same
key/same request returns the original result without another revision or model
call; changed input returns 409. An accepted run retry still returns its run
reference, including when the run has failed. A deliberate retry uses a new key.
Idempotency must be checked before a stale-base conflict on a replay.

Creating a message does not generate. The React flow posts a user message once
then starts a run referencing it; retries cannot silently append duplicate
messages. The message must belong to the same thread and project as the run.

### Revision Content DTO

Use a tagged union, not multiple conflicting content columns in a request:

| `format` | Required content | Validation |
| --- | --- | --- |
| `markdown` | `markdown: string` | Document types such as brainstorm/PRD |
| `graph` | `nodes: array`, `edges: array` | Diagram type, unique IDs and valid endpoints |
| `frame` | `nodes: array`, `edges: array` | Wireframe/UI canvas preserving existing UI frame schema |

Each body includes `schema_version: 1`. Reject incompatible type/format pairs
and content from more than one variant. Frame preview/export/canonical fields
must remain consistent with actual generator DTOs; React app migration alone
does not introduce a TSX export contract. UI output remains untrusted and needs
sanitization and isolated rendering; no sandbox library choice is forced here.

Artifact creation is one transaction including v1. No empty artifact is left
behind by failed inference. Metadata rename need not create a content revision.
Manual content edits are saved as meaningful changes, not every pointer movement.

### Generation Request Example

```json
{
  "message_id": "10000000-0000-4000-8000-000000000001",
  "output": {"type": "erd", "title": "Order data model"},
  "target": {
    "artifact_id": "20000000-0000-4000-8000-000000000002",
    "expected_revision_id": "30000000-0000-4000-8000-000000000003",
    "element_ids": ["table-orders"]
  },
  "context": {
    "mode": "explicit",
    "sources": [
      {
        "artifact_id": "40000000-0000-4000-8000-000000000004",
        "revision_id": "50000000-0000-4000-8000-000000000005"
      }
    ],
    "decision_revisions": []
  }
}
```

For a new artifact, omit `target`. For revision, both target IDs are required;
`element_ids` optionally restricts the patch scope and must exist in that base
revision. Output type must match the target. The exact target revision is also
captured as input even when no other context sources are selected.

Initial implementation produces one artifact per run to keep commit semantics
small. This does not restrict a thread to one artifact; multiple-output runs may
be added later with explicit transaction semantics.

`explicit` may select no upstream sources. `project_relevant` may choose only
authorized data in the active project; excluded sources stay excluded. Resolve
latest selections once at run acceptance, freeze exact revisions, and record
decision text/status as of the run. A preview is not a substitute for validation
and access checks when the run starts or commits. Untrusted source text cannot
override system instructions or authorization. Missing requirements stay labeled
as assumptions/questions, not automatically approved decisions.

### Run and Streaming Contract

Run states: `queued -> running -> succeeded | failed | canceled`; cancel requests
may expose `cancel_requested` before terminal cancellation. Final commit and
cancel transition are serialized: only one terminal outcome wins. Canceling an
already canceled run is idempotent; other terminal runs return 409 on cancel.

Run output references the committed artifact/revision and assistant message,
not just transient HTML. A conflict at commit marks the run failed with
`REVISION_CONFLICT` and does not overwrite the newer edit.

SSE event names: `status`, `progress`, `complete`, `error`. Each payload includes
`run_id` and monotonically increasing `sequence`; progress is advisory and may
have no token count. Heartbeats are SSE comments, not business events. `complete`
is emitted only after persistence commits, and `error` is terminal for a failed
or canceled run. Repeated subscribers do not start additional inference.

React uses authenticated `fetch` streaming, not native EventSource relying on
custom Bearer headers or a token embedded in a URL. Initial version does not
promise durable event replay. On disconnect, React reads run status, deduplicates
by run/sequence if reconnecting, and fetches the committed revision if complete.
A transport disconnect is not automatic cancellation of an accepted run. Restart
recovery must terminalize interrupted runs honestly if no resumable worker exists.

## 7. Cross-Artifact Revision and Approval

Lineage records the revisions actually used, attached to a committed downstream
revision. Arbitrary client writes to lineage are not exposed. Manual revisions
and restores explicitly preserve/copy the provenance of the base/restored content
and distinguish that from new AI-consumed context; they do not fabricate a fresh
model invocation.

`needs_review` is a derived relationship state comparing pinned source revisions
and current/approved source heads. Historical references never get repointed.
An archived source or a revoked decision approval is visible as a review reason.
A stale badge is not proof of semantic conflict and is not an instruction to
regenerate. An update to another artifact is a separate authorized run with the
user's chosen sources and expected destination revision.

Product decisions default to `proposed`. Changing approved decision text creates
a new proposed decision revision, preserving the prior approved text and review
events. Context can therefore explain exactly which decision was used earlier.
No wizard sequence, automatic prerequisite PRD, or automatic downstream rewrite
is introduced by these contracts.

## 8. Delivery Slices and Acceptance Cases

Sprint 11 implements the project/thread/artifact/revision foundation and one
React load-save-reopen slice. Context resolver, document generator and complete
run orchestration belong to Sprint 12; diagram migration to Sprint 13; wireframe
and UI migration to Sprint 14; cross-artifact review UX to Sprint 15. The catalog
is the target contract, not an obligation to ship every route in Sprint 11.
Unimplemented generators are rejected before provider invocation; scaffolded
routes must not falsely advertise success.

Required implementation tests include:

1. Project A identifiers combined with B thread/revision/decision/run IDs fail
   even when the same user owns both projects; another owner cannot read them.
2. Multiple threads can revise one artifact; one thread can produce several
   artifact types without switching a project-wide storage mode.
3. Two writes from one base revision yield one success and one 409; idempotent
   retries produce no duplicate message, revision, run or provider invocation.
4. Editing v7 from a stale v6 fails; restoring v2 from v7 creates v8 with the
   correct content/provenance, leaving previous revisions unchanged.
5. Invalid/cross-project sources are rejected before provider calls; automatic
   context reports selected/omitted revisions, and decision approval is explicit.
6. Provider, persistence, cancellation and SSE disconnect failures never produce
   a fake completion or partially committed current artifact pointer.
7. A source update changes review state without changing downstream content;
   targeted patches preserve non-targeted content in regression tests.

## 9. Step 2 Schema Corrections Required Before SQL Implementation

These requirements override conflicting illustrative DDL in Step 2. The exact
SQL and reset/backfill choice will be finalized in Step 4:

- Enforce same-project membership with composite foreign keys/unique keys for
  nested thread, message, artifact, revision, source, decision and run references.
  Separate foreign keys on two UUIDs do not enforce their relationship.
- Prefer one owner on project plus explicit creator/actor audit fields. Child
  ownership duplication must not create a second, inconsistent permission model.
- Store a current revision reference that must belong to the same artifact;
  retain a unique artifact/version pair and validate positive version numbers.
  Serialize appends with optimistic checks and a transaction, not unguarded MAX+1.
- Revision immutability requires application/DB role restrictions, not a label.
  Use archival for normal UX; destructive reset/deletion is a separate policy.
- Approval is revision-scoped; decisions default to proposed, not approved.
  Add decision revisions/review history and artifact revision reviews.
- Store pinned source revision IDs with enforced membership. A mutable
  `is_stale` column alone is not authoritative historical lineage.
- Add run records, frozen input/source manifests and idempotency persistence
  when introducing generation endpoints; do not pretend UUID arrays capture
  provenance, approval history or exact consumed versions.
- Validate tagged payloads/schema versions and all enum-like values. Preserve
  the existing frame DTO until a deliberate schema migration changes it.
- `CREATE TABLE IF NOT EXISTS` does not remodel existing tables. Development
  redesign permission allows a simpler verified transition, not an assumption
  that resetting any configured database is safe.

## 10. Validation and Next Step

This document is a design specification. Passing the existing OpenAPI/router
check validates the current API only, not these unimplemented v2 operations.
Do not add them to the served OpenAPI as if live. Detailed executable DTO/schema
validation and contract tests will accompany each implemented route.

Next: Step 4 selects the development schema transition, aligns the Step 2 draft
with these invariants, and defines an executable Sprint 11 checklist. No runtime
compatibility, database integrity, isolation or streaming guarantees are claimed
as already implemented by this planning step.
