-- F4 Comment reactions: people put one of a fixed set of emoji on comments (Issue context).
-- Users are referenced by id only (Identity context).
CREATE TABLE comment_reactions (
    workspace_id uuid NOT NULL REFERENCES workspaces (id) ON DELETE CASCADE,
    comment_id   uuid NOT NULL REFERENCES comments (id) ON DELETE CASCADE,
    user_id      uuid NOT NULL,
    emoji        text NOT NULL CHECK (emoji IN ('👍', '👎', '😄', '🎉', '😕', '❤️', '🚀', '👀')),
    reacted_at   timestamptz NOT NULL,
    PRIMARY KEY (comment_id, user_id, emoji)
);
CREATE INDEX comment_reactions_comment ON comment_reactions (workspace_id, comment_id, reacted_at);

-- Row-level security as for every workspace table (ADR-0010).
ALTER TABLE comment_reactions ENABLE ROW LEVEL SECURITY;
ALTER TABLE comment_reactions FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON comment_reactions
    USING (nullif(current_setting('kyber.workspace', true), '') IS NULL
           OR workspace_id = nullif(current_setting('kyber.workspace', true), '')::uuid)
    WITH CHECK (nullif(current_setting('kyber.workspace', true), '') IS NULL
           OR workspace_id = nullif(current_setting('kyber.workspace', true), '')::uuid);
