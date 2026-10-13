-- D6 Forms: a project's intake forms. Questions are a JSON array of {id, label, required, long}.
-- The project is referenced by key only (another context; no cross-context foreign keys).
CREATE TABLE forms (
    id           uuid PRIMARY KEY,
    workspace_id uuid NOT NULL REFERENCES workspaces (id) ON DELETE CASCADE,
    project_key  text NOT NULL,
    name         text NOT NULL,
    intro        text NOT NULL DEFAULT '',
    issue_type   text NOT NULL,
    questions    jsonb NOT NULL DEFAULT '[]',
    version      int NOT NULL
);
CREATE INDEX forms_project ON forms (workspace_id, project_key);
