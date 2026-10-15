-- B9: a daily email digest instead of one email per notification. digest_sent_at keeps the last send, so
-- each user gets one digest a day across replicas.
ALTER TABLE notification_prefs ADD COLUMN email_digest boolean NOT NULL DEFAULT false;
ALTER TABLE notification_prefs ADD COLUMN digest_sent_at timestamptz;
CREATE INDEX notification_prefs_digest ON notification_prefs (user_id) WHERE email_digest;
-- The digest lists a user's unread notifications across workspaces.
CREATE INDEX notifications_unread_by_recipient ON notifications (recipient_id, created_at DESC) WHERE read_at IS NULL;
