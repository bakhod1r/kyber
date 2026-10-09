-- Sprint 07 (KYB-S26): Insights activity log, a read model fed by the outbox relay.
CREATE TABLE activity (
    event       bigint NOT NULL,
    at          timestamptz NOT NULL,
    project_key text NOT NULL,
    issue_id    text NOT NULL,
    kind        text NOT NULL CHECK (kind IN ('created', 'status', 'sprint', 'estimate')),
    from_val    text NOT NULL DEFAULT '',
    to_val      text NOT NULL DEFAULT '',
    from_points integer,
    to_points   integer,
    PRIMARY KEY (event, issue_id, kind)
);
CREATE INDEX activity_project ON activity (project_key, at, event);

-- backfill: published events are never deleted from the outbox, so history since v0.2 is complete.
INSERT INTO activity (event, at, project_key, issue_id, kind, from_val, to_val, from_points, to_points)
SELECT o.id, o.created_at, regexp_replace(o.payload->>'key', '-[0-9]+$', ''), o.payload->>'id',
       CASE o.name WHEN 'issue.created' THEN 'created' WHEN 'issue.transitioned' THEN 'status'
                   WHEN 'issue.sprint_changed' THEN 'sprint' ELSE 'estimate' END,
       CASE WHEN o.name IN ('issue.transitioned', 'issue.sprint_changed') THEN coalesce(o.payload->>'from', '') ELSE '' END,
       CASE o.name WHEN 'issue.created' THEN 'todo'
                   WHEN 'issue.estimated' THEN ''
                   ELSE coalesce(o.payload->>'to', '') END,
       CASE WHEN o.name = 'issue.estimated' AND jsonb_typeof(o.payload->'from') = 'number'
            THEN round((o.payload->>'from')::numeric * 10)::int END,
       CASE WHEN o.name = 'issue.estimated' AND jsonb_typeof(o.payload->'to') = 'number'
            THEN round((o.payload->>'to')::numeric * 10)::int END
FROM outbox o
WHERE o.name IN ('issue.created', 'issue.transitioned', 'issue.sprint_changed', 'issue.estimated')
  AND o.payload ? 'key' AND o.payload ? 'id'
ON CONFLICT DO NOTHING;
