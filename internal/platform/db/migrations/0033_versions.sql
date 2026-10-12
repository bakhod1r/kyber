-- S48: project versions (Jira's fix versions / releases) and the issues planned into them.
CREATE TABLE versions (
    id           uuid PRIMARY KEY,
    workspace_id uuid NOT NULL,
    project_key  text NOT NULL,
    name         text NOT NULL,
    description  text NOT NULL DEFAULT '',
    release_date timestamptz,
    state        text NOT NULL CHECK (state IN ('unreleased', 'released', 'archived')),
    released_at  timestamptz,
    version      int NOT NULL,
    UNIQUE (workspace_id, project_key, name),
    FOREIGN KEY (workspace_id, project_key) REFERENCES projects (workspace_id, key) ON DELETE CASCADE
);

CREATE TABLE issue_fix_versions (
    issue_id   uuid NOT NULL REFERENCES issues (id) ON DELETE CASCADE,
    version_id uuid NOT NULL REFERENCES versions (id) ON DELETE CASCADE,
    PRIMARY KEY (issue_id, version_id)
);
CREATE INDEX issue_fix_versions_version ON issue_fix_versions (version_id);
