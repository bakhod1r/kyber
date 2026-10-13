-- Sprint 12 (S65): messages saved for later, optionally with a reminder. The user belongs to Identity: by id only.
CREATE TABLE saved_messages (
    workspace_id    uuid NOT NULL,
    user_id         uuid NOT NULL,
    message_id      uuid NOT NULL REFERENCES messages (id) ON DELETE CASCADE,
    conversation_id uuid NOT NULL,
    saved_at        timestamptz NOT NULL,
    remind_at       timestamptz,
    reminded_at     timestamptz,
    PRIMARY KEY (workspace_id, user_id, message_id)
);
CREATE INDEX saved_messages_by_user ON saved_messages (workspace_id, user_id, saved_at DESC);
CREATE INDEX saved_messages_due ON saved_messages (remind_at) WHERE remind_at IS NOT NULL AND reminded_at IS NULL;

ALTER TABLE saved_messages ENABLE ROW LEVEL SECURITY;
ALTER TABLE saved_messages FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON saved_messages
    USING (nullif(current_setting('kyber.workspace', true), '') IS NULL
           OR workspace_id = nullif(current_setting('kyber.workspace', true), '')::uuid)
    WITH CHECK (nullif(current_setting('kyber.workspace', true), '') IS NULL
           OR workspace_id = nullif(current_setting('kyber.workspace', true), '')::uuid);

-- Their reminders arrive as notifications of kind 'reminder'.
ALTER TABLE notifications DROP CONSTRAINT notifications_kind_check;
ALTER TABLE notifications ADD CONSTRAINT notifications_kind_check
    CHECK (kind IN ('assigned', 'commented', 'mentioned', 'transitioned', 'announcement', 'meeting', 'reminder'));
