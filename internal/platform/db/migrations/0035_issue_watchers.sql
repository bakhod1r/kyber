-- Watchers (Jira): people following an issue get its comments and status changes.
CREATE TABLE issue_watchers (
    issue_id   uuid NOT NULL REFERENCES issues (id) ON DELETE CASCADE,
    user_id    uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    watched_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (issue_id, user_id)
);
CREATE INDEX issue_watchers_user ON issue_watchers (user_id);

-- Notifications for watchers: status changes.
ALTER TABLE notifications DROP CONSTRAINT notifications_kind_check;
ALTER TABLE notifications ADD CONSTRAINT notifications_kind_check CHECK (kind IN ('assigned', 'commented', 'mentioned', 'transitioned'));
