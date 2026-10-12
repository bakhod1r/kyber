-- Sprint 10 (KYB-S44): Scrum and Kanban boards. Columns (name, mapped statuses, WIP min/max) are a value
-- list owned by the board aggregate, stored as JSON.
CREATE TABLE boards (
    id           uuid PRIMARY KEY,
    workspace_id uuid NOT NULL,
    project_key  text NOT NULL,
    name         text NOT NULL CHECK (btrim(name) <> ''),
    kind         text NOT NULL CHECK (kind IN ('scrum', 'kanban')),
    columns      jsonb NOT NULL,
    version      int NOT NULL,
    FOREIGN KEY (workspace_id, project_key) REFERENCES projects (workspace_id, key) ON DELETE CASCADE
);
CREATE INDEX boards_project ON boards (workspace_id, project_key, name);
