-- Sprint 12 (S70): user groups; mentioning @handle notifies the members. Users by id only.
CREATE TABLE user_groups (
    id           uuid PRIMARY KEY,
    workspace_id uuid NOT NULL,
    handle       text NOT NULL CHECK (handle ~ '^[a-z0-9][a-z0-9-]{1,29}$'),
    name         text NOT NULL DEFAULT '',
    member_ids   uuid[] NOT NULL DEFAULT '{}',
    created_by   uuid NOT NULL,
    created_at   timestamptz NOT NULL
);
CREATE UNIQUE INDEX user_groups_handle ON user_groups (workspace_id, handle);

ALTER TABLE user_groups ENABLE ROW LEVEL SECURITY;
ALTER TABLE user_groups FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON user_groups
    USING (nullif(current_setting('kyber.workspace', true), '') IS NULL
           OR workspace_id = nullif(current_setting('kyber.workspace', true), '')::uuid)
    WITH CHECK (nullif(current_setting('kyber.workspace', true), '') IS NULL
           OR workspace_id = nullif(current_setting('kyber.workspace', true), '')::uuid);
