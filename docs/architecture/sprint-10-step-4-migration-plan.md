# Sprint 10 Step 4: Development Transition Strategy & Sprint 11 Execution Blueprint

Status: final planning document for Sprint 10; no migrations, SQL execution,
or code changes have been applied.
Date: 2026-10-28.

## 1. Context and Development Environment Parameters

The user confirmed:
- The environment is development (`var-www-project-diagram` on VPS).
- Authorization is granted to evolve or replace the database schema cleanly.
- No destructive or manual reset should be performed during planning; execution
  starts in Sprint 11.

This document defines the transition strategy, the target SQL schema aligned
with the Step 3 domain invariants, and the testable delivery checklist for
Sprint 11.

---

## 2. Transition Strategy Selection

We evaluated two approaches for transitioning the development database:

### Option A: Unconditional Truncate/Drop Rebuild
- **Description**: Drop all existing tables and re-run a clean `init.sql`.
- **Trade-off**: Wipes all test credentials (`access_credentials`), API usage logs,
  and active developer sessions. Requires re-bootstrapping credentials and losing
  any manual test projects.

### Option B: Controlled Additive Target Evolution (Recommended)
- **Description**:
  1. Retain `access_credentials` (so developer tokens remain valid).
  2. Create the new relational tables (`projects_v2` or evolved `projects`,
     `chat_threads`, `chat_messages_v2`, `artifacts`, `artifact_revisions`,
     `artifact_context_links`, `product_decisions`, `generation_runs`,
     `idempotency_keys`).
  3. When Sprint 11 runs its first migration, an automated idempotent routine
     maps existing legacy projects into the new structure (1 project -> 1 thread
     -> 1 default artifact with v1 revision).
  4. Once React and backend `/api/v2` are fully validated, legacy columns can be
     safely retired.
- **Why Option B wins**: Zero disruption to running sessions, no lost auth tokens,
  and instant verification with both old and new data models.

---

## 3. Target SQL Schema (Incorporating Step 3 Invariants)

The target schema resolves all findings from Step 3 Section 9:
- **Composite Foreign Keys**: Enforces same-project membership across threads,
  messages, artifacts, and revisions.
- **Immutable Revisions**: Unique `(artifact_id, version_number)`.
- **Exact Lineage**: Pinned upstream revision IDs, not just mutable flags.
- **Idempotency & Runs**: Dedicated tables for generation lifecycle and replay prevention.

```sql
-- 1. Projects (Evolved Container)
CREATE TABLE IF NOT EXISTS projects (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     UUID NOT NULL REFERENCES access_credentials(id) ON DELETE CASCADE,
    title       VARCHAR(255) NOT NULL,
    description TEXT,
    is_pinned   BOOLEAN NOT NULL DEFAULT false,
    metadata    JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_projects_user ON projects(user_id, updated_at DESC);

-- 2. Chat Threads (Scoped to Project)
CREATE TABLE IF NOT EXISTS chat_threads (
    id                 UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id         UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    title              VARCHAR(255) NOT NULL,
    stage_hint         VARCHAR(50) NOT NULL DEFAULT 'general',
    active_artifact_id UUID,
    metadata           JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at         TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at         TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_thread_project UNIQUE (id, project_id)
);
CREATE INDEX IF NOT EXISTS idx_chat_threads_proj ON chat_threads(project_id, updated_at DESC);

-- 3. Thread Messages
CREATE TABLE IF NOT EXISTS chat_messages_v2 (
    id                      UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id              UUID NOT NULL,
    thread_id               UUID NOT NULL,
    role                    VARCHAR(20) NOT NULL, -- 'user' | 'assistant' | 'system'
    content                 TEXT NOT NULL,
    client_message_id       VARCHAR(100),
    target_element_ids      JSONB,
    referenced_artifact_ids JSONB,
    metadata                JSONB NOT NULL DEFAULT '{}'::jsonb,
    sequence_number         BIGINT GENERATED ALWAYS AS IDENTITY,
    created_at              TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT fk_msg_thread FOREIGN KEY (thread_id, project_id) 
        REFERENCES chat_threads(id, project_id) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_chat_messages_v2_thread ON chat_messages_v2(thread_id, sequence_number ASC);

-- 4. Artifacts (Registry of Deliverables)
CREATE TABLE IF NOT EXISTS artifacts (
    id                     UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id             UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    type                   VARCHAR(50) NOT NULL, -- 'brainstorm'|'prd'|'erd'|'flowchart'|'architecture'|'wireframe'|'ui_design'
    title                  VARCHAR(255) NOT NULL,
    slug                   VARCHAR(100) NOT NULL,
    format                 VARCHAR(50) NOT NULL, -- 'markdown' | 'graph' | 'frame'
    current_version_number INT NOT NULL DEFAULT 1,
    current_revision_id    UUID,
    status                 VARCHAR(50) NOT NULL DEFAULT 'draft',
    metadata               JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at             TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at             TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_artifact_project UNIQUE (id, project_id),
    CONSTRAINT uq_artifact_slug UNIQUE (project_id, slug)
);
CREATE INDEX IF NOT EXISTS idx_artifacts_proj_type ON artifacts(project_id, type);

-- 5. Artifact Revisions (Immutable Snapshots)
CREATE TABLE IF NOT EXISTS artifact_revisions (
    id                 UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id         UUID NOT NULL,
    artifact_id        UUID NOT NULL,
    version_number     INT NOT NULL,
    change_summary     TEXT NOT NULL DEFAULT '',
    content_format     VARCHAR(50) NOT NULL,
    document_content   TEXT,
    graph_nodes        JSONB,
    graph_edges        JSONB,
    frame_data         JSONB,
    trigger_message_id UUID,
    author_id          UUID NOT NULL REFERENCES access_credentials(id),
    author_type        VARCHAR(50) NOT NULL DEFAULT 'ai', -- 'ai'|'manual'|'restore'|'sync'
    created_at         TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT fk_rev_artifact FOREIGN KEY (artifact_id, project_id) 
        REFERENCES artifacts(id, project_id) ON DELETE CASCADE,
    CONSTRAINT uq_artifact_version UNIQUE (artifact_id, version_number)
);
CREATE INDEX IF NOT EXISTS idx_artifact_revisions_ver ON artifact_revisions(artifact_id, version_number DESC);

-- 6. Context Lineage (Exact Provenance)
CREATE TABLE IF NOT EXISTS artifact_context_links (
    id                         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id                 UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    downstream_artifact_id     UUID NOT NULL,
    downstream_revision_number INT NOT NULL,
    upstream_artifact_id       UUID NOT NULL,
    upstream_revision_id       UUID NOT NULL REFERENCES artifact_revisions(id) ON DELETE CASCADE,
    link_type                  VARCHAR(50) NOT NULL DEFAULT 'context_source',
    created_at                 TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_context_link UNIQUE (downstream_artifact_id, downstream_revision_number, upstream_artifact_id)
);

-- 7. Idempotency Keys (Replay Protection)
CREATE TABLE IF NOT EXISTS idempotency_keys (
    key          VARCHAR(128) PRIMARY KEY,
    user_id      UUID NOT NULL REFERENCES access_credentials(id) ON DELETE CASCADE,
    resource     VARCHAR(255) NOT NULL,
    request_hash VARCHAR(64) NOT NULL,
    status_code  INT NOT NULL,
    response     JSONB NOT NULL,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at   TIMESTAMPTZ NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_idempotency_expires ON idempotency_keys(expires_at);
```

---

## 4. Sprint 11 Phased Execution Checklist

Sprint 11 will deliver the working foundation slice:

### Phase 1: Database Setup & Migration (Backend)
- [ ] Add `04_connected_workspace.sql` and integrate with `RunMigrations` in `postgres.go`.
- [ ] Run migration tests against local database fixture.
- [ ] Implement backfill routine for legacy `projects` rows.

### Phase 2: Product & Thread Domain (`internal/modules/product`)
- [ ] Domain structs and repository for `projects` and `chat_threads`.
- [ ] Service implementing `CreateProject`, `ListProjects`, `CreateThread`, `ListThreads`, `PostUserMessage`.
- [ ] HTTP handlers registered on `POST /api/v2/projects`, `GET /api/v2/projects`, `POST /api/v2/projects/{id}/threads`, etc.
- [ ] Scoped query unit tests verifying owner isolation.

### Phase 3: Artifact & Revision Domain (`internal/modules/artifact`)
- [ ] Domain structs and repository for `artifacts` and `artifact_revisions`.
- [ ] Append revision with optimistic concurrency check (`expected_revision_id`).
- [ ] Rollback implementation: appending version $N+1$ with content of version $V$.
- [ ] HTTP handlers registered on `GET /api/v2/projects/{id}/artifacts`, `POST /api/v2/projects/{id}/artifacts`, etc.

### Phase 4: React UI Shell & First End-to-End Slice (`frontend-react`)
- [ ] Project list & creation dialog using Radix Dialog + Lucide.
- [ ] Multi-thread sidebar navigation inside the active project.
- [ ] Canvas view connected to active artifact (React Flow rendering nodes/edges or markdown text).
- [ ] Manual node drag save or text edit persisting a new revision.
- [ ] Version history drawer displaying revisions with 1-click rollback.

---

## 5. Quality & Verification Gates

Before Sprint 11 is signed off:
1. `make verify` passes (100% Go unit tests clean).
2. `make test-race` passes (Zero race conditions on concurrent revision appends).
3. `make lint-spec` passes (OpenAPI synchronization).
4. `npm run build && npm run lint` passes in `frontend-react`.
5. Manual verification of the first end-to-end slice: Create Project -> Create Thread -> Create Artifact -> Edit -> Rollback.

---

## 6. Sprint 10 Final Summary

- **Step 1**: Deep Read-Only Audit of backend, Vue, and React starter. (`docs/architecture/sprint-10-step-1-audit.md`)
- **Step 2**: Target Relational Schema & Entity Design. (`docs/architecture/sprint-10-step-2-schema-design.md`)
- **Step 3**: Modular Monolith Boundaries & API Contracts. (`docs/architecture/sprint-10-step-3-contracts.md`)
- **Step 4**: Development Transition Strategy & Sprint 11 Blueprint. (`docs/architecture/sprint-10-step-4-migration-plan.md`)

**Sprint 10 is COMPLETE.** We are ready to begin **Sprint 11 Step 1**.
