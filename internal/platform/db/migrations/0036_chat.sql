-- Sprint 11 (S50, ADR-0005): team chat — channels, direct messages and their messages.
CREATE TABLE conversations (
    id           uuid PRIMARY KEY,
    workspace_id uuid NOT NULL REFERENCES workspaces (id),
    kind         text NOT NULL CHECK (kind IN ('channel', 'dm')),
    name         text NOT NULL DEFAULT '',
    topic        text NOT NULL DEFAULT '',
    private      boolean NOT NULL,
    dm_key       text,
    created_by   uuid NOT NULL REFERENCES users (id),
    created_at   timestamptz NOT NULL,
    CHECK ((kind = 'channel' AND name ~ '^[a-z0-9_-]{1,80}$' AND dm_key IS NULL) OR (kind = 'dm' AND private AND dm_key IS NOT NULL))
);
CREATE UNIQUE INDEX conversations_channel_name ON conversations (workspace_id, name) WHERE kind = 'channel';
CREATE UNIQUE INDEX conversations_dm_key ON conversations (workspace_id, dm_key) WHERE kind = 'dm';

CREATE TABLE conversation_members (
    conversation_id uuid NOT NULL REFERENCES conversations (id) ON DELETE CASCADE,
    user_id         uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    joined_at       timestamptz NOT NULL,
    last_read_at    timestamptz NOT NULL DEFAULT '-infinity',
    PRIMARY KEY (conversation_id, user_id)
);
CREATE INDEX conversation_members_user ON conversation_members (user_id);

CREATE TABLE messages (
    id              uuid PRIMARY KEY,
    conversation_id uuid NOT NULL REFERENCES conversations (id) ON DELETE CASCADE,
    author_id       uuid NOT NULL REFERENCES users (id),
    body            text NOT NULL CHECK (char_length(body) <= 4000),
    created_at      timestamptz NOT NULL,
    edited_at       timestamptz,
    deleted         boolean NOT NULL DEFAULT false,
    CHECK (deleted OR btrim(body) <> '')
);
CREATE INDEX messages_history ON messages (conversation_id, created_at DESC, id DESC);
