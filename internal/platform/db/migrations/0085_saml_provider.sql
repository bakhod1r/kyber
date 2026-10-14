-- ADR-0017: sign in with a SAML 2.0 identity provider.
ALTER TABLE external_identities DROP CONSTRAINT external_identities_provider_check;
ALTER TABLE external_identities ADD CONSTRAINT external_identities_provider_check
    CHECK (provider IN ('google', 'telegram', 'oidc', 'github', 'saml'));
