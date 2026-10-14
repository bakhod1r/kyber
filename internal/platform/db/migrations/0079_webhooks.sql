-- F5 Webhooks (ADR-0014): a project's outgoing webhooks, their delivery queue, and development
-- links (commits and pull requests mentioning an issue key). Projects and issues are referenced
-- by key only (other contexts).
CREATE TABLE webhooks (
    id           uuid PRIMARY KEY,
    workspace_id uuid NOT NULL REFERENCES workspaces (id) ON DELETE CASCADE,
    project_key  text NOT NULL,
    url          text NOT NULL,
    secret       text NOT NULL, -- HMAC key; needed in clear to sign every delivery
    events       text[] NOT NULL,
    active       boolean NOT NULL,
    created_by   uuid NOT NULL,
    created_at   timestamptz NOT NULL
);
CREATE INDEX webhooks_project ON webhooks (workspace_id, project_key, created_at);

CREATE TABLE webhook_deliveries (
    id           uuid PRIMARY KEY,
    workspace_id uuid NOT NULL REFERENCES workspaces (id) ON DELETE CASCADE,
    webhook_id   uuid NOT NULL REFERENCES webhooks (id) ON DELETE CASCADE,
    event        text NOT NULL,
    key          text NOT NULL UNIQUE, -- <outbox message>/<webhook>: redelivered events enqueue once
    body         bytea NOT NULL,
    status       text NOT NULL CHECK (status IN ('pending', 'delivered', 'failed')),
    attempts     int NOT NULL DEFAULT 0,
    next_at      timestamptz NOT NULL,
    last_status  int NOT NULL DEFAULT 0,
    last_error   text NOT NULL DEFAULT '',
    created_at   timestamptz NOT NULL,
    delivered_at timestamptz
);
CREATE INDEX webhook_deliveries_due ON webhook_deliveries (next_at) WHERE status = 'pending';
CREATE INDEX webhook_deliveries_recent ON webhook_deliveries (webhook_id, created_at DESC);

CREATE TABLE development_links (
    workspace_id uuid NOT NULL REFERENCES workspaces (id) ON DELETE CASCADE,
    issue_key    text NOT NULL,
    kind         text NOT NULL CHECK (kind IN ('commit', 'pull_request')),
    repo         text NOT NULL,
    ref          text NOT NULL,
    url          text NOT NULL,
    title        text NOT NULL,
    state        text NOT NULL DEFAULT '',
    updated_at   timestamptz NOT NULL,
    PRIMARY KEY (workspace_id, issue_key, kind, repo, ref)
);

-- Row-level security as for every workspace table (ADR-0010).
ALTER TABLE webhooks ENABLE ROW LEVEL SECURITY;
ALTER TABLE webhooks FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON webhooks
    USING (nullif(current_setting('kyber.workspace', true), '') IS NULL
           OR workspace_id = nullif(current_setting('kyber.workspace', true), '')::uuid)
    WITH CHECK (nullif(current_setting('kyber.workspace', true), '') IS NULL
           OR workspace_id = nullif(current_setting('kyber.workspace', true), '')::uuid);
ALTER TABLE webhook_deliveries ENABLE ROW LEVEL SECURITY;
ALTER TABLE webhook_deliveries FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON webhook_deliveries
    USING (nullif(current_setting('kyber.workspace', true), '') IS NULL
           OR workspace_id = nullif(current_setting('kyber.workspace', true), '')::uuid)
    WITH CHECK (nullif(current_setting('kyber.workspace', true), '') IS NULL
           OR workspace_id = nullif(current_setting('kyber.workspace', true), '')::uuid);
ALTER TABLE development_links ENABLE ROW LEVEL SECURITY;
ALTER TABLE development_links FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON development_links
    USING (nullif(current_setting('kyber.workspace', true), '') IS NULL
           OR workspace_id = nullif(current_setting('kyber.workspace', true), '')::uuid)
    WITH CHECK (nullif(current_setting('kyber.workspace', true), '') IS NULL
           OR workspace_id = nullif(current_setting('kyber.workspace', true), '')::uuid);
