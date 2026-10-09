-- Sprint 04: issue details (KYB-S15) and comments (KYB-S16).
ALTER TABLE issues
    ADD COLUMN description text NOT NULL DEFAULT '' CHECK (char_length(description) <= 20000),
    ADD COLUMN priority    text NOT NULL DEFAULT 'medium'
        CHECK (priority IN ('lowest', 'low', 'medium', 'high', 'highest')),
    ADD COLUMN assignee_id uuid REFERENCES users (id) ON DELETE SET NULL;
CREATE INDEX issues_assignee ON issues (assignee_id) WHERE assignee_id IS NOT NULL;

CREATE TABLE comments (
    id         uuid PRIMARY KEY,
    issue_id   uuid NOT NULL REFERENCES issues (id) ON DELETE CASCADE,
    author_id  uuid NOT NULL REFERENCES users (id),
    body       text NOT NULL CHECK (char_length(body) BETWEEN 1 AND 10000),
    created_at timestamptz NOT NULL
);
CREATE INDEX comments_issue ON comments (issue_id, created_at, id);
