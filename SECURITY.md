# Security

## Secret handling

Runtime secrets must be supplied through environment variables or the local `.env` file, which is ignored by Git. Built-in provider, internal-service, database-password, and administrator-key fallbacks have been removed. Previously committed values remain in Git history and must be rotated; this change does not revoke them.

Generate strong values with:

```bash
openssl rand -hex 32
```

## Required environment variables

| Variable | Required | Notes |
| --- | --- | --- |
| `DB_PASSWORD` | Yes in development Compose | The development stack refuses to start without it. Other deployments must match their database authentication configuration. |
| `INTERNAL_SERVICE_KEY` | Yes | Shared secret for Go-to-Python and callback requests. Recommend at least 32 bytes. |
| `OPEN_AI_API_KEY` | Yes for the Compose AI stack | Go also accepts `OPENAI_API_KEY` or `AI_API_KEY`, with `OPENAI_API_KEY` taking precedence. Python prefers `OPEN_AI_API_KEY`. Set only one alias to avoid ambiguity. |
| `ANTHROPIC_API_KEY` | Yes for `AI_PROVIDER=anthropic` | Used by the Go backend only. |
| `MASTER_ADMIN_KEY` | First boot only | 32-64 bytes. Used only when the master administrator row does not exist. It never overwrites an existing credential. |

The Go backend validates `INTERNAL_SERVICE_KEY` and the selected provider key at startup and exits with a clear error instead of running with an empty secret. This happens before connecting to PostgreSQL or Redis. The Python UI Design runtime remains OpenAI-compatible only; selecting Anthropic in Go does not switch the Python runtime to Anthropic.

## Master administrator bootstrap

The old default administrator key was committed to the repository and must be treated as compromised.

1. Rotate an exposed administrator key through an authorized database maintenance operation. The current admin API does not provide a credential-key rotation endpoint.
2. Set `MASTER_ADMIN_KEY` to a new random 32-64 byte value before the first start on a fresh database.
3. Do not reuse `MASTER_ADMIN_KEY` as a normal user credential.
4. Existing administrator rows are left untouched on restart, so changing `MASTER_ADMIN_KEY` later does not rotate the credential automatically.

`internal/platform/database/init.sql` no longer creates or updates administrator credentials. `EnsureMasterAdmin` creates the account once and adopts orphan projects in the same transaction.

Restart preserves all existing administrator fields, including the key, role, active state, permissions, and expiry. `MASTER_ADMIN_KEY` is a bootstrap input, not an ongoing password reset mechanism; it may be unset after initial creation. Credentials in `access_credentials` are hashed with SHA-256 hex digests (`credential_key`) alongside a non-secret prefix column (`credential_key_prefix`).

## Hashed credential storage and migration

- The application stores SHA-256 digests rather than raw bearer tokens. Incoming tokens are hashed before database lookup; lookup errors no longer include the submitted token. This does not erase plaintext retained in pre-migration backups, logs, WAL, or old database pages.
- New credentials generate high-entropy tokens formatted as `RL-` followed by 32 cryptographically secure bytes encoded with base64url (46 characters total). Entropy generation failures return an error rather than falling back to timestamps or pseudorandom seeds.
- `POST /api/admin/credentials` returns the generated raw token exactly once. All subsequent reads (`GET /api/admin/credentials`, `PUT /api/admin/credentials/{id}`, `GET /api/auth/me`) return a display mask (`<prefix>********` or `********`), never the raw token or database digest.
- `POST /api/auth/verify` returns the verified submitted bearer token in `token`, while `user.credential_key` provides the masked display value.

### Startup migration procedure

The backend executes `MigrateCredentialKeys` at startup before accepting traffic:

1. Obtains an exclusive table lock: `LOCK TABLE access_credentials IN EXCLUSIVE MODE`.
2. Finds all legacy rows where `credential_key_prefix = ''`.
3. Replaces `credential_key` with its SHA-256 digest and sets `credential_key_prefix = 'hidden'`.
4. Commits the transaction. Any error aborts startup and rolls back all row updates.
5. Idempotent: rows already migrated (`credential_key_prefix <> ''`) are skipped.

### Operational and deployment runbook

- **Stop all old instances**: Perform this migration in a maintenance window. Never run old and new binaries against the migrated database together: an old binary would accept a stored digest as a bearer credential, bypassing the new protection.
- **Database snapshot**: Take a full PostgreSQL snapshot/backup before upgrading.
- **Irreversible**: Once credentials are converted to one-way digests, reverting to an older binary without restoring a pre-migration database snapshot will break token verification.
- **Backup retention**: Pre-migration backups and PostgreSQL WAL archives contain plaintext credentials and must be handled as sensitive secrets until purged under retention policies.
- **Weak key rotation**: Hashing does not increase the entropy of short or previously exposed keys. Legacy keys (such as default administrator credentials) must still be replaced.
- **Frontend note**: The current admin UI contains a copy/reveal toggle that copies the displayed value. For existing credentials, this copies the masked string (`RL-XXXX-********`). The one-time banner shown upon creation is the only opportunity to copy the raw token.


## Secret rotation

Rotate the OpenAI-compatible provider key, `INTERNAL_SERVICE_KEY`, database password, and master administrator credential if any of them have ever appeared in a commit, log, screenshot, or shared environment file. Revoke the old provider key at the provider. Coordinate an internal-service key change across Go, the Python gateway, and the worker. Updating a database password environment variable does not change an existing database role's password; rotate that role separately. Never delete a database volume to rotate credentials.

## Resource ownership and internal callbacks

- Project comments (list, create, status update, delete) and exports require authentication and ownership of the parent project. Foreign or missing resources return 404, including for administrators accessing another user's project.
- Async UI dispatch creates the project under the authenticated user and persists its job/project/user/correlation binding in PostgreSQL (`async_job_bindings`). Status reads verify that binding and project ownership before returning Redis status.
- Worker callbacks use `X-Internal-Service-Key`, not a user bearer token. An absent configured secret, missing header, or incorrect key fails closed with 401. The route middleware and callback handler both enforce this check.
- A callback must match the dispatched job and project, and its correlation ID is checked when supplied. A mismatch returns 409; missing job/project identifiers return 400; a missing legacy project returns 404. Legacy callbacks without a binding may use project metadata or the previous Redis project mapping.
- Foundations and settings reads remain shared workspace data available to authenticated users. Updating global settings requires an administrator; ordinary users receive 403.

### Deployment and consistency limitations

The idempotent startup schema adds `async_job_bindings`. It has not been applied to a live database as part of this refactor. Verify the table exists before enabling the updated async routes.

Project creation and binding insertion are separate operations, not one transaction. A binding failure can leave an orphan queued project, although the job is not enqueued. Transactional dispatch/outbox work remains deferred.

Legacy jobs without a binding return 404 on status reads, even if their Redis status remains available (the current status TTL is two hours). Callback compatibility does not backfill bindings. The binding table currently permits one job per project; retry/regeneration workflows must revisit that constraint.

No production deployment or secret rotation is performed by these code changes.

## Local checks

```bash
make verify
make test-race
make lint-spec
```

The default test suite is offline and does not require provider credentials. Live provider tests remain opt-in through `RUN_LIVE_AI_TESTS=1`; see `TESTING.md`.
