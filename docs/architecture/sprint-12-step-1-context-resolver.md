# Sprint 12 Step 1: Project Context Resolver

Date: 2026-10-28
Status: Implemented; backend unit, race, contract and disposable PostgreSQL integration checks pass.

## Scope

This slice previews the exact project, artifact revisions and chat messages that
can become input for an AI run. It does not invoke an LLM, create a generation
run, write an assistant message or change an artifact. No database migration,
production deployment or frontend change is needed in this step.

Implementation is in the primary checkout at
`/var/www/project-diagram/backend-diagram`, not a worktree.

## API

`POST /api/v2/projects/{projectId}/threads/{threadId}/context/resolve`

Requires the existing Bearer credential. This is a read-only POST preview, not a
creation/action operation, so no `Idempotency-Key` is required. A repeat request
may return a newer preview if the project changes. The OpenAPI now documents 52
operations matched against the router.

Example explicit selection:

```json
{
  "mode": "explicit",
  "sources": [
    {
      "artifact_id": "44444444-4444-4444-8444-444444444444",
      "revision_id": "55555555-5555-4555-8555-555555555555"
    }
  ],
  "thread_ids": [],
  "max_content_bytes": 49152
}
```

- `explicit` is the default; zero artifact sources is valid.
- Omit `revision_id` to resolve that artifact's current revision in the snapshot.
- Explicit source priority follows request order. Duplicate artifact selections,
  invalid identifiers and contradictory include/exclude lists are rejected.
- `project_relevant` selects up to 50 newest non-archived artifact heads, ordered
  by `updated_at DESC, id DESC`. This is a deterministic recency policy, not
  semantic relevance or an LLM claim. `sources` must be empty in this mode.
- `excluded_artifact_ids` keeps selected artifacts out of automatic context.
- The active thread is included automatically. Up to five additional same-project
  `thread_ids` can be selected; other threads are not read implicitly.
- Each thread contributes its newest 20 user/assistant messages. Stored system
  and tool rows are not promoted into trusted system instructions.

## Manifest And Budget

The success envelope contains `data` with project metadata, exact included
artifact/revision/version IDs, typed content bodies, message IDs and sequence
numbers, omissions, budget usage and `preview_only: true`.

Context body variants are `{markdown: string}`, `{nodes: array, edges: array}`
or the existing frame JSON object. No templates or synthetic fallback content
are introduced.

The content budget defaults to 48 KiB and supports 1-128 KiB. It counts the
project title/description UTF-8 bytes, serialized source JSON body bytes from
PostgreSQL, and raw message UTF-8 bytes. It is not a tokenizer count, complete
HTTP response-size limit or a promise that an eventual provider request fits.
The provider integration must reserve prompt/output overhead independently.

Priority: project metadata, selected artifact bodies, then active-thread recent
messages and additional-thread recent messages in selection order. Messages are
returned in chronological sequence order after selection. Sources/messages that
do not fit are omitted whole with `content_budget` and their exact IDs. JSON and
text are never silently sliced. The reader avoids transferring oversized bodies
from PostgreSQL. An oversized required project description returns 422.

`candidate_limit_reached` reports that automatic selection did not enumerate all
artifacts. `history_limited_thread_ids` reports threads with older messages beyond
the 20-message window. These limits are distinct from per-item budget omissions.

## Isolation And Module Boundaries

- Controller: authenticated actor from server context, strict request decoder,
  32 KiB request limit, 10-second context deadline and V2 error envelopes.
- Service: selection validation, normalization, deterministic budget policy and
  manifest assembly behind a consumer-owned `ContextReader` port.
- Integration adapter: `internal/integration/contextgateway` implements the port
  using SQL. Generation does not import product/artifact modules or repositories.
- Access to the project and active thread is checked together. Every selected
  thread and artifact/revision must belong to that project, even if the same
  user owns other projects. Missing and unauthorized refs share a 404 response.
- All reads occur in one read-only repeatable-read transaction so the preview
  cannot combine heads from different database snapshots. Failures return no
  partial context. Archived artifacts are not eligible sources.
- Future generation must reauthorize, resolve and persist a frozen manifest at
  acceptance. A preview is not authorization or durable run lineage. Source
  content remains untrusted data; this endpoint does not execute it.

## Validation

```sh
make verify
make test-race
make lint-spec
RUN_DATABASE_INTEGRATION_TESTS=1 go test ./internal/platform/database -v -count=1
```

Integration tests start and stop their own temporary PostgreSQL clusters with
temporary fixture databases. They do not use application `.env` credentials or
mutate the development/production database. Tests cover owner and project
isolation, mismatched revisions, current/historical ref selection, exclusions,
budget omissions, history limits, body validation, safe errors and SQL failures.

The existing React client also passes `npm run build` and `npm run lint`; this
step adds no context-selection UI yet.

## Remaining Sprint 12 Slices

1. Step 1: Context preview/resolver and isolation checks. **Done.**
2. Step 2: Document-generation engine for brainstorming/PRD using the existing
   provider port, with mock-based tests and no hardcoded document templates.
3. Step 3: Persisted generation runs, frozen inputs, idempotency, cancellation,
   authorized progress and atomic artifact/message commits.
4. Step 4: React source selection, generation progress, failure/retry and result
   navigation, plus end-to-end verification.

Diagram engines and wireframe/UI-design migration remain separate subsequent
sprints. Decision approval/review resolution is not implemented by this slice;
unsupported request fields are rejected instead of silently ignored.
