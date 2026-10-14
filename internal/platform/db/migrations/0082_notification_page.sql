-- Sprint 15 (S95): a doc comment notification links to the commented page.
ALTER TABLE notifications ADD COLUMN page_id text NOT NULL DEFAULT '';
