-- ADR-0003 phase 2: a project's own permission scheme. A project without rows uses the default.
CREATE TABLE project_permission_grants (
    workspace_id uuid NOT NULL,
    project_key  text NOT NULL,
    permission   text NOT NULL,
    holder_kind  text NOT NULL,
    holder_value text NOT NULL DEFAULT '',
    PRIMARY KEY (workspace_id, project_key, permission, holder_kind, holder_value),
    FOREIGN KEY (workspace_id, project_key) REFERENCES projects (workspace_id, key) ON DELETE CASCADE
);
