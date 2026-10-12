-- KYB-S45: issue links (blocks, relates, duplicates, clones). Deleting an issue removes its links.
CREATE TABLE issue_links (
    id           uuid PRIMARY KEY,
    workspace_id uuid NOT NULL,
    type         text NOT NULL CHECK (type IN ('blocks', 'relates', 'duplicates', 'clones')),
    from_id      uuid NOT NULL REFERENCES issues (id) ON DELETE CASCADE,
    to_id        uuid NOT NULL REFERENCES issues (id) ON DELETE CASCADE,
    created_at   timestamptz NOT NULL DEFAULT now(),
    CHECK (from_id <> to_id),
    UNIQUE (workspace_id, type, from_id, to_id)
);
CREATE INDEX issue_links_from ON issue_links (workspace_id, from_id);
CREATE INDEX issue_links_to ON issue_links (workspace_id, to_id);
