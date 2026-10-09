-- Sprint 09 (KYB-S39): sign in with Google and Telegram. One row per provider account.
CREATE TABLE external_identities (
    provider   text NOT NULL CHECK (provider IN ('google', 'telegram')),
    subject    text NOT NULL,
    user_id    uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (provider, subject)
);
CREATE INDEX external_identities_user ON external_identities (user_id);
