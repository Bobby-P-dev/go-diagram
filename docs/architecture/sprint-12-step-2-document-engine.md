# Sprint 12 Step 2: Document Generation Engine

Date: 2026-10-29
Status: Implemented and verified offline with fake clients and local HTTP providers.

## Scope

This step provides an internal, independently testable document engine for
`brainstorm` and `prd`. It consumes a resolved context manifest and returns a
validated Markdown candidate. It supports creating a draft or revising the
complete `base_document` supplied by the future run coordinator.

The engine is not yet exposed as a generation endpoint or wired into chat.
There are still 52 documented HTTP operations. No database migration, application
database write, frontend redesign, real provider invocation or deployment was
performed. Work is in `/var/www/project-diagram/backend-diagram`.

## Boundaries

- `internal/modules/generation/engine.go`: internal input, candidate, engine
  interface, progress callback and errors.
- `internal/modules/generation/service/document_engine.go`: validation, prompt
  policy, candidate construction and bounded repair.
- `internal/modules/generation/service/ports.go`: consumer-owned LLM port;
  generation does not import the platform LLM client or other domain modules.
- `internal/integration/llmgateway`: adapter to the existing configurable
  platform client. Provider, model and key remain environment configuration.
- `internal/platform/llm`: context-aware requests and retry waits, with legacy
  non-context methods retained as wrappers for existing callers.

There is no controller, HTTP request DTO or database repository for this slice:
the engine has no transport or persistence responsibility. Step 3 will own run
acceptance, authorization and commit coordination instead of making the engine
write artifacts directly.

## Output Contract, Not A Document Template

The model must return exactly four nonempty string fields:

- `title`: at most 255 characters; if a title was explicitly requested, it must
  match that title.
- `document_content`: the complete Markdown draft, at most 128 KiB.
- `change_summary`: at most 2048 bytes.
- `assistant_message`: at most 4096 bytes.

The engine attaches `type`, `format: markdown`, `schema_version: 1` and exact
included source references itself. The model cannot author IDs, provenance,
approval flags or message roles. Provenance records supplied inputs; it does not
prove that every source statement is reflected correctly in generated prose.

Only the JSON envelope is fixed. There is no prefilled document body, mandatory
heading order, generic title/summary fallback or canned success message. Raw
Markdown cannot bypass structured validation. A PRD can be generated without a
prior brainstorming artifact.

The system policy asks for grounded content, the user's language, relevant
trade-offs/acceptance criteria, labeled assumptions and open questions. It does
not prescribe invented SLA targets, technologies or approvals. Semantic quality,
truthfulness and preservation of unrelated paragraphs still need model/output
review; structural validation does not establish those properties.

## Context And Repair

The manifest, user instruction, requested title and optional base document are
serialized into one user-level JSON envelope. Source text and historical
messages are not interpolated into the system prompt or promoted into system
roles. The prompt explicitly treats reference content as untrusted data. This
reduces role confusion; it is not a guarantee against prompt injection.

Malformed or contract-invalid output receives at most two repair calls after
the initial attempt: three completions total. Every repair includes the original
input, exact source references, original revision content, failed response and a
safe validation error. It uses the same generation policy. Unknown fields,
duplicate keys, case-variant keys, trailing JSON, missing values and invalid
types are rejected. A complete outer JSON code fence can be removed without
rewriting strings inside the document. The general legacy JSON sanitizer is not
used to alter Markdown or silently remove text.

The serialized initial input is capped at 256 KiB, the user prompt at 16 KiB and
each raw response at 256 KiB. Repair envelopes remain bounded by those limits
but include both the original input and previous response; these byte limits do
not guarantee a provider token budget. Oversized raw output fails immediately.
The HTTP transport's existing bounded transient retries are separate from
semantic repair: up to three transport attempts per completion. A provider
failure is propagated, not converted into a fabricated document or another
semantic repair attempt.

Progress chunks are provisional provider JSON text, not validated Markdown or
evidence that a revision has committed. Consumers must not concatenate separate
repair/transport attempts into a canonical document. Step 3/4 should expose run
status and fetch the final validated result instead.

## Cancellation And Provider Compatibility

The platform client now offers `CallLLMContext`, `CallLLMStreamContext`,
`CallLLMTextContext` and `CallLLMTextStreamContext`. Request contexts reach the
HTTP client, body reads and retry backoff. The engine checks cancellation before
and after completions and never returns a candidate after cancellation.

Explicit provider truncation, filtering and tool-call termination are errors,
not successful text completions, even when the returned text happens to be valid
JSON. This behavior also applies to legacy wrappers. The implementation retains
existing streaming/standard-response support and JSON object mode for the
OpenAI-compatible path; it does not add provider-specific strict JSON Schema or
tool calling requirements.

The Meta-compatible mock verifies `/v1/chat/completions`, configured Bearer key,
`muse-spark-1.3`, streaming content and the engine/adapter/client chain. This is
local protocol coverage, not verification against a live Meta service. Existing
`.env.example` configuration is sufficient; actual credentials were not read,
changed or sent. Ensure higher-priority key aliases do not override the intended
`MODEL_API_KEY` when configuring the application later.

## Validation

Verified:

```sh
go test ./internal/modules/generation/... ./internal/integration/llmgateway/... ./internal/platform/llm/... -count=1
make verify
make test-race
make lint-spec
git diff --check
```

Tests cover draft creation, full-document revision inputs, source references,
stream callbacks, conservative fence handling, repair exhaustion, oversized
output, invalid/duplicate/unknown JSON fields, original context retained during
repair, provider failures, in-flight request cancellation, retry deadlines and
provider-declared incomplete output. The existing React build/lint also passes;
React source was not changed.

## Next: Sprint 12 Step 3

Implement persisted runs and frozen input snapshots; authorize the project,
trigger message, selected sources and revision target at acceptance. For a
revision, load the exact expected base from the database, not client-provided
Markdown. Preserve/merge lineage under the artifact contract. Apply optimistic
concurrency checks and atomically commit the artifact revision, assistant message
and successful run result. Add cancellation/state handling and authorized
progress without holding a database transaction open during model calls.

The internal engine does not replace those checks. Do not expose its raw
`DocumentInput` as a public endpoint that accepts a client-authored manifest.
