-- Per-user notification channels (email, Telegram); absent row = both on.
CREATE TABLE notification_prefs (
    user_id  uuid PRIMARY KEY REFERENCES users (id) ON DELETE CASCADE,
    email    boolean NOT NULL,
    telegram boolean NOT NULL
);
