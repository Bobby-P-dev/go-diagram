CREATE TEMP TABLE workspace_import_candidates ON COMMIT DROP AS
SELECT p.*,
    uuid_generate_v5(uuid_ns_url(), 'workspace-thread:' || p.id::text) AS imported_thread_id,
    uuid_generate_v5(uuid_ns_url(), 'workspace-artifact:' || p.id::text) AS imported_artifact_id,
    uuid_generate_v5(uuid_ns_url(), 'workspace-revision:' || p.id::text) AS imported_revision_id
FROM projects p
WHERE p.user_id IS NOT NULL AND p.project_mode IN ('diagram', 'ui_design')
    AND NOT EXISTS (SELECT 1 FROM legacy_workspace_imports imported WHERE imported.project_id = p.id);

INSERT INTO chat_threads(id, project_id, title, stage_hint, created_at, updated_at)
SELECT imported_thread_id, id, 'General Discussion',
    CASE WHEN project_mode = 'ui_design' THEN 'ui_design' ELSE 'diagram' END,
    created_at, updated_at
FROM workspace_import_candidates;

INSERT INTO chat_messages_v2(id, project_id, thread_id, role, content, target_element_ids, metadata, created_at)
SELECT message.id, message.project_id, project.imported_thread_id, message.role, message.content,
    CASE WHEN jsonb_typeof(message.target_node_ids) = 'array'
        THEN message.target_node_ids ELSE '[]'::jsonb END,
    COALESCE(message.metadata, '{}'::jsonb) ||
        CASE WHEN jsonb_typeof(message.target_node_ids) NOT IN ('array', 'null')
            THEN jsonb_build_object('legacy_target_node_ids', message.target_node_ids)
            ELSE '{}'::jsonb END,
    message.created_at
FROM chat_messages message JOIN workspace_import_candidates project ON project.id = message.project_id
ORDER BY message.created_at, message.id;

INSERT INTO artifacts(id, project_id, type, title, slug, format,
    current_version_number, current_revision_id, metadata, created_at, updated_at)
SELECT imported_artifact_id, project.id,
    CASE WHEN project_mode = 'ui_design' THEN 'ui_design'
        WHEN btrim(diagram_type) <> '' AND diagram_type NOT IN ('prd', 'brainstorm', 'wireframe', 'ui_design') THEN diagram_type
        ELSE 'diagram' END,
    CASE WHEN btrim(title) = '' THEN 'Imported artifact' ELSE title END, 'legacy-primary',
    CASE WHEN project_mode = 'ui_design' THEN 'frame' ELSE 'graph' END,
    (SELECT count(*) + 1 FROM diagram_versions version WHERE version.project_id = project.id),
    imported_revision_id, jsonb_build_object('legacy_project_mode', project_mode, 'legacy_diagram_type', diagram_type),
    project.created_at, project.updated_at
FROM workspace_import_candidates project;

INSERT INTO artifact_revisions(id, project_id, artifact_id, version_number, change_summary, content_format,
    graph_nodes, graph_edges, frame_data, trigger_message_id, author_id, author_type, metadata, created_at)
SELECT version.id, project.id, project.imported_artifact_id,
    row_number() OVER (PARTITION BY project.id ORDER BY version.version_number, version.created_at, version.id),
    COALESCE(version.change_summary, ''),
    CASE WHEN project.project_mode = 'ui_design' THEN 'frame' ELSE 'graph' END,
    CASE WHEN project.project_mode <> 'ui_design' THEN version.nodes END,
    CASE WHEN project.project_mode <> 'ui_design' THEN version.edges END,
    CASE WHEN project.project_mode = 'ui_design' THEN jsonb_build_object('nodes', version.nodes, 'edges', version.edges) END,
    message.id, project.user_id, 'migration',
    jsonb_build_object('legacy_version_id', version.id, 'legacy_version_number', version.version_number,
        'legacy_trigger_message_id', version.trigger_message_id), version.created_at
FROM diagram_versions version
JOIN workspace_import_candidates project ON project.id = version.project_id
LEFT JOIN chat_messages_v2 message ON message.id = version.trigger_message_id AND message.project_id = project.id;

INSERT INTO artifact_revisions(id, project_id, artifact_id, version_number, change_summary, content_format,
    graph_nodes, graph_edges, frame_data, author_id, author_type, metadata, created_at)
SELECT imported_revision_id, project.id, imported_artifact_id,
    (SELECT count(*) + 1 FROM diagram_versions version WHERE version.project_id = project.id),
    'Imported current legacy canvas',
    CASE WHEN project_mode = 'ui_design' THEN 'frame' ELSE 'graph' END,
    CASE WHEN project_mode <> 'ui_design' THEN current_nodes END,
    CASE WHEN project_mode <> 'ui_design' THEN current_edges END,
    CASE WHEN project_mode = 'ui_design' THEN jsonb_build_object('nodes', current_nodes, 'edges', current_edges) END,
    user_id, 'migration', jsonb_build_object('legacy_current_snapshot', true), updated_at
FROM workspace_import_candidates project;

UPDATE chat_threads thread SET active_artifact_id = project.imported_artifact_id
FROM workspace_import_candidates project WHERE thread.id = project.imported_thread_id;

INSERT INTO legacy_workspace_imports(project_id, thread_id, artifact_id)
SELECT id, imported_thread_id, imported_artifact_id FROM workspace_import_candidates;
