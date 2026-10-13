-- C5 Attachments: what is known about files on issues; the bytes live in the blob store
-- (KYBER_ATTACHMENTS_DIR). Issues and users are referenced by key and id only (other contexts).
CREATE TABLE attachments (
    id           uuid PRIMARY KEY,
    workspace_id uuid NOT NULL REFERENCES workspaces (id) ON DELETE CASCADE,
    issue_key    text NOT NULL,
    name         text NOT NULL,
    content_type text NOT NULL,
    size         bigint NOT NULL CHECK (size > 0),
    sha256       text NOT NULL,
    uploaded_by  uuid NOT NULL,
    uploaded_at  timestamptz NOT NULL
);
CREATE INDEX attachments_issue ON attachments (workspace_id, issue_key, uploaded_at);

-- Row-level security as for every workspace table (ADR-0010).
ALTER TABLE attachments ENABLE ROW LEVEL SECURITY;
ALTER TABLE attachments FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON attachments
    USING (nullif(current_setting('kyber.workspace', true), '') IS NULL
           OR workspace_id = nullif(current_setting('kyber.workspace', true), '')::uuid)
    WITH CHECK (nullif(current_setting('kyber.workspace', true), '') IS NULL
           OR workspace_id = nullif(current_setting('kyber.workspace', true), '')::uuid);
