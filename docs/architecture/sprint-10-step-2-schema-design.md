# Sprint 10 Step 2: Target Relational Schema & Entity Architecture

**Status**: Target Design Document (Read-Only / Planning Phase)  
**Date**: 2026-10-28  
**Scope**: PostgreSQL Schema for the Connected Multi-Artifact Product Workspace (`var-www-project-diagram`).

> Design review note (Step 3): the DDL below is illustrative, not an executable
> migration. See `sprint-10-step-3-contracts.md`, especially section 9, for required
> same-project constraints, revision/approval safety, exact context provenance,
> and concurrency corrections before implementation. The user confirms this is
> development and allows a clean redesign; the additive compatibility proposal
> in section 4 is an option, not an approved or mandatory migration procedure.

---

## 1. Architectural Philosophy

The target schema moves the backend from a single-canvas tool to an **End-to-End Product Engineering Workspace**.
It is governed by five strict principles:

1. **Project as Security & Context Boundary**: Everything belongs to a `project_id` and `user_id`. No query or AI prompt may cross this boundary.
2. **Chat Threads vs Artifacts**:
   - *Chats* are transient or ongoing conversations where requirements are discussed and prompts are submitted.
   - *Artifacts* are durable, structured deliverables (PRD document, ERD graph, Flowchart graph, Wireframe frame, UI Design frame).
   - One chat can produce or iterate multiple artifacts; one artifact can be refined across multiple chats.
3. **Immutable Revisions**: Revising an artifact never overwrites history. Every modification creates an immutable `artifact_revisions` record (`v1` → `v2` → `v3`), enabling safe experimentation and 1-click rollback.
4. **Context Dependency Graph (`artifact_context_links`)**: Downstream artifacts (e.g. UI Design) explicitly link to upstream artifacts and versions (e.g. PRD v2 + ERD v3). When an upstream artifact increments its version, downstream links flag an opt-in review notice (`is_stale = true`).
5. **Explicit Decisions vs Speculative Chat**: `product_decisions` stores approved business rules and constraints separately from casual chat speculation.

---

## 2. Entity-Relationship Diagram (Target Domain Model)

```text
 ┌───────────────────────┐
 │   access_credentials  │
 └───────────┬───────────┘
             │ 1
             │
             │ N
 ┌───────────▼───────────┐       1:N       ┌────────────────────────┐
 │       projects        ├─────────────────►│   product_decisions    │
 └─────┬───────────┬─────┘                 └────────────────────────┘
       │ 1         │ 1
       │           │
       │ N         │ N
┌──────▼─────┐  ┌──▼───────────────┐  1:N  ┌────────────────────────┐
│chat_threads│  │    artifacts     ├───────►│   artifact_revisions   │
└──────┬─────┘  └──┬───────────────┘       └───────────▲────────────┘
       │ 1         │ 1                                 │
       │           │                                   │ Upstream & Downstream
       │ N         │ N                                 │ Revisions
┌──────▼──────┐ ┌──▼────────────────────────┐          │
│chat_messages│ │   artifact_context_links  ├──────────┘
└─────────────┘ └───────────────────────────┘
```

---

## 3. Detailed Table Specifications (PostgreSQL DDL)

### 3.1 `projects` (Workspace Container)
Represents a distinct product initiative.
```sql
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

CREATE INDEX IF NOT EXISTS idx_projects_user_id ON projects(user_id);
CREATE INDEX IF NOT EXISTS idx_projects_pinned_updated ON projects(user_id, is_pinned DESC, updated_at DESC);
```

---

### 3.2 `chat_threads` (Conversational Threads)
Allows multiple concurrent or stage-focused conversations within a project.
```sql
CREATE TABLE IF NOT EXISTS chat_threads (
    id                 UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id         UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    user_id            UUID NOT NULL REFERENCES access_credentials(id) ON DELETE CASCADE,
    title              VARCHAR(255) NOT NULL,
    stage              VARCHAR(50) NOT NULL DEFAULT 'general', 
    -- 'general' | 'brainstorm' | 'prd' | 'diagram' | 'wireframe' | 'ui_design'
    active_artifact_id UUID, -- Optional pointer to an artifact currently in focus
    metadata           JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at         TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at         TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_chat_threads_project ON chat_threads(project_id, updated_at DESC);
CREATE INDEX IF NOT EXISTS idx_chat_threads_user ON chat_threads(user_id);
```

---

### 3.3 `chat_messages` (Threaded Messages)
Messages scoped to a specific thread, tracking targeted element IDs and cited artifacts.
```sql
CREATE TABLE IF NOT EXISTS chat_messages (
    id                      UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    thread_id               UUID NOT NULL REFERENCES chat_threads(id) ON DELETE CASCADE,
    project_id              UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    role                    VARCHAR(50) NOT NULL, -- 'user' | 'assistant' | 'system'
    content                 TEXT NOT NULL,
    target_node_ids         JSONB, -- Array of targeted node/section IDs for surgical revisions
    referenced_artifact_ids JSONB, -- Array of artifact IDs explicitly loaded into prompt context
    metadata                JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at              TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_chat_messages_thread ON chat_messages(thread_id, created_at ASC);
CREATE INDEX IF NOT EXISTS idx_chat_messages_project ON chat_messages(project_id);
```

---

### 3.4 `artifacts` (Durable Deliverables)
Core registry of deliverables within a project.
```sql
CREATE TABLE IF NOT EXISTS artifacts (
    id                     UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id             UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    user_id                UUID NOT NULL REFERENCES access_credentials(id) ON DELETE CASCADE,
    type                   VARCHAR(50) NOT NULL, 
    -- 'brainstorm' | 'prd' | 'erd' | 'flowchart' | 'architecture' | 'wireframe' | 'ui_design'
    title                  VARCHAR(255) NOT NULL,
    slug                   VARCHAR(100) NOT NULL,
    format                 VARCHAR(50) NOT NULL, 
    -- 'markdown' (for text/PRD) | 'graph' (for nodes/edges) | 'frame' (for UI Frame)
    current_version_number INT NOT NULL DEFAULT 1,
    status                 VARCHAR(50) NOT NULL DEFAULT 'draft', -- 'draft' | 'approved' | 'deprecated'
    metadata               JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at             TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at             TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_artifacts_project_slug UNIQUE (project_id, slug)
);

CREATE INDEX IF NOT EXISTS idx_artifacts_project ON artifacts(project_id, type);
CREATE INDEX IF NOT EXISTS idx_artifacts_user ON artifacts(user_id);
```

---

### 3.5 `artifact_revisions` (Immutable Version Snapshots)
Complete state snapshots for each revision of an artifact.
```sql
CREATE TABLE IF NOT EXISTS artifact_revisions (
    id                 UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    artifact_id        UUID NOT NULL REFERENCES artifacts(id) ON DELETE CASCADE,
    project_id         UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    version_number     INT NOT NULL,
    change_summary     TEXT NOT NULL DEFAULT '',
    content_format     VARCHAR(50) NOT NULL, -- 'markdown' | 'graph' | 'frame'
    document_content   TEXT,                 -- Populated when format = 'markdown'
    graph_nodes        JSONB,                -- Populated when format in ('graph', 'frame')
    graph_edges        JSONB,                -- Populated when format in ('graph')
    frame_data         JSONB,                -- Populated when format = 'frame' (UI frame tokens, raw_html, TSX)
    trigger_message_id UUID REFERENCES chat_messages(id) ON DELETE SET NULL,
    author_type        VARCHAR(50) NOT NULL DEFAULT 'ai', -- 'ai' | 'user_manual' | 'sync_update'
    created_at         TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_artifact_revisions_version UNIQUE (artifact_id, version_number)
);

CREATE INDEX IF NOT EXISTS idx_artifact_revisions_lookup ON artifact_revisions(artifact_id, version_number DESC);
CREATE INDEX IF NOT EXISTS idx_artifact_revisions_project ON artifact_revisions(project_id);
```

---

### 3.6 `artifact_context_links` (Lineage & Sync Engine)
Tracks the exact upstream dependencies of any generated or revised artifact.
```sql
CREATE TABLE IF NOT EXISTS artifact_context_links (
    id                         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id                 UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    downstream_artifact_id     UUID NOT NULL REFERENCES artifacts(id) ON DELETE CASCADE,
    downstream_revision_number INT NOT NULL,
    upstream_artifact_id       UUID NOT NULL REFERENCES artifacts(id) ON DELETE CASCADE,
    upstream_revision_number   INT NOT NULL,
    link_type                  VARCHAR(50) NOT NULL DEFAULT 'context_source', 
    -- 'context_source' | 'derived_from' | 'sync_candidate'
    is_stale                   BOOLEAN NOT NULL DEFAULT false, -- Set true if upstream receives higher revision
    created_at                 TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_context_links UNIQUE (downstream_artifact_id, downstream_revision_number, upstream_artifact_id)
);

CREATE INDEX IF NOT EXISTS idx_context_links_downstream ON artifact_context_links(downstream_artifact_id, downstream_revision_number);
CREATE INDEX IF NOT EXISTS idx_context_links_upstream ON artifact_context_links(upstream_artifact_id);
CREATE INDEX IF NOT EXISTS idx_context_links_stale ON artifact_context_links(project_id, is_stale);
```

---

### 3.7 `product_decisions` (Approved Decision Registry)
Explicit, high-signal business rules and architecture decisions extracted from discussions.
```sql
CREATE TABLE IF NOT EXISTS product_decisions (
    id                 UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id         UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    title              VARCHAR(255) NOT NULL,
    category           VARCHAR(50) NOT NULL, -- 'feature' | 'business_rule' | 'constraint' | 'out_of_scope'
    status             VARCHAR(50) NOT NULL DEFAULT 'approved', -- 'proposed' | 'approved' | 'superseded'
    rationale          TEXT,
    source_artifact_id UUID REFERENCES artifacts(id) ON DELETE SET NULL,
    source_message_id  UUID REFERENCES chat_messages(id) ON DELETE SET NULL,
    metadata           JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at         TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at         TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_product_decisions_project ON product_decisions(project_id, status);
```

---

## 4. Backwards Compatibility & Non-Destructive Migration Plan

Existing production databases hold data in `projects`, `chat_messages`, and `diagram_versions`.
During execution (Sprint 11):
1. New tables will be created cleanly (`CREATE TABLE IF NOT EXISTS`).
2. An idempotent migration script will backfill each existing project:
   - Create 1 default `chat_threads` row titled `"General Discussion"`.
   - Update existing `chat_messages` rows to point `thread_id` to that default thread.
   - Create 1 default `artifacts` row (type `'flowchart'` or `'ui_design'` matching `project_mode`) containing the current `current_nodes` and `current_edges`.
   - Migrate historical `diagram_versions` into `artifact_revisions`.
3. Legacy columns on `projects` will be preserved so existing endpoints (`GET /api/projects/{id}`) continue operating normally until the frontend fully switches to the new endpoints.

---

## 5. Step 2 Sign-Off & Progression to Step 3

The database schema design is complete and documented.  
**Ready for Step 3**: Design the Module Boundaries, Go Interfaces, and API Endpoints for `Workspace`, `Threads`, and `Artifacts`.
