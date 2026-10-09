-- Sprint 09 (KYB-S33): workspaces (tenants) on subdomains, ADR-0004.
-- Phase 1: projects belong to a workspace; project keys stay unique per installation.
CREATE TABLE workspaces (
    id         uuid PRIMARY KEY,
    slug       text NOT NULL UNIQUE CHECK (slug ~ '^[a-z0-9]+(-[a-z0-9]+)*$' AND length(slug) BETWEEN 3 AND 32),
    name       text NOT NULL CHECK (btrim(name) <> ''),
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE workspace_members (
    workspace_id uuid NOT NULL REFERENCES workspaces (id) ON DELETE CASCADE,
    user_id      uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    role         text NOT NULL CHECK (role IN ('owner', 'admin', 'member')),
    PRIMARY KEY (workspace_id, user_id)
);
CREATE INDEX workspace_members_user ON workspace_members (user_id);

-- Everything that exists today lives in the default workspace (single-tenant mode).
INSERT INTO workspaces (id, slug, name) VALUES ('00000000-0000-4000-8000-000000000001', 'default', 'Default');
ALTER TABLE projects ADD COLUMN workspace_id uuid NOT NULL DEFAULT '00000000-0000-4000-8000-000000000001' REFERENCES workspaces (id);
ALTER TABLE projects ALTER COLUMN workspace_id DROP DEFAULT;
CREATE INDEX projects_workspace ON projects (workspace_id);
