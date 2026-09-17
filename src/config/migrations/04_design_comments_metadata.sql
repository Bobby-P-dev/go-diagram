-- Migration 04: Add metadata column to design_comments for targeted element references
ALTER TABLE design_comments ADD COLUMN IF NOT EXISTS metadata JSONB DEFAULT '{}'::jsonb;
