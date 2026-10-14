-- Sprint 11 (S59, ADR-0009): whiteboards drawn together in a conversation. Conversations, users and
-- workspaces belong to other contexts: referenced by id only.
CREATE TABLE whiteboards (
    id              uuid PRIMARY KEY,
    workspace_id    uuid NOT NULL,
    conversation_id uuid NOT NULL,
    title           text NOT NULL CHECK (char_length(btrim(title)) BETWEEN 1 AND 120),
    created_by      uuid NOT NULL,
    created_at      timestamptz NOT NULL
);
CREATE INDEX whiteboards_conversation ON whiteboards (conversation_id, created_at DESC);

-- One row per element; version is the per-element optimistic lock (last writer wins per shape).
CREATE TABLE whiteboard_elements (
    whiteboard_id uuid NOT NULL REFERENCES whiteboards (id) ON DELETE CASCADE,
    element_id    text NOT NULL,
    kind          text NOT NULL CHECK (kind IN ('rect', 'ellipse', 'arrow', 'line', 'freehand', 'text', 'sticky')),
    x             double precision NOT NULL,
    y             double precision NOT NULL,
    w             double precision NOT NULL,
    h             double precision NOT NULL,
    points        jsonb NOT NULL DEFAULT '[]',
    body          text NOT NULL DEFAULT '',
    color         text NOT NULL DEFAULT '',
    version       integer NOT NULL,
    updated_by    uuid NOT NULL,
    updated_at    timestamptz NOT NULL,
    PRIMARY KEY (whiteboard_id, element_id)
);
