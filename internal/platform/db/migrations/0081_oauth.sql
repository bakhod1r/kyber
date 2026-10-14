-- OAuth 2.1 for MCP clients (ADR-0016): dynamically registered public clients and single-use
-- authorization codes (hash only, PKCE S256). Clients and users are global: no workspace_id.
CREATE TABLE oauth_clients (
    id            uuid PRIMARY KEY,
    name          text NOT NULL,
    redirect_uris text[] NOT NULL,
    created_at    timestamptz NOT NULL
);

CREATE TABLE oauth_codes (
    code_hash    bytea PRIMARY KEY,
    client_id    uuid NOT NULL REFERENCES oauth_clients (id) ON DELETE CASCADE,
    user_id      uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    redirect_uri text NOT NULL,
    challenge    text NOT NULL,
    expires_at   timestamptz NOT NULL
);
CREATE INDEX oauth_codes_expiry ON oauth_codes (expires_at);
