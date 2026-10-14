-- F6: sign in with a generic OpenID Connect provider (SSO) and GitHub.
ALTER TABLE external_identities DROP CONSTRAINT external_identities_provider_check;
ALTER TABLE external_identities ADD CONSTRAINT external_identities_provider_check
    CHECK (provider IN ('google', 'telegram', 'oidc', 'github'));
