ALTER TABLE projects ADD COLUMN IF NOT EXISTS description TEXT NOT NULL DEFAULT '';
ALTER TABLE projects ALTER COLUMN diagram_type SET DEFAULT 'none';

CREATE TABLE chat_threads (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    title VARCHAR(255) NOT NULL CHECK (btrim(title) <> ''),
    stage_hint VARCHAR(50) NOT NULL DEFAULT 'general',
    active_artifact_id UUID,
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb CHECK (jsonb_typeof(metadata) = 'object'),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (id, project_id)
);
CREATE INDEX idx_chat_threads_project_updated ON chat_threads(project_id, updated_at DESC, id);

CREATE TABLE chat_messages_v2 (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    project_id UUID NOT NULL,
    thread_id UUID NOT NULL,
    role VARCHAR(20) NOT NULL CHECK (role IN ('user', 'assistant', 'system', 'tool')),
    content TEXT NOT NULL,
    client_message_id VARCHAR(128),
    target_element_ids JSONB CHECK (jsonb_typeof(target_element_ids) = 'array'),
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb CHECK (jsonb_typeof(metadata) = 'object'),
    sequence_number BIGINT GENERATED ALWAYS AS IDENTITY,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (id, project_id),
    UNIQUE (thread_id, client_message_id),
    FOREIGN KEY (thread_id, project_id) REFERENCES chat_threads(id, project_id) ON DELETE CASCADE
);
CREATE INDEX idx_chat_messages_v2_thread_seq ON chat_messages_v2(thread_id, sequence_number);

CREATE TABLE artifacts (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    type VARCHAR(50) NOT NULL CHECK (btrim(type) <> ''),
    title VARCHAR(255) NOT NULL CHECK (btrim(title) <> ''),
    slug VARCHAR(100) NOT NULL CHECK (btrim(slug) <> ''),
    format VARCHAR(20) NOT NULL CHECK (format IN ('markdown', 'graph', 'frame')),
    current_version_number INT NOT NULL DEFAULT 1 CHECK (current_version_number > 0),
    current_revision_id UUID NOT NULL DEFAULT uuid_generate_v4(),
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb CHECK (jsonb_typeof(metadata) = 'object'),
    archived_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (id, project_id),
    UNIQUE (id, project_id, format),
    UNIQUE (project_id, slug),
    CHECK (
        (type IN ('brainstorm', 'prd') AND format = 'markdown') OR
        (type IN ('wireframe', 'ui_design') AND format = 'frame') OR
        (type NOT IN ('brainstorm', 'prd', 'wireframe', 'ui_design') AND format = 'graph')
    )
);
CREATE INDEX idx_artifacts_project_type ON artifacts(project_id, type, created_at DESC, id);

CREATE TABLE artifact_revisions (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    project_id UUID NOT NULL,
    artifact_id UUID NOT NULL,
    version_number INT NOT NULL CHECK (version_number > 0),
    change_summary TEXT NOT NULL DEFAULT '',
    content_format VARCHAR(20) NOT NULL,
    schema_version INT NOT NULL DEFAULT 1 CHECK (schema_version = 1),
    document_content TEXT,
    graph_nodes JSONB,
    graph_edges JSONB,
    frame_data JSONB,
    trigger_message_id UUID,
    author_id UUID REFERENCES access_credentials(id) ON DELETE SET NULL,
    author_type VARCHAR(20) NOT NULL CHECK (author_type IN ('ai', 'manual', 'restore', 'sync', 'migration')),
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb CHECK (jsonb_typeof(metadata) = 'object'),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (artifact_id, version_number),
    UNIQUE (artifact_id, project_id, id),
    UNIQUE (artifact_id, project_id, version_number),
    UNIQUE (artifact_id, project_id, id, version_number),
    FOREIGN KEY (artifact_id, project_id, content_format)
        REFERENCES artifacts(id, project_id, format) ON DELETE CASCADE,
    FOREIGN KEY (trigger_message_id, project_id)
        REFERENCES chat_messages_v2(id, project_id) DEFERRABLE INITIALLY DEFERRED,
    CHECK (
        (content_format = 'markdown' AND document_content IS NOT NULL
            AND graph_nodes IS NULL AND graph_edges IS NULL AND frame_data IS NULL) OR
        (content_format = 'graph' AND document_content IS NULL AND frame_data IS NULL
            AND graph_nodes IS NOT NULL AND jsonb_typeof(graph_nodes) = 'array'
            AND graph_edges IS NOT NULL AND jsonb_typeof(graph_edges) = 'array') OR
        (content_format = 'frame' AND document_content IS NULL AND graph_nodes IS NULL AND graph_edges IS NULL
            AND frame_data IS NOT NULL AND jsonb_typeof(frame_data) = 'object'
            AND frame_data ? 'nodes' AND jsonb_typeof(frame_data->'nodes') = 'array'
            AND frame_data ? 'edges' AND jsonb_typeof(frame_data->'edges') = 'array')
    )
);

ALTER TABLE artifacts ADD CONSTRAINT fk_artifact_current_revision
    FOREIGN KEY (id, project_id, current_revision_id, current_version_number)
    REFERENCES artifact_revisions(artifact_id, project_id, id, version_number)
    DEFERRABLE INITIALLY DEFERRED;
ALTER TABLE chat_threads ADD CONSTRAINT fk_thread_active_artifact
    FOREIGN KEY (active_artifact_id, project_id) REFERENCES artifacts(id, project_id)
    DEFERRABLE INITIALLY DEFERRED;

CREATE FUNCTION reject_revision_mutation() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF TG_OP = 'DELETE' AND NOT EXISTS (SELECT 1 FROM artifacts WHERE id = OLD.artifact_id) THEN
        RETURN OLD;
    END IF;
    RAISE EXCEPTION 'Artifact revisions are immutable; append a revision instead' USING ERRCODE = '23514';
END;
$$;
CREATE TRIGGER artifact_revisions_immutable BEFORE UPDATE OR DELETE ON artifact_revisions
    FOR EACH ROW EXECUTE FUNCTION reject_revision_mutation();

CREATE TABLE artifact_context_links (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    downstream_artifact_id UUID NOT NULL,
    downstream_revision_number INT NOT NULL,
    upstream_artifact_id UUID NOT NULL,
    upstream_revision_id UUID NOT NULL,
    link_type VARCHAR(30) NOT NULL DEFAULT 'context_source'
        CHECK (link_type IN ('context_source', 'derived_from')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (downstream_artifact_id, downstream_revision_number, upstream_artifact_id),
    FOREIGN KEY (downstream_artifact_id, project_id, downstream_revision_number)
        REFERENCES artifact_revisions(artifact_id, project_id, version_number) ON DELETE CASCADE,
    FOREIGN KEY (upstream_artifact_id, project_id, upstream_revision_id)
        REFERENCES artifact_revisions(artifact_id, project_id, id) DEFERRABLE INITIALLY DEFERRED
);
CREATE INDEX idx_context_links_upstream ON artifact_context_links(upstream_artifact_id, upstream_revision_id);

CREATE TABLE product_decisions (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    title VARCHAR(255) NOT NULL CHECK (btrim(title) <> ''),
    category VARCHAR(30) NOT NULL DEFAULT 'feature'
        CHECK (category IN ('feature', 'business_rule', 'constraint', 'out_of_scope')),
    status VARCHAR(20) NOT NULL DEFAULT 'proposed'
        CHECK (status IN ('proposed', 'approved', 'superseded', 'rejected')),
    rationale TEXT,
    source_artifact_id UUID,
    source_message_id UUID,
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb CHECK (jsonb_typeof(metadata) = 'object'),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (id, project_id),
    FOREIGN KEY (source_artifact_id, project_id) REFERENCES artifacts(id, project_id),
    FOREIGN KEY (source_message_id, project_id) REFERENCES chat_messages_v2(id, project_id)
);
CREATE INDEX idx_product_decisions_project ON product_decisions(project_id, status);

CREATE TABLE idempotency_keys (
    key VARCHAR(128) NOT NULL CHECK (btrim(key) <> ''),
    user_id UUID NOT NULL REFERENCES access_credentials(id) ON DELETE CASCADE,
    project_id UUID REFERENCES projects(id) ON DELETE CASCADE,
    resource VARCHAR(255) NOT NULL CHECK (btrim(resource) <> ''),
    request_hash VARCHAR(64) NOT NULL CHECK (request_hash ~ '^[a-f0-9]{64}$'),
    status_code INT CHECK (status_code BETWEEN 200 AND 599),
    response JSONB,
    state VARCHAR(20) NOT NULL DEFAULT 'processing' CHECK (state IN ('processing', 'completed')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at TIMESTAMPTZ NOT NULL CHECK (expires_at > created_at),
    PRIMARY KEY (user_id, resource, key),
    CHECK ((state = 'processing' AND status_code IS NULL AND response IS NULL) OR
        (state = 'completed' AND status_code IS NOT NULL AND response IS NOT NULL))
);
CREATE INDEX idx_idempotency_expires ON idempotency_keys(expires_at);

CREATE TABLE legacy_workspace_imports (
    project_id UUID PRIMARY KEY REFERENCES projects(id) ON DELETE CASCADE,
    thread_id UUID NOT NULL,
    artifact_id UUID NOT NULL,
    imported_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    FOREIGN KEY (thread_id, project_id) REFERENCES chat_threads(id, project_id),
    FOREIGN KEY (artifact_id, project_id) REFERENCES artifacts(id, project_id)
);
