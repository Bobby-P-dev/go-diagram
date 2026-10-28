# Testing

## Quick reference

| Command | Purpose |
| --- | --- |
| `make test` | Offline test suite |
| `make verify` | `go vet` plus the offline suite |
| `make test-race` | Race detector on internal modules, platform, app composition, and legacy facades |
| `make lint-spec` | OpenAPI contract validation and route parity |
| `make test-live` | Opt-in provider tests that may consume tokens |

## Default suite (offline)

```bash
make test
```

This is the suite used for normal development and CI. It must not contact an LLM provider and does not require an API key.

Use `make verify` to run `go vet` and the offline test suite together.

## API contract

`docs/api/openapi.yaml` is the source of truth for the HTTP contract. It is written in **OpenAPI 3.0.3** and currently documents **38 operations** across 31 paths.

```bash
python3 -m venv .venv
.venv/bin/python -m pip install PyYAML==6.0.3
PATH="$PWD/.venv/bin:$PATH" make lint-spec
```

If PyYAML is already installed for your `python3`, run:

```bash
make lint-spec
```

`scripts/check_openapi.py` checks that:

- the YAML parses and declares OpenAPI 3.0.x;
- every operation has a unique `operationId`, a `summary`, declared `tags`, and responses;
- every `$ref` resolves locally;
- the unsupported `const` keyword is not used in the 3.0 document;
- every route registered in `internal/app/routes.go` exists in the spec, and vice versa.

This is a lightweight structural check, not a full OpenAPI semantic validator or a request/response conformance test.

`src/routes/contract_test.go` also compares method/path pairs against the Go router's AST (`internal/app/routes.go`) in both directions, so the normal offline suite catches route drift even when Python is unavailable. That test expects the contract's current two-space indentation and unquoted path keys; `make lint-spec` parses YAML independently.

When a route is added, removed, or renamed, update `internal/app/routes.go` and `docs/api/openapi.yaml` in the same change. If the operation count changes intentionally, update `EXPECTED_OPERATIONS` in `scripts/check_openapi.py`.

## Live AI tests (opt-in)

```bash
make test-live
```

Live tests call the configured AI provider and may consume tokens or incur cost. They only run when `RUN_LIVE_AI_TESTS=1` is set explicitly; otherwise they are skipped.

An opted-in live test fails before contacting the provider if its configured API key is missing. No provider or internal-service secret has a built-in fallback. See `SECURITY.md` for startup requirements and admin bootstrap behavior.

## Current limitation

The LLM client does not yet receive the request context. A live test can therefore keep waiting on the provider's HTTP timeout even after the test context expires. Context propagation is tracked for the UI Design refactor sprint.

The transitional identity facade also uses `context.Background()` because its legacy interface does not accept a request context. Repository and service methods already accept `context.Context`; controller/middleware extraction must propagate request cancellation before production hardening is considered complete.

## Characterization coverage

The offline suite locks these critical behaviors before refactoring:

- blank, template, and AI-backed project creation;
- project ownership checks on chat, canvas updates, and rollback;
- canvas update persistence;
- chat persistence of user/assistant messages and version increments;
- rollback restoring the target snapshot and recording a new version;
- authenticated fork forwarding the user, token, and title;
- async success callbacks persisting frames, versions, chat, and usage logs;
- async failure callbacks recording the worker error and returning failure;
- async dispatch persisting the requesting user's ownership and durable job binding;
- owner-only async status reads and rejected job/project callbacks;
- project-owner-only comment reads, creation, updates, deletion, and export reads;
- internal-service middleware failing closed for missing or incorrect secrets;
- administrator-only settings authorization;
- deterministic SHA-256 credential hashing and prefix extraction;
- masked credential display preventing raw token or digest leakage;
- transactional and idempotent legacy credential migration with rollback on batch failure;
- bearer authentication using hashed database lookup without echoing unauthenticated tokens;
- single-use raw token disclosure upon credential creation with hashed persistence;
- masked credential display across admin listings and authenticated profile endpoints;
- credential repository row mapping, prepared hash persistence, missing-row handling, canceled-context rejection, and partial-result rejection;
- identity service token normalization, digest/prepared-prefix persistence, one-time raw disclosure, masked reads, Master Admin update/delete protection, summary formatting, and context forwarding.

These checks use in-memory models, `httptest`, and a local in-process Redis test server. They do not establish live PostgreSQL migration behavior or a deployed Go/Python worker round trip.

### Step 3 PostgreSQL verification

An additional manual check ran against an isolated temporary PostgreSQL 16 cluster listening only on its own Unix socket, not the application database. It verified schema initialization, legacy token migration and login, rejection of a stored digest as a bearer token, idempotent migration, hashed administrator bootstrap, new credential creation and hashed persistence, and masked update/list responses. The temporary server was stopped afterward. This manual check is not part of the default offline suite or a production deployment test.

### Sprint 2 Step 1 verification

The legacy model tests now exercise the repository through the compatibility facade. Additional tests cover update policies (Master Admin protection, preserved versus explicitly cleared expiry), repository cancellation for all operations, row-read failure without partial results, and an import-boundary guard preventing the identity module from importing `src/` layers.

A separate isolated PostgreSQL 16 check also passed after repository extraction: schema initialization, repeated migration, legacy authentication and digest rejection, bootstrap/restart, legacy project adoption, credential creation/defaults, hashed persistence, last-login mapping, masked reads/lists, update/expiry clearing, protected Master Admin mutations/deletion, project counts, and persisted deletion. The server listened only on a fresh temporary Unix socket and was stopped afterward. No application database, VPS, or live AI provider was used; this manual check is not part of CI.

### Sprint 2 Step 2 verification

Service unit tests cover blank-token rejection without repository access, hashing and masking, one-time raw token disclosure with digest persistence, Master Admin update/delete protection, summary formatting, repository error propagation, and request-context forwarding. The isolated PostgreSQL 16 flow was re-run after service extraction through the compatibility facade and passed the same migration, authentication, bootstrap, CRUD, masking, expiry, permission, and project-count checks.

## Continuous integration

`.github/workflows/ci.yml` runs on all branch pushes and pull requests, and can be started manually. It sets `RUN_LIVE_AI_TESTS=0`, uses read-only repository permissions, and executes:

1. `make lint-spec`
2. `make verify`
3. `make test-race`

The same commands pass locally. The workflow has not yet been observed running on GitHub, so its first remote run still needs to be checked.
