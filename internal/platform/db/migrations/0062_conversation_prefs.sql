-- Sprint 12 (S71): per-person conversation settings — notification level and sidebar section.
CREATE TABLE conversation_prefs (
    workspace_id    uuid NOT NULL,
    user_id         uuid NOT NULL,
    conversation_id uuid NOT NULL REFERENCES conversations (id) ON DELETE CASCADE,
    level           text NOT NULL CHECK (level IN ('all', 'mentions', 'muted')),
    section         text NOT NULL DEFAULT '' CHECK (char_length(section) <= 40),
    PRIMARY KEY (user_id, conversation_id)
);
CREATE INDEX conversation_prefs_conv ON conversation_prefs (conversation_id);

ALTER TABLE conversation_prefs ENABLE ROW LEVEL SECURITY;
ALTER TABLE conversation_prefs FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON conversation_prefs
    USING (nullif(current_setting('kyber.workspace', true), '') IS NULL
           OR workspace_id = nullif(current_setting('kyber.workspace', true), '')::uuid)
    WITH CHECK (nullif(current_setting('kyber.workspace', true), '') IS NULL
           OR workspace_id = nullif(current_setting('kyber.workspace', true), '')::uuid);

-- "all" sends a notification of kind 'message' for each message.
ALTER TABLE notifications DROP CONSTRAINT notifications_kind_check;
ALTER TABLE notifications ADD CONSTRAINT notifications_kind_check
    CHECK (kind IN ('assigned', 'commented', 'mentioned', 'transitioned', 'announcement', 'meeting', 'reminder', 'message'));
