-- Custom fields: per-project field definitions and the values issues hold (keyed by field id).
CREATE TABLE custom_fields (
    id           uuid PRIMARY KEY,
    workspace_id uuid NOT NULL,
    project_key  text NOT NULL,
    name         text NOT NULL,
    type         text NOT NULL CHECK (type IN ('text', 'number', 'date', 'select')),
    options      jsonb NOT NULL DEFAULT '[]',
    created_at   timestamptz NOT NULL DEFAULT clock_timestamp(),
    version      int NOT NULL,
    UNIQUE (workspace_id, project_key, name),
    FOREIGN KEY (workspace_id, project_key) REFERENCES projects (workspace_id, key) ON DELETE CASCADE
);

ALTER TABLE issues ADD COLUMN custom_fields jsonb NOT NULL DEFAULT '{}';
