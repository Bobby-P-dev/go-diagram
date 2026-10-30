ALTER TABLE chat_messages_v2 ADD CONSTRAINT uq_messages_project_thread UNIQUE (id, project_id, thread_id);

CREATE TABLE generation_runs (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    project_id UUID NOT NULL,
    thread_id UUID NOT NULL,
    user_id UUID NOT NULL REFERENCES access_credentials(id) ON DELETE CASCADE,
    trigger_message_id UUID NOT NULL,
    output_type VARCHAR(50) NOT NULL CHECK (output_type IN ('brainstorm', 'prd')),
    output_title VARCHAR(255) NOT NULL DEFAULT '',
    target_artifact_id UUID,
    target_expected_revision_id UUID,
    context_mode VARCHAR(30) NOT NULL DEFAULT 'explicit' CHECK (context_mode IN ('explicit', 'project_relevant')),
    frozen_manifest JSONB NOT NULL CHECK (jsonb_typeof(frozen_manifest) = 'object'),
    prompt TEXT NOT NULL DEFAULT '',
    base_document TEXT,
    status VARCHAR(30) NOT NULL DEFAULT 'queued'
        CHECK (status IN ('queued', 'running', 'cancel_requested', 'succeeded', 'failed', 'canceled')),
    error_code VARCHAR(50),
    error_message TEXT,
    committed_artifact_id UUID,
    committed_revision_id UUID,
    committed_message_id UUID,
    idempotency_key VARCHAR(128),
    request_hash VARCHAR(64),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_at TIMESTAMPTZ,
    lease_expires_at TIMESTAMPTZ,
    UNIQUE (id, project_id),
    FOREIGN KEY (project_id) REFERENCES projects(id) ON DELETE CASCADE,
    FOREIGN KEY (thread_id, project_id) REFERENCES chat_threads(id, project_id) ON DELETE CASCADE,
    FOREIGN KEY (trigger_message_id, project_id, thread_id) REFERENCES chat_messages_v2(id, project_id, thread_id) ON DELETE CASCADE,
    FOREIGN KEY (target_artifact_id, project_id) REFERENCES artifacts(id, project_id) DEFERRABLE INITIALLY DEFERRED,
    FOREIGN KEY (target_artifact_id, project_id, target_expected_revision_id)
        REFERENCES artifact_revisions(artifact_id, project_id, id) DEFERRABLE INITIALLY DEFERRED,
    FOREIGN KEY (committed_artifact_id, project_id) REFERENCES artifacts(id, project_id) DEFERRABLE INITIALLY DEFERRED,
    FOREIGN KEY (committed_artifact_id, project_id, committed_revision_id)
        REFERENCES artifact_revisions(artifact_id, project_id, id) DEFERRABLE INITIALLY DEFERRED,
    FOREIGN KEY (committed_message_id, project_id, thread_id) REFERENCES chat_messages_v2(id, project_id, thread_id) DEFERRABLE INITIALLY DEFERRED,
    CHECK ((target_artifact_id IS NULL) = (target_expected_revision_id IS NULL)),
    CHECK ((status IN ('succeeded','failed','canceled')) = (completed_at IS NOT NULL)),
    CHECK (status <> 'succeeded' OR
        (committed_artifact_id IS NOT NULL AND committed_revision_id IS NOT NULL AND committed_message_id IS NOT NULL)),
    CHECK (idempotency_key IS NOT NULL AND btrim(idempotency_key) <> ''),
    CHECK (request_hash IS NOT NULL AND request_hash ~ '^[a-f0-9]{64}$')
);

CREATE UNIQUE INDEX idx_generation_runs_idempotency
    ON generation_runs(user_id, project_id, idempotency_key)
    WHERE idempotency_key IS NOT NULL;

CREATE INDEX idx_generation_runs_project_thread_created
    ON generation_runs(project_id, thread_id, created_at DESC);

CREATE INDEX idx_generation_runs_status
    ON generation_runs(status);
