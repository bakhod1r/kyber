-- Sprint 12 (S69): channel admins (besides the owner, created_by) and archiving.
ALTER TABLE conversations ADD COLUMN admin_ids uuid[] NOT NULL DEFAULT '{}';
ALTER TABLE conversations ADD COLUMN archived_at timestamptz;
