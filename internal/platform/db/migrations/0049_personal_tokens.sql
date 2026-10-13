-- Personal access tokens (ADR-0012): long-lived credentials for scripts and AI assistants (MCP).
-- Only the SHA-256 hash of the secret is stored. Users are global, so no workspace_id (no RLS).
CREATE TABLE personal_tokens (
    id           uuid PRIMARY KEY,
    user_id      uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    name         text NOT NULL,
    token_hash   bytea NOT NULL UNIQUE,
    created_at   timestamptz NOT NULL,
    expires_at   timestamptz,
    last_used_at timestamptz
);
CREATE INDEX personal_tokens_user ON personal_tokens (user_id, created_at DESC);
