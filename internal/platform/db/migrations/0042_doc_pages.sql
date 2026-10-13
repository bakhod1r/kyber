-- D5 Docs: a project's Markdown pages and every saved revision. Project and users are referenced by
-- key and id only (other contexts; no cross-context foreign keys).
CREATE TABLE doc_pages (
    id           uuid PRIMARY KEY,
    workspace_id uuid NOT NULL REFERENCES workspaces (id) ON DELETE CASCADE,
    project_key  text NOT NULL,
    title        text NOT NULL,
    body         text NOT NULL,
    author_id    uuid NOT NULL,
    updated_by   uuid NOT NULL,
    updated_at   timestamptz NOT NULL,
    version      int NOT NULL
);
CREATE INDEX doc_pages_project ON doc_pages (workspace_id, project_key);

CREATE TABLE doc_revisions (
    page_id uuid NOT NULL REFERENCES doc_pages (id) ON DELETE CASCADE,
    number  int NOT NULL,
    title   text NOT NULL,
    body    text NOT NULL,
    by_id   uuid NOT NULL,
    at      timestamptz NOT NULL,
    PRIMARY KEY (page_id, number)
);
