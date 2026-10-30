ALTER TABLE generation_runs
    DROP CONSTRAINT generation_runs_output_type_check,
    ADD CONSTRAINT generation_runs_output_type_check
        CHECK (output_type IN ('brainstorm', 'prd', 'erd', 'flowchart', 'sequence')),
    ADD COLUMN base_graph JSONB,
    ADD CONSTRAINT generation_runs_base_content_check CHECK (
        (base_graph IS NULL OR jsonb_typeof(base_graph) = 'object')
        AND (output_type NOT IN ('brainstorm', 'prd') OR base_graph IS NULL)
        AND (output_type NOT IN ('erd', 'flowchart', 'sequence') OR base_document IS NULL)
        AND (target_artifact_id IS NOT NULL OR (base_graph IS NULL AND base_document IS NULL))
        AND (target_artifact_id IS NULL
            OR (output_type IN ('brainstorm', 'prd') AND base_document IS NOT NULL)
            OR (output_type IN ('erd', 'flowchart', 'sequence') AND base_graph IS NOT NULL))
    );
