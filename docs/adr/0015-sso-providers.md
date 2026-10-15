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

## Amendment (2026-10-15, S106): group → role mapping

- A new bounded context `internal/ssosync` owns **group mappings** (`sso_group_mappings`: group,
  scope `workspace|project`, workspace, project key, role) and the **grants SSO made**
  (`sso_grants`). Site admins manage mappings at `GET/POST /api/v1/admin/sso/group-mappings` and
  `DELETE …/{id}` (Administration page).
- Groups come from the OIDC claim `KYBER_OIDC_GROUPS_CLAIM` (default `groups`; a string or a list)
  and the SAML attribute `KYBER_SAML_GROUPS_ATTRIBUTE` (default `groups`, `memberOf` or a claim URI
  ending in `/groups`). A provider that sends no groups leaves roles untouched (AC4).
- On every external login, after the session is created, `Plan()` (pure domain) compares the user's
  groups (case-insensitive) with the mappings: the highest mapped role per target is granted; a target
  the user no longer qualifies for is revoked **only if SSO granted it**. Manual grants are never
  taken away; the last owner/admin guards still apply. Default-workspace membership is implicit.
- Grants and revokes run through new actor-less system methods on the workspace and project
  services, each in the target's tenant, and every change is audited (`sso.role_granted`,
  `sso.role_revoked`). A failing sync is logged and never blocks sign-in.
