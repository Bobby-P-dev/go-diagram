-- Consolidated Database Initialization Script
-- Automatically executed on first boot by postgres container via /docker-entrypoint-initdb.d/init.sql

CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- 1. Diagrams Table
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

-- 2. Projects Table
CREATE TABLE IF NOT EXISTS projects (
    id            UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    title         VARCHAR(255) NOT NULL,
    diagram_type  VARCHAR(50) NOT NULL,
    project_mode  VARCHAR(50) NOT NULL DEFAULT 'diagram',
    is_pinned     BOOLEAN NOT NULL DEFAULT false,
    metadata      JSONB DEFAULT '{}'::jsonb,
    current_nodes JSONB NOT NULL DEFAULT '[]'::jsonb,
    current_edges JSONB NOT NULL DEFAULT '[]'::jsonb,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_projects_updated_at ON projects (updated_at DESC);
CREATE INDEX IF NOT EXISTS idx_projects_pinned_updated ON projects (is_pinned DESC, updated_at DESC);

-- 3. Chat Messages Table
CREATE TABLE IF NOT EXISTS chat_messages (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    project_id      UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    role            VARCHAR(50) NOT NULL,
    content         TEXT NOT NULL,
    target_node_ids JSONB,
    metadata        JSONB DEFAULT '{}'::jsonb,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_chat_messages_project_id ON chat_messages (project_id);
CREATE INDEX IF NOT EXISTS idx_chat_messages_created_at ON chat_messages (created_at ASC);

-- 4. Diagram Versions (Snapshots) Table
CREATE TABLE IF NOT EXISTS diagram_versions (
    id                 UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    project_id         UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    version_number     INT NOT NULL,
    change_summary     TEXT,
    nodes              JSONB NOT NULL DEFAULT '[]'::jsonb,
    edges              JSONB NOT NULL DEFAULT '[]'::jsonb,
    trigger_message_id UUID,
    created_at         TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_diagram_versions_project_id ON diagram_versions (project_id, version_number DESC);

-- 5. AI Usage Logs Table
CREATE TABLE IF NOT EXISTS ai_usage_logs (
    id                UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    project_id        UUID,
    provider          VARCHAR(50) NOT NULL,
    model             VARCHAR(100) NOT NULL,
    prompt_tokens     INT DEFAULT 0,
    completion_tokens INT DEFAULT 0,
    total_tokens      INT DEFAULT 0,
    latency_ms        INT DEFAULT 0,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- 6. Diagram Templates Table
CREATE TABLE IF NOT EXISTS diagram_templates (
    id           UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name         VARCHAR(255) NOT NULL,
    category     VARCHAR(100) NOT NULL,
    description  TEXT,
    diagram_type VARCHAR(100) NOT NULL,
    nodes        JSONB NOT NULL DEFAULT '[]'::jsonb,
    edges        JSONB NOT NULL DEFAULT '[]'::jsonb,
    is_featured  BOOLEAN NOT NULL DEFAULT false,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- 7. UI Templates Table
CREATE TABLE IF NOT EXISTS ui_templates (
    id          UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name        VARCHAR(255) NOT NULL,
    category    VARCHAR(100) NOT NULL,
    device      VARCHAR(50) NOT NULL DEFAULT 'web',
    description TEXT,
    theme       JSONB NOT NULL DEFAULT '{}'::jsonb,
    sections    JSONB NOT NULL DEFAULT '[]'::jsonb,
    code_export JSONB NOT NULL DEFAULT '{}'::jsonb,
    is_featured BOOLEAN NOT NULL DEFAULT false,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- 8. Design Comments Table
CREATE TABLE IF NOT EXISTS design_comments (
    id          UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    project_id  UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    node_id     VARCHAR(100),
    author      VARCHAR(100) NOT NULL DEFAULT 'Designer',
    content     TEXT NOT NULL,
    status      VARCHAR(50) NOT NULL DEFAULT 'open',
    position_x  DOUBLE PRECISION DEFAULT 0,
    position_y  DOUBLE PRECISION DEFAULT 0,
    metadata    JSONB DEFAULT '{}'::jsonb,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_design_comments_project_id ON design_comments (project_id);

-- 9. Design Foundations Table
CREATE TABLE IF NOT EXISTS design_foundations (
    id          UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name        VARCHAR(150) NOT NULL,
    category    VARCHAR(50) NOT NULL DEFAULT 'system',
    description TEXT,
    tokens      JSONB NOT NULL DEFAULT '{}'::jsonb,
    is_default  BOOLEAN NOT NULL DEFAULT false,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_design_foundations_category ON design_foundations (category);

-- 10. Project Exports Table
CREATE TABLE IF NOT EXISTS project_exports (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    project_id      UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    export_type     VARCHAR(50) NOT NULL,
    file_name       VARCHAR(255) NOT NULL,
    content         TEXT NOT NULL,
    file_size_bytes INTEGER DEFAULT 0,
    metadata        JSONB DEFAULT '{}'::jsonb,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_project_exports_project_id ON project_exports (project_id);

-- 11. Workspace Settings Table
CREATE TABLE IF NOT EXISTS workspace_settings (
    id                 VARCHAR(50) PRIMARY KEY DEFAULT 'default',
    default_mode       VARCHAR(50) DEFAULT 'diagram',
    theme              VARCHAR(50) DEFAULT 'dark',
    preferred_ai_model VARCHAR(100) DEFAULT 'cx/gpt-5.6-sol',
    canvas_preferences JSONB DEFAULT '{"snap_to_grid": true, "grid_size": 20, "show_minimap": true}'::jsonb,
    updated_at         TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Initial Seeds for workspace_settings
INSERT INTO workspace_settings (id, default_mode, theme, preferred_ai_model, canvas_preferences)
VALUES ('default', 'ui_design', 'dark', 'cx/gpt-5.6-sol', '{"snap_to_grid": true, "grid_size": 20, "show_minimap": true}'::jsonb)
ON CONFLICT (id) DO NOTHING;

-- Initial Seeds for design_foundations
INSERT INTO design_foundations (name, category, description, tokens, is_default)
VALUES 
(
    'Ramp Clean',
    'system',
    'Fintech clean design with emerald accents, high contrast slate typography, and crisp border cards.',
    '{"primary": "#10b981", "mode": "dark", "radius": "12px", "border": "#334155", "font": "Inter"}'::jsonb,
    true
),
(
    'Raycast Keyboard',
    'system',
    'Command-palette dark aesthetic with ultra-sharp borders, mono accents, and deep charcoal background.',
    '{"primary": "#f43f5e", "mode": "dark", "radius": "8px", "border": "#27272a", "font": "Inter"}'::jsonb,
    false
),
(
    'Vercel Minimal',
    'system',
    'High contrast monochrome aesthetic, pure white on void black with minimal shadows and clean lines.',
    '{"primary": "#000000", "mode": "dark", "radius": "6px", "border": "#262626", "font": "Geist"}'::jsonb,
    false
),
(
    'Linear Dark',
    'system',
    'Productivity tool aesthetic with soft indigo gradients, subtle glassmorphism and compact data grids.',
    '{"primary": "#6366f1", "mode": "dark", "radius": "10px", "border": "#312e81", "font": "Inter"}'::jsonb,
    false
)
ON CONFLICT DO NOTHING;

-- 12. Project Shares Table
CREATE TABLE IF NOT EXISTS project_shares (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    share_token     VARCHAR(64) UNIQUE NOT NULL,
    project_id      UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    title           VARCHAR(255) NOT NULL,
    is_active       BOOLEAN NOT NULL DEFAULT true,
    view_count      INT NOT NULL DEFAULT 0,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_project_shares_token ON project_shares (share_token);
CREATE INDEX IF NOT EXISTS idx_project_shares_project_id ON project_shares (project_id);

-- 13. Access Credentials Table (User & Admin Token-based Authentication)
CREATE TABLE IF NOT EXISTS access_credentials (
    id                   UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    credential_key       VARCHAR(64) UNIQUE NOT NULL,
    name                 VARCHAR(150) NOT NULL,
    role                 VARCHAR(50) NOT NULL DEFAULT 'user',
    is_active            BOOLEAN NOT NULL DEFAULT true,
    can_generate_diagram BOOLEAN NOT NULL DEFAULT true,
    can_generate_ui      BOOLEAN NOT NULL DEFAULT true,
    expires_at           TIMESTAMPTZ,
    last_login_at        TIMESTAMPTZ,
    created_at           TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at           TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_access_credentials_key ON access_credentials (credential_key);

-- Idempotent column migrations for existing instances
ALTER TABLE access_credentials ADD COLUMN IF NOT EXISTS can_generate_diagram BOOLEAN NOT NULL DEFAULT true;
ALTER TABLE access_credentials ADD COLUMN IF NOT EXISTS can_generate_ui BOOLEAN NOT NULL DEFAULT true;

-- Add user_id column to projects table if not exists
ALTER TABLE projects ADD COLUMN IF NOT EXISTS user_id UUID REFERENCES access_credentials(id) ON DELETE CASCADE;
CREATE INDEX IF NOT EXISTS idx_projects_user_id ON projects (user_id);

-- Seed Master Administrator account
INSERT INTO access_credentials (id, credential_key, name, role, is_active, expires_at, can_generate_diagram, can_generate_ui)
VALUES (
    'a0000000-0000-0000-0000-000000000001',
    '@bbaystr772',
    'Administrator Utama',
    'admin',
    true,
    NULL,
    true,
    true
)
ON CONFLICT (id) DO UPDATE SET credential_key = '@bbaystr772', role = 'admin', is_active = true, can_generate_diagram = true, can_generate_ui = true;

-- Associate all existing orphan projects with the Master Admin account
UPDATE projects
SET user_id = 'a0000000-0000-0000-0000-000000000001'
WHERE user_id IS NULL;

