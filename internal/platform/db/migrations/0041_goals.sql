-- D4 Goals: outcomes a project commits to. The project and owner are referenced by key and id only
-- (other contexts; no cross-context foreign keys).
CREATE TABLE goals (
    id           uuid PRIMARY KEY,
    workspace_id uuid NOT NULL REFERENCES workspaces (id) ON DELETE CASCADE,
    project_key  text NOT NULL,
    owner_id     uuid NOT NULL,
    name         text NOT NULL,
    target_date  date,
    jql          text NOT NULL DEFAULT '',
    status       text NOT NULL,
    version      int NOT NULL
);
CREATE INDEX goals_project ON goals (workspace_id, project_key);
