-- Sprint 11 (S50, ADR-0005): team chat — channels, direct messages and their messages.
-- Users and workspaces belong to other contexts: referenced by id only (ADR-0005, no cross-context FKs).
CREATE TABLE conversations (
    id           uuid PRIMARY KEY,
    workspace_id uuid NOT NULL,
    kind         text NOT NULL CHECK (kind IN ('channel', 'dm')),
    name         text NOT NULL DEFAULT '',
    topic        text NOT NULL DEFAULT '',
    private      boolean NOT NULL,
    dm_key       text,
    created_by   uuid NOT NULL,
    created_at   timestamptz NOT NULL,
    CHECK ((kind = 'channel' AND name ~ '^[a-z0-9_-]{1,80}$' AND dm_key IS NULL) OR (kind = 'dm' AND private AND dm_key IS NOT NULL))
);
CREATE UNIQUE INDEX conversations_channel_name ON conversations (workspace_id, name) WHERE kind = 'channel';
CREATE UNIQUE INDEX conversations_dm_key ON conversations (workspace_id, dm_key) WHERE kind = 'dm';

CREATE TABLE conversation_members (
    conversation_id uuid NOT NULL REFERENCES conversations (id) ON DELETE CASCADE,
    user_id         uuid NOT NULL,
    joined_at       timestamptz NOT NULL,
    last_read_at    timestamptz NOT NULL DEFAULT '-infinity',
    PRIMARY KEY (conversation_id, user_id)
);
CREATE INDEX conversation_members_user ON conversation_members (user_id);

CREATE TABLE messages (
    id              uuid PRIMARY KEY,
    conversation_id uuid NOT NULL REFERENCES conversations (id) ON DELETE CASCADE,
    author_id       uuid NOT NULL,
    body            text NOT NULL CHECK (char_length(body) <= 4000),
    created_at      timestamptz NOT NULL,
    edited_at       timestamptz,
    deleted         boolean NOT NULL DEFAULT false,
    thread_id       uuid REFERENCES messages (id) ON DELETE CASCADE,
    -- System messages (e.g. a huddle started): their kind, and what they are about (unique).
    kind            text NOT NULL DEFAULT '' CHECK (kind IN ('', 'huddle_started', 'huddle_ended', 'huddle_recording')),
    ref             text NOT NULL DEFAULT '',
    -- S54b: an attached file (its bytes are in the blob store under the message id)
    file_name       text,
    file_type       text,
    file_size       bigint CHECK (file_size BETWEEN 1 AND 1073741824),
    pinned_by       uuid,
    pinned_at       timestamptz,
    CHECK (deleted OR btrim(body) <> '')
);
CREATE INDEX messages_history ON messages (conversation_id, created_at DESC, id DESC) WHERE thread_id IS NULL;
CREATE INDEX messages_pinned ON messages (conversation_id, pinned_at DESC) WHERE pinned_at IS NOT NULL;
CREATE UNIQUE INDEX messages_ref ON messages (ref) WHERE ref <> '';
CREATE INDEX messages_thread ON messages (thread_id, created_at) WHERE thread_id IS NOT NULL;

CREATE TABLE message_reactions (
    message_id uuid NOT NULL REFERENCES messages (id) ON DELETE CASCADE,
    user_id    uuid NOT NULL,
    emoji      text NOT NULL CHECK (char_length(emoji) BETWEEN 1 AND 32),
    PRIMARY KEY (message_id, emoji, user_id)
);
