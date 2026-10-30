ALTER TABLE generation_runs
    ADD COLUMN mutation JSONB,
    ADD CONSTRAINT generation_runs_mutation_check CHECK (
        mutation IS NULL OR (
            output_type IN ('erd', 'flowchart', 'sequence')
            AND target_artifact_id IS NOT NULL
            AND target_expected_revision_id IS NOT NULL
            AND base_graph IS NOT NULL
            AND jsonb_typeof(mutation) = 'object'
            AND mutation ? 'node_ids' AND mutation ? 'edge_ids'
            AND jsonb_typeof(mutation->'node_ids') = 'array'
            AND jsonb_typeof(mutation->'edge_ids') = 'array'
            AND CASE WHEN jsonb_typeof(mutation->'node_ids') = 'array'
                          AND jsonb_typeof(mutation->'edge_ids') = 'array'
                THEN jsonb_array_length(mutation->'node_ids') + jsonb_array_length(mutation->'edge_ids') BETWEEN 1 AND 32
                ELSE false END
        )
    );
