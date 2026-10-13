-- Sprint 12 (S79): docs and links kept at hand in a conversation's header (docs by id: Docs context).
CREATE TABLE conversation_bookmarks (
    id              uuid PRIMARY KEY,
    workspace_id    uuid NOT NULL,
    conversation_id uuid NOT NULL REFERENCES conversations (id) ON DELETE CASCADE,
    kind            text NOT NULL CHECK (kind IN ('doc', 'link')),
    ref             text NOT NULL CHECK (char_length(ref) <= 2000),
    title           text NOT NULL CHECK (char_length(title) BETWEEN 1 AND 100),
    added_by        uuid NOT NULL,
    added_at        timestamptz NOT NULL
);
CREATE INDEX conversation_bookmarks_conv ON conversation_bookmarks (conversation_id, added_at);

ALTER TABLE conversation_bookmarks ENABLE ROW LEVEL SECURITY;
ALTER TABLE conversation_bookmarks FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON conversation_bookmarks
    USING (nullif(current_setting('kyber.workspace', true), '') IS NULL
           OR workspace_id = nullif(current_setting('kyber.workspace', true), '')::uuid)
    WITH CHECK (nullif(current_setting('kyber.workspace', true), '') IS NULL
           OR workspace_id = nullif(current_setting('kyber.workspace', true), '')::uuid);
