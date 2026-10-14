# ADR-0015: Single sign-on with OpenID Connect and GitHub

Status: accepted (2026-10-14).

## Context

Kyber signs users in with a password, Google and Telegram. Organisations want their own identity
provider (Okta, Keycloak, Entra ID, Authentik…), and developer teams want GitHub. Both should link
to existing accounts exactly as Google does, without new trust rules.

## Decision

- **Generic OpenID Connect.** `internal/identity/infrastructure/oidcprovider` is the Google adapter
  generalised: discovery at `<issuer>/.well-known/openid-configuration`, authorization code flow
  with PKCE (S256), state and nonce, and the ID token verified against the provider's JWKS
  (signature, issuer, audience, expiry, nonce). Google is now a preset of it
  (`infrastructure/google`: Google's issuer plus `prompt=select_account`). The installation's own
  provider is configured by `KYBER_OIDC_ISSUER`, `KYBER_OIDC_CLIENT_ID`, `KYBER_OIDC_CLIENT_SECRET`
  and labelled by `KYBER_OIDC_NAME` (default "SSO"). It **requires `email_verified=true`**: a token
  without it is refused, so an IdP that does not vouch for addresses cannot sign anyone in.
  Provider `oidc`, subject = the ID token's `sub`. One provider per installation.
- **GitHub.** `infrastructure/github`: OAuth web flow with state (and PKCE), then `GET /user`
  (subject = numeric user id, which survives renames) and `GET /user/emails`; only the primary
  verified email is vouched for. Without one the account gets a synthetic address, like Telegram.
  The web and API base URLs are configurable so tests run against `httptest` fakes.
- **One redirect flow in HTTP.** `/api/v1/auth/{google,oidc,github}/{start,callback}` share one
  handler: the `kyber_oauth` cookie (state.nonce.verifier, path-scoped per provider) is compared to
  `state` in constant time (login CSRF); failures redirect to `/login?error=<provider>_failed` or
  `<provider>_email_taken`. `GET /api/v1/auth/providers` adds `oidc` (label or null) and `github`.
- **Linking** is unchanged (`Service.LoginExternal`): a linked identity wins; otherwise a verified
  email links to the existing account; an unverified one never takes it over.
- Migration `0053` widens the `external_identities.provider` check to `oidc` and `github`.

## Consequences

- Adding another OpenID provider preset is a few lines; the verification code exists once.
- The `oidc` subject is only unique per issuer. Changing `KYBER_OIDC_ISSUER` to another IdP keeps
  the old links; accounts re-link by verified email.

## Follow-ups (out of scope)

- **SAML 2.0** sign-in (SP metadata, signed assertions, IdP-initiated login) for IdPs without OIDC.
- **SCIM 2.0** provisioning (create/deactivate users and groups from the IdP).
- Several OIDC providers per installation, and per-workspace SSO (ADR-0004) with enforced SSO and
  email-domain routing.
- Group/role claims mapped to Kyber roles (ADR-0003).
