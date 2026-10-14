-- Sprint 12 (S81): incoming webhooks. Only the secret's SHA-256 is stored; the URL is shown once.
CREATE TABLE chat_webhooks (
    id              uuid PRIMARY KEY,
    workspace_id    uuid NOT NULL,
    conversation_id uuid NOT NULL REFERENCES conversations (id) ON DELETE CASCADE,
    name            text NOT NULL CHECK (char_length(name) BETWEEN 1 AND 40),
    secret_hash     text NOT NULL,
    created_by      uuid NOT NULL,
    created_at      timestamptz NOT NULL
);
CREATE UNIQUE INDEX chat_webhooks_secret ON chat_webhooks (secret_hash);
CREATE INDEX chat_webhooks_conv ON chat_webhooks (conversation_id, created_at);

ALTER TABLE chat_webhooks ENABLE ROW LEVEL SECURITY;
ALTER TABLE chat_webhooks FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON chat_webhooks
    USING (nullif(current_setting('kyber.workspace', true), '') IS NULL
           OR workspace_id = nullif(current_setting('kyber.workspace', true), '')::uuid)
    WITH CHECK (nullif(current_setting('kyber.workspace', true), '') IS NULL
           OR workspace_id = nullif(current_setting('kyber.workspace', true), '')::uuid);

ALTER TABLE messages DROP CONSTRAINT messages_kind_check;
ALTER TABLE messages ADD CONSTRAINT messages_kind_check
    CHECK (kind IN ('', 'huddle_started', 'huddle_ended', 'huddle_recording', 'project_event', 'transcript', 'workflow', 'webhook'));
