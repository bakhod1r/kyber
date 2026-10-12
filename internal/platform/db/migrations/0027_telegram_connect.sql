-- Links that connect a user's Telegram chat for notifications: stored by SHA-256 hash, single use, 10 minutes.
CREATE TABLE telegram_connect_tokens (
    hash       text PRIMARY KEY,
    user_id    uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    expires_at timestamptz NOT NULL
);
