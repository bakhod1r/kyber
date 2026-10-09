-- Architecture review: inboxes are per workspace (ADR-0004). Existing rows take the
-- workspace of the issue's project.
ALTER TABLE notifications ADD COLUMN workspace_id uuid;
UPDATE notifications n SET workspace_id = p.workspace_id
FROM projects p WHERE p.key = regexp_replace(n.issue_key, '-[0-9]+$', '');
UPDATE notifications SET workspace_id = '00000000-0000-4000-8000-000000000001' WHERE workspace_id IS NULL;
ALTER TABLE notifications ALTER COLUMN workspace_id SET NOT NULL;
DROP INDEX notifications_inbox;
DROP INDEX notifications_unread;
CREATE INDEX notifications_inbox ON notifications (recipient_id, workspace_id, created_at DESC);
CREATE INDEX notifications_unread ON notifications (recipient_id, workspace_id) WHERE read_at IS NULL;
