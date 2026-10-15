-- S106: identity-provider groups mapped to workspace and project roles, and the roles SSO granted
-- (so SSO only ever revokes what it gave). Site-wide configuration: targets name their workspace;
-- written at login without a workspace scope, so no row-level security.
CREATE TABLE sso_group_mappings (
    id           uuid PRIMARY KEY,
    group_name   text        NOT NULL,
    scope        text        NOT NULL CHECK (scope IN ('workspace', 'project')),
    workspace_id uuid        NOT NULL REFERENCES workspaces (id) ON DELETE CASCADE,
    project_key  text        NOT NULL DEFAULT '',
    role         text        NOT NULL,
    created_by   uuid        REFERENCES users (id) ON DELETE SET NULL,
    created_at   timestamptz NOT NULL
);

CREATE TABLE sso_grants (
    user_id      uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    scope        text NOT NULL,
    workspace_id uuid NOT NULL REFERENCES workspaces (id) ON DELETE CASCADE,
    project_key  text NOT NULL DEFAULT '',
    role         text NOT NULL,
    PRIMARY KEY (user_id, scope, workspace_id, project_key)
);
