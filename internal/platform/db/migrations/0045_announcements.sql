-- E1: announcements to a project's or a workspace's members, delivered as notifications.
CREATE TABLE announcements (
    id           uuid PRIMARY KEY,
    workspace_id uuid NOT NULL REFERENCES workspaces (id) ON DELETE CASCADE,
    project_key  text NOT NULL DEFAULT '', -- '' = the whole workspace; by key only (no cross-context FK)
    title        text NOT NULL,
    body         text NOT NULL,
    author_id    uuid NOT NULL,
    created_at   timestamptz NOT NULL
);
CREATE INDEX announcements_feed ON announcements (workspace_id, project_key, created_at DESC);

-- Notifications about announcements point at a project (or none), not at an issue.
ALTER TABLE notifications ADD COLUMN project_key text NOT NULL DEFAULT '';
ALTER TABLE notifications DROP CONSTRAINT notifications_kind_check;
ALTER TABLE notifications ADD CONSTRAINT notifications_kind_check
    CHECK (kind IN ('assigned', 'commented', 'mentioned', 'transitioned', 'announcement'));
