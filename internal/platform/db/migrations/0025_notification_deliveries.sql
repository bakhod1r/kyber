-- Email and Telegram deliveries of notifications: an at-least-once queue with retries and a dead-letter state.
CREATE TABLE notification_deliveries (
    id           uuid PRIMARY KEY,
    workspace_id uuid NOT NULL,
    channel      text NOT NULL CHECK (channel IN ('email', 'telegram')),
    recipient    text NOT NULL,
    subject      text NOT NULL DEFAULT '',
    body         text NOT NULL,
    key          text NOT NULL UNIQUE,
    status       text NOT NULL CHECK (status IN ('pending', 'sent', 'dead')),
    attempts     int NOT NULL DEFAULT 0,
    next_at      timestamptz NOT NULL,
    last_error   text NOT NULL DEFAULT '',
    created_at   timestamptz NOT NULL,
    sent_at      timestamptz
);
CREATE INDEX notification_deliveries_due ON notification_deliveries (next_at) WHERE status = 'pending';
