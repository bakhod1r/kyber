-- Issue history (Jira's History tab): who did what to an issue, and when, with old and new values.
-- Filled by the relay from issue.created / issue.changed / comment.added / issue.imported.
CREATE TABLE issue_history (
    workspace_id uuid NOT NULL,
    event        bigint NOT NULL,
    seq          int NOT NULL,
    issue_id     text NOT NULL,
    at           timestamptz NOT NULL,
    by_user      text NOT NULL DEFAULT '',
    field        text NOT NULL,
    from_val     text NOT NULL DEFAULT '',
    to_val       text NOT NULL DEFAULT '',
    PRIMARY KEY (workspace_id, event, seq)
);
CREATE INDEX issue_history_issue ON issue_history (workspace_id, issue_id, at, event, seq);

-- Backfill: outbox events are never deleted, so every past action is recovered. Events written before
-- issue.changed existed carry no actor (except assignments) and edits carry no values; those stay blank.
INSERT INTO issue_history (workspace_id, event, seq, issue_id, at, by_user, field, from_val, to_val)
SELECT o.workspace_id, o.id, 0, o.payload->>'id', o.created_at,
       CASE o.name WHEN 'issue.created' THEN coalesce(o.payload->>'reporter', '')
                   WHEN 'issue.assigned' THEN coalesce(o.payload->>'by', '') ELSE '' END,
       CASE o.name WHEN 'issue.created' THEN 'created' WHEN 'issue.transitioned' THEN 'status'
                   WHEN 'issue.assigned' THEN 'assignee' WHEN 'issue.sprint_changed' THEN 'sprint'
                   WHEN 'issue.estimated' THEN 'estimate' ELSE 'imported' END,
       CASE WHEN o.name IN ('issue.created', 'issue.imported') THEN '' ELSE coalesce(o.payload->>'from', '') END,
       CASE o.name WHEN 'issue.created' THEN coalesce(o.payload->>'title', '')
                   WHEN 'issue.imported' THEN coalesce(o.payload->>'external_key', '')
                   ELSE coalesce(o.payload->>'to', '') END
FROM outbox o
WHERE o.name IN ('issue.created', 'issue.transitioned', 'issue.assigned', 'issue.sprint_changed', 'issue.estimated', 'issue.imported')
  AND o.payload ? 'id';

INSERT INTO issue_history (workspace_id, event, seq, issue_id, at, field)
SELECT o.workspace_id, o.id, (f.n - 1)::int, o.payload->>'id', o.created_at, f.field
FROM outbox o, jsonb_array_elements_text(o.payload->'fields') WITH ORDINALITY AS f(field, n)
WHERE o.name = 'issue.edited' AND o.payload ? 'id';

INSERT INTO issue_history (workspace_id, event, seq, issue_id, at, by_user, field, to_val)
SELECT o.workspace_id, o.id, 0, o.payload->>'issue_id', o.created_at, coalesce(o.payload->>'author', ''), 'comment', coalesce(o.payload->>'body', '')
FROM outbox o
WHERE o.name = 'comment.added' AND o.payload ? 'issue_id';
