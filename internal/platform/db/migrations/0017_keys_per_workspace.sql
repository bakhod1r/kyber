-- QA-1 / ADR-0004 phase 2: project keys are unique per workspace (like Jira sites), so every row
-- that is addressed by a project key carries its workspace. The outbox records the workspace too,
-- so event handlers run scoped to it.
ALTER TABLE issues DROP CONSTRAINT issues_project_key_fkey;
ALTER TABLE sprints DROP CONSTRAINT sprints_project_key_fkey;
ALTER TABLE projects DROP CONSTRAINT projects_key_key;
ALTER TABLE projects ADD CONSTRAINT projects_workspace_key UNIQUE (workspace_id, key);

ALTER TABLE issues ADD COLUMN workspace_id uuid;
UPDATE issues i SET workspace_id = p.workspace_id FROM projects p WHERE p.key = i.project_key;
ALTER TABLE issues ALTER COLUMN workspace_id SET NOT NULL;
ALTER TABLE issues DROP CONSTRAINT issues_project_key_number_key;
ALTER TABLE issues ADD CONSTRAINT issues_workspace_key UNIQUE (workspace_id, project_key, number);
ALTER TABLE issues ADD CONSTRAINT issues_project_fkey FOREIGN KEY (workspace_id, project_key) REFERENCES projects (workspace_id, key);

ALTER TABLE sprints ADD COLUMN workspace_id uuid;
UPDATE sprints s SET workspace_id = p.workspace_id FROM projects p WHERE p.key = s.project_key;
ALTER TABLE sprints ALTER COLUMN workspace_id SET NOT NULL;
ALTER TABLE sprints ADD CONSTRAINT sprints_project_fkey FOREIGN KEY (workspace_id, project_key) REFERENCES projects (workspace_id, key);
DROP INDEX sprints_one_active;
CREATE UNIQUE INDEX sprints_one_active ON sprints (workspace_id, project_key) WHERE state = 'active';

ALTER TABLE activity ADD COLUMN workspace_id uuid;
UPDATE activity a SET workspace_id = p.workspace_id FROM projects p WHERE p.key = a.project_key;
UPDATE activity SET workspace_id = '00000000-0000-4000-8000-000000000001' WHERE workspace_id IS NULL;
ALTER TABLE activity ALTER COLUMN workspace_id SET NOT NULL;
ALTER TABLE activity ALTER COLUMN workspace_id SET DEFAULT '00000000-0000-4000-8000-000000000001';
DROP INDEX activity_project;
CREATE INDEX activity_project ON activity (workspace_id, project_key, at, event);

ALTER TABLE import_mappings ADD COLUMN workspace_id uuid;
UPDATE import_mappings m SET workspace_id = p.workspace_id FROM projects p WHERE p.key = m.project_key;
UPDATE import_mappings SET workspace_id = '00000000-0000-4000-8000-000000000001' WHERE workspace_id IS NULL;
ALTER TABLE import_mappings ALTER COLUMN workspace_id SET NOT NULL;
ALTER TABLE import_mappings DROP CONSTRAINT import_mappings_pkey;
ALTER TABLE import_mappings ADD PRIMARY KEY (workspace_id, project_key, external_key);

ALTER TABLE outbox ADD COLUMN workspace_id uuid NOT NULL DEFAULT '00000000-0000-4000-8000-000000000001';
