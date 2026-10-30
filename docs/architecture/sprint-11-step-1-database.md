# Sprint 11 Step 1: Connected Workspace Database Foundation

Date: 2026-10-28
Status: implemented and verified against a disposable PostgreSQL 16 instance.

## Delivered

- Additive migration for project containers, threads, messages, artifacts,
  immutable revisions, context lineage, decisions, generation runs and
  idempotency storage. Existing credentials and legacy tables are retained.
- Composite foreign keys enforce project boundaries at the database layer.
- Embedded SQL with a checksum ledger, advisory transaction lock and atomic
  migration execution. Startup fails instead of serving a partially migrated
  schema when migration fails.
- Idempotent legacy import preserving messages, historical snapshots and the
  current canvas. Imported projects are recorded to prevent duplicate imports.
- Regression tests for new and previously applied migrations, checksum drift,
  baseline errors, backfill errors and commit errors.

## Validation

- `go test ./...`: passed.
- `make lint-spec`: passed; existing HTTP contracts remain unchanged.
- `make test-race`: passed.
- `RUN_DATABASE_INTEGRATION_TESTS=1 go test ./internal/platform/database -run '^TestConnectedWorkspaceIntegration$' -v -count=1`: passed.

Integration coverage includes fresh and repeated migration, legacy history,
idempotent backfill, same-project constraints, revision immutability, atomic
failure and checksum drift. The fixture starts its own temporary PostgreSQL
cluster; these tests do not migrate the running application's database.

## Operational Boundary

No application restart, deployment or migration against the running database
was performed during this verification. Migration runs on application startup.
Backfill is a one-time snapshot per legacy project, not ongoing synchronization
between legacy writes and the future v2 API. Cutover must account for any legacy
edits after import before the new interface becomes authoritative.

## Next Steps

1. Step 2: product/thread repository, service and controller with owner-scoped
   access and `/api/v2` contracts.
2. Step 3: artifact/revision APIs, optimistic concurrency and append-only rollback.
3. Step 4: React integration and manual end-to-end verification.

Sprint 11 is not complete until these remaining steps are implemented and
verified. This step does not yet expose new APIs or connected AI context.
