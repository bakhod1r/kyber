-- Sprint 09 (KYB-S40): log in with a one-time code sent by the Telegram bot.
CREATE TABLE otp_challenges (
    id          uuid PRIMARY KEY,
    nonce       text NOT NULL UNIQUE,
    expires_at  timestamptz NOT NULL,
    telegram_id text NOT NULL DEFAULT '',
    name        text NOT NULL DEFAULT '',
    code_hash   bytea,
    attempts    integer NOT NULL DEFAULT 0,
    used        boolean NOT NULL DEFAULT false
);
CREATE INDEX otp_challenges_expires ON otp_challenges (expires_at);
