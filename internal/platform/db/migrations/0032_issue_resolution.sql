-- S48: Jira's resolution field ("" = unresolved). Issues already in a Done-category status are
-- resolved as "done": the built-in workflow's "done", or a project workflow's Done statuses.
ALTER TABLE issues ADD COLUMN resolution text NOT NULL DEFAULT '';

UPDATE issues i SET resolution = 'done'
WHERE (i.status = 'done' AND NOT EXISTS (
          SELECT 1 FROM project_workflows w WHERE w.workspace_id = i.workspace_id AND w.project_key = i.project_key))
   OR EXISTS (
          SELECT 1 FROM project_workflows w, jsonb_array_elements(w.definition -> 'statuses') st
          WHERE w.workspace_id = i.workspace_id AND w.project_key = i.project_key
            AND st ->> 'id' = i.status AND st ->> 'category' = 'done');
