-- The user's IANA time zone (sent by their browser) for times in email/Telegram messages;
-- '' = unknown (UTC). Meetings need no change: rescheduling clears reminded_at.
ALTER TABLE notification_prefs ADD COLUMN time_zone text NOT NULL DEFAULT '';
