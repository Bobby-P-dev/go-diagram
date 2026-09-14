CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

CREATE TABLE IF NOT EXISTS diagrams (
    id          UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    prompt      TEXT        NOT NULL,
    diagram_type VARCHAR(50) NOT NULL,
    nodes       JSONB       NOT NULL DEFAULT '[]'::jsonb,
    edges       JSONB       NOT NULL DEFAULT '[]'::jsonb,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_diagrams_created_at ON diagrams (created_at DESC);
