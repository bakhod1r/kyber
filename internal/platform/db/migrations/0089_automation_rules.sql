-- G8 automation rules: when an issue event happens and the issue matches a JQL condition, run actions as the
-- rule's owner. Projects and users are referenced by key and id only (other contexts).
CREATE TABLE automation_rules (
    id           uuid PRIMARY KEY,
    workspace_id uuid NOT NULL REFERENCES workspaces (id) ON DELETE CASCADE,
    project_key  text NOT NULL,
    owner_id     uuid NOT NULL,
    name         text NOT NULL,
    trigger      text NOT NULL,
    to_status    text NOT NULL DEFAULT '',
    condition    text NOT NULL DEFAULT '',
    actions      jsonb NOT NULL,
    enabled      boolean NOT NULL,
    version      int NOT NULL
);
CREATE INDEX automation_rules_project ON automation_rules (workspace_id, project_key);

-- The recent runs of a project's rules (an audit trail; old rows can be pruned).
CREATE TABLE automation_runs (
    id           bigserial PRIMARY KEY,
    workspace_id uuid NOT NULL REFERENCES workspaces (id) ON DELETE CASCADE,
    rule_id      uuid NOT NULL,
    project_key  text NOT NULL,
    issue_key    text NOT NULL,
    at           timestamptz NOT NULL,
    ok           boolean NOT NULL,
    detail       text NOT NULL
);
CREATE INDEX automation_runs_recent ON automation_runs (workspace_id, project_key, at DESC);

-- Row-level security as for every workspace table (ADR-0010).
ALTER TABLE automation_rules ENABLE ROW LEVEL SECURITY;
ALTER TABLE automation_rules FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON automation_rules
    USING (nullif(current_setting('kyber.workspace', true), '') IS NULL
           OR workspace_id = nullif(current_setting('kyber.workspace', true), '')::uuid)
    WITH CHECK (nullif(current_setting('kyber.workspace', true), '') IS NULL
           OR workspace_id = nullif(current_setting('kyber.workspace', true), '')::uuid);
ALTER TABLE automation_runs ENABLE ROW LEVEL SECURITY;
ALTER TABLE automation_runs FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON automation_runs
    USING (nullif(current_setting('kyber.workspace', true), '') IS NULL
           OR workspace_id = nullif(current_setting('kyber.workspace', true), '')::uuid)
    WITH CHECK (nullif(current_setting('kyber.workspace', true), '') IS NULL
           OR workspace_id = nullif(current_setting('kyber.workspace', true), '')::uuid);
