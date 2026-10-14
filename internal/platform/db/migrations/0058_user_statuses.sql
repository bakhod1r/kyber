-- Sprint 12 (S66): a status (emoji, text, expiry) and Do Not Disturb per person and workspace.
CREATE TABLE user_statuses (
    workspace_id uuid NOT NULL,
    user_id      uuid NOT NULL,
    emoji        text NOT NULL,
    text         text NOT NULL CHECK (char_length(text) <= 100),
    expires_at   timestamptz,
    dnd_until    timestamptz,
    PRIMARY KEY (workspace_id, user_id)
);

ALTER TABLE user_statuses ENABLE ROW LEVEL SECURITY;
ALTER TABLE user_statuses FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON user_statuses
    USING (nullif(current_setting('kyber.workspace', true), '') IS NULL
           OR workspace_id = nullif(current_setting('kyber.workspace', true), '')::uuid)
    WITH CHECK (nullif(current_setting('kyber.workspace', true), '') IS NULL
           OR workspace_id = nullif(current_setting('kyber.workspace', true), '')::uuid);
