-- Sprint 12 (S67): messages scheduled for later, and personal reminders (/remind me …).
CREATE TABLE scheduled_messages (
    id              uuid PRIMARY KEY,
    workspace_id    uuid NOT NULL,
    conversation_id uuid NOT NULL REFERENCES conversations (id) ON DELETE CASCADE,
    author_id       uuid NOT NULL,
    body            text NOT NULL CHECK (char_length(body) <= 4000),
    send_at         timestamptz NOT NULL,
    created_at      timestamptz NOT NULL
);
CREATE INDEX scheduled_messages_mine ON scheduled_messages (workspace_id, author_id, send_at);
CREATE INDEX scheduled_messages_due ON scheduled_messages (send_at);

CREATE TABLE reminders (
    id              uuid PRIMARY KEY,
    workspace_id    uuid NOT NULL,
    user_id         uuid NOT NULL,
    conversation_id uuid NOT NULL,
    text            text NOT NULL CHECK (char_length(text) <= 4000),
    remind_at       timestamptz NOT NULL,
    reminded_at     timestamptz
);
CREATE INDEX reminders_due ON reminders (remind_at) WHERE reminded_at IS NULL;

DO $$
DECLARE t text;
BEGIN
    FOREACH t IN ARRAY ARRAY['scheduled_messages', 'reminders'] LOOP
        EXECUTE format('ALTER TABLE %I ENABLE ROW LEVEL SECURITY', t);
        EXECUTE format('ALTER TABLE %I FORCE ROW LEVEL SECURITY', t);
        EXECUTE format($p$CREATE POLICY tenant_isolation ON %I
            USING (nullif(current_setting('kyber.workspace', true), '') IS NULL
                   OR workspace_id = nullif(current_setting('kyber.workspace', true), '')::uuid)
            WITH CHECK (nullif(current_setting('kyber.workspace', true), '') IS NULL
                   OR workspace_id = nullif(current_setting('kyber.workspace', true), '')::uuid)$p$, t);
    END LOOP;
END $$;
