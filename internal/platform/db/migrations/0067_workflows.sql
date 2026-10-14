-- Sprint 12 (S80): simple automations in a channel — a welcome on join, or a message on a schedule.
CREATE TABLE chat_workflows (
    id              uuid PRIMARY KEY,
    workspace_id    uuid NOT NULL,
    conversation_id uuid NOT NULL REFERENCES conversations (id) ON DELETE CASCADE,
    trigger         text NOT NULL CHECK (trigger IN ('member_joined', 'schedule')),
    minute          integer NOT NULL CHECK (minute BETWEEN 0 AND 1439),
    days            smallint NOT NULL CHECK (days BETWEEN 0 AND 127),
    text            text NOT NULL CHECK (char_length(text) <= 4000),
    created_by      uuid NOT NULL,
    created_at      timestamptz NOT NULL,
    last_run        timestamptz NOT NULL
);
CREATE INDEX chat_workflows_conv ON chat_workflows (conversation_id, created_at);
CREATE INDEX chat_workflows_scheduled ON chat_workflows (trigger) WHERE trigger = 'schedule';

ALTER TABLE chat_workflows ENABLE ROW LEVEL SECURITY;
ALTER TABLE chat_workflows FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON chat_workflows
    USING (nullif(current_setting('kyber.workspace', true), '') IS NULL
           OR workspace_id = nullif(current_setting('kyber.workspace', true), '')::uuid)
    WITH CHECK (nullif(current_setting('kyber.workspace', true), '') IS NULL
           OR workspace_id = nullif(current_setting('kyber.workspace', true), '')::uuid);

ALTER TABLE messages DROP CONSTRAINT messages_kind_check;
ALTER TABLE messages ADD CONSTRAINT messages_kind_check
    CHECK (kind IN ('', 'huddle_started', 'huddle_ended', 'huddle_recording', 'project_event', 'transcript', 'workflow'));
