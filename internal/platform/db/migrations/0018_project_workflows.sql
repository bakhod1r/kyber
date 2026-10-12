-- Sprint 10 (KYB-S43): per-project configurable workflows (statuses with Jira categories + transitions).
-- Projects without a row use the built-in simplified workflow (To Do / In Progress / Done).
CREATE TABLE project_workflows (
    workspace_id uuid NOT NULL,
    project_key  text NOT NULL,
    definition   jsonb NOT NULL,
    updated_at   timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (workspace_id, project_key),
    FOREIGN KEY (workspace_id, project_key) REFERENCES projects (workspace_id, key) ON DELETE CASCADE
);
