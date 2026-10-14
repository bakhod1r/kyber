-- Sprint 13 (S84): a workspace's bot users (the users themselves live in Identity: by id only).
CREATE TABLE chat_bots (
    workspace_id uuid NOT NULL,
    user_id      uuid NOT NULL,
    name         text NOT NULL CHECK (char_length(name) BETWEEN 1 AND 100),
    created_by   uuid NOT NULL,
    created_at   timestamptz NOT NULL,
    PRIMARY KEY (workspace_id, user_id)
);

ALTER TABLE chat_bots ENABLE ROW LEVEL SECURITY;
ALTER TABLE chat_bots FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON chat_bots
    USING (nullif(current_setting('kyber.workspace', true), '') IS NULL
           OR workspace_id = nullif(current_setting('kyber.workspace', true), '')::uuid)
    WITH CHECK (nullif(current_setting('kyber.workspace', true), '') IS NULL
           OR workspace_id = nullif(current_setting('kyber.workspace', true), '')::uuid);
