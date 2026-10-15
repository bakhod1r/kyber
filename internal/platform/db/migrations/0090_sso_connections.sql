-- S107 (ADR-0018): identity providers stored in the database. The client secret is sealed with
-- AES-256-GCM under KYBER_ENCRYPTION_KEY and never leaves the server. Site-wide configuration
-- (read at sign-in, before any workspace is known), so no row-level security.
CREATE TABLE sso_connections (
    id            uuid PRIMARY KEY,
    kind          text        NOT NULL CHECK (kind IN ('oidc')),
    name          text        NOT NULL,
    issuer        text        NOT NULL,
    client_id     text        NOT NULL,
    client_secret bytea       NOT NULL,
    groups_claim  text        NOT NULL DEFAULT '',
    workspace_id  uuid        REFERENCES workspaces (id) ON DELETE CASCADE,
    enabled       boolean     NOT NULL DEFAULT true,
    created_at    timestamptz NOT NULL
);

-- Identities from a stored connection are "sso:<connection id>", so two IdPs' subjects never collide.
ALTER TABLE external_identities DROP CONSTRAINT external_identities_provider_check;
ALTER TABLE external_identities ADD CONSTRAINT external_identities_provider_check
    CHECK (provider IN ('google', 'telegram', 'oidc', 'github', 'saml') OR provider LIKE 'sso:%');
