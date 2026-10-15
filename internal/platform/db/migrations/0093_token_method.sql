-- ADR-0018 §5 follow-up: a personal access or OAuth (MCP) token remembers how its creator had signed in,
-- so a token made through a workspace's required SSO connection works there. Older tokens have none.
ALTER TABLE personal_tokens ADD COLUMN auth_method text NOT NULL DEFAULT '';
ALTER TABLE oauth_codes ADD COLUMN auth_method text NOT NULL DEFAULT '';
