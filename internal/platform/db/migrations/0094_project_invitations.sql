-- B1: invite people to a project by email. Only the link token's SHA-256 is stored; users are referenced by
-- id only (Identity context), projects by key.
CREATE TABLE project_invitations (
    id           uuid PRIMARY KEY,
    workspace_id uuid NOT NULL REFERENCES workspaces (id) ON DELETE CASCADE,
    project_key  text NOT NULL,
    email        text NOT NULL,
    role         text NOT NULL CHECK (role IN ('admin', 'member', 'viewer')),
    invited_by   uuid NOT NULL,
    token_hash   text NOT NULL UNIQUE,
    created_at   timestamptz NOT NULL,
    expires_at   timestamptz NOT NULL,
    accepted_by  uuid,
    accepted_at  timestamptz
);
-- One pending invitation per project and address: inviting again replaces it.
CREATE UNIQUE INDEX project_invitations_pending ON project_invitations (workspace_id, project_key, email)
    WHERE accepted_by IS NULL;

-- Row-level security as for every workspace table (ADR-0010).
ALTER TABLE project_invitations ENABLE ROW LEVEL SECURITY;
ALTER TABLE project_invitations FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON project_invitations
    USING (nullif(current_setting('kyber.workspace', true), '') IS NULL
           OR workspace_id = nullif(current_setting('kyber.workspace', true), '')::uuid)
    WITH CHECK (nullif(current_setting('kyber.workspace', true), '') IS NULL
           OR workspace_id = nullif(current_setting('kyber.workspace', true), '')::uuid);
