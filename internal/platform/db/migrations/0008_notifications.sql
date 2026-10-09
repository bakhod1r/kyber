-- Sprint 06 (KYB-S23): in-app notifications. (source_event, recipient_id) is unique so
-- that at-least-once outbox delivery never produces duplicates.
CREATE TABLE notifications (
    id           uuid PRIMARY KEY,
    recipient_id uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    kind         text NOT NULL CHECK (kind IN ('assigned', 'commented', 'mentioned')),
    issue_key    text NOT NULL,
    issue_title  text NOT NULL,
    actor_name   text NOT NULL,
    excerpt      text NOT NULL DEFAULT '',
    source_event bigint NOT NULL,
    created_at   timestamptz NOT NULL,
    read_at      timestamptz,
    UNIQUE (source_event, recipient_id)
);
CREATE INDEX notifications_inbox ON notifications (recipient_id, created_at DESC);
CREATE INDEX notifications_unread ON notifications (recipient_id) WHERE read_at IS NULL;
