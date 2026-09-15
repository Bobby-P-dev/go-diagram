-- Migration: 03_system_enhancements.sql
-- Menambahkan tabel dan kolom yang dibutuhkan untuk kolaborasi review, audit AI metadata, design foundations, export artifacts, dan workspace settings.

-- 1. Tambah metadata pada chat_messages untuk menyimpan Change Plan & compiler audit
ALTER TABLE chat_messages ADD COLUMN IF NOT EXISTS metadata JSONB DEFAULT '{}'::jsonb;

-- 2. Tabel design_comments untuk persistensi review & anotasi feedback di canvas
CREATE TABLE IF NOT EXISTS design_comments (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    node_id VARCHAR(100),
    author VARCHAR(100) NOT NULL DEFAULT 'Designer',
    content TEXT NOT NULL,
    status VARCHAR(50) NOT NULL DEFAULT 'open', -- 'open', 'resolved'
    position_x DOUBLE PRECISION DEFAULT 0,
    position_y DOUBLE PRECISION DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_design_comments_project_id ON design_comments (project_id);
CREATE INDEX IF NOT EXISTS idx_design_comments_node_id ON design_comments (node_id);

-- 3. Tabel design_foundations untuk menyimpan token design system terstandarisasi
CREATE TABLE IF NOT EXISTS design_foundations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(150) NOT NULL,
    category VARCHAR(50) NOT NULL DEFAULT 'system', -- 'system', 'custom'
    description TEXT,
    tokens JSONB NOT NULL DEFAULT '{}'::jsonb,
    is_default BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_design_foundations_category ON design_foundations (category);

-- 4. Tabel project_exports untuk riwayat build & export code (Vue, HTML, SQL DDL, Mermaid)
CREATE TABLE IF NOT EXISTS project_exports (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    export_type VARCHAR(50) NOT NULL, -- 'html_bundle', 'vue_sfc', 'mermaid', 'sql_ddl', 'json_spec'
    file_name VARCHAR(255) NOT NULL,
    content TEXT NOT NULL,
    file_size_bytes INTEGER DEFAULT 0,
    metadata JSONB DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_project_exports_project_id ON project_exports (project_id);

-- 5. Tabel workspace_settings untuk konfigurasi preferensi pengguna dan canvas
CREATE TABLE IF NOT EXISTS workspace_settings (
    id VARCHAR(50) PRIMARY KEY DEFAULT 'default',
    default_mode VARCHAR(50) DEFAULT 'diagram',
    theme VARCHAR(50) DEFAULT 'dark',
    preferred_ai_model VARCHAR(100) DEFAULT 'ag/gemini-3.8-flash-high',
    canvas_preferences JSONB DEFAULT '{"snap_to_grid": true, "grid_size": 20, "show_minimap": true}'::jsonb,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Seed awal untuk workspace_settings
INSERT INTO workspace_settings (id, default_mode, theme, preferred_ai_model, canvas_preferences)
VALUES ('default', 'ui_design', 'dark', 'ag/gemini-3.8-flash-high', '{"snap_to_grid": true, "grid_size": 20, "show_minimap": true}'::jsonb)
ON CONFLICT (id) DO NOTHING;

-- Seed fondasi desain bawaan (Anti-Slop Foundations)
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
