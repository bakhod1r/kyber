-- Sprint 13 (S86): comments pinned to whiteboard elements.
CREATE TABLE whiteboard_comments (
    id              uuid PRIMARY KEY,
    workspace_id    uuid NOT NULL,
    board_id        uuid NOT NULL REFERENCES whiteboards (id) ON DELETE CASCADE,
    conversation_id uuid NOT NULL,
    element_id      text NOT NULL,
    author_id       uuid NOT NULL,
    body            text NOT NULL CHECK (char_length(body) BETWEEN 1 AND 2000),
    created_at      timestamptz NOT NULL
);
CREATE INDEX whiteboard_comments_board ON whiteboard_comments (board_id, created_at);

ALTER TABLE whiteboard_comments ENABLE ROW LEVEL SECURITY;
ALTER TABLE whiteboard_comments FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON whiteboard_comments
    USING (nullif(current_setting('kyber.workspace', true), '') IS NULL
           OR workspace_id = nullif(current_setting('kyber.workspace', true), '')::uuid)
    WITH CHECK (nullif(current_setting('kyber.workspace', true), '') IS NULL
           OR workspace_id = nullif(current_setting('kyber.workspace', true), '')::uuid);
