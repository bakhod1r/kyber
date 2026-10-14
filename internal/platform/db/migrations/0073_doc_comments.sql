-- Sprint 13 (S86): comments on doc paragraphs; @mentions in them arrive as notifications of kind 'doc'.
CREATE TABLE doc_comments (
    id           uuid PRIMARY KEY,
    workspace_id uuid NOT NULL,
    page_id      uuid NOT NULL REFERENCES doc_pages (id) ON DELETE CASCADE,
    project_key  text NOT NULL,
    paragraph    integer NOT NULL CHECK (paragraph >= 0),
    quote        text NOT NULL CHECK (char_length(quote) <= 200),
    author_id    uuid NOT NULL,
    body         text NOT NULL CHECK (char_length(body) BETWEEN 1 AND 2000),
    created_at   timestamptz NOT NULL
);
CREATE INDEX doc_comments_page ON doc_comments (page_id, created_at);

ALTER TABLE doc_comments ENABLE ROW LEVEL SECURITY;
ALTER TABLE doc_comments FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON doc_comments
    USING (nullif(current_setting('kyber.workspace', true), '') IS NULL
           OR workspace_id = nullif(current_setting('kyber.workspace', true), '')::uuid)
    WITH CHECK (nullif(current_setting('kyber.workspace', true), '') IS NULL
           OR workspace_id = nullif(current_setting('kyber.workspace', true), '')::uuid);

ALTER TABLE notifications DROP CONSTRAINT notifications_kind_check;
ALTER TABLE notifications ADD CONSTRAINT notifications_kind_check
    CHECK (kind IN ('assigned', 'commented', 'mentioned', 'transitioned', 'announcement', 'meeting', 'reminder', 'message', 'doc'));
