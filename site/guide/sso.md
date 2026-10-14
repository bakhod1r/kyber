# Single sign-on & SCIM

Besides email and password, Kyber signs users in with external providers. A **verified** email from a
provider links to an existing account with the same email. Set `KYBER_PUBLIC_URL` first — redirect URIs are
built from it.

## Google

Google Cloud → APIs & Services → Credentials → OAuth client (Web application). Authorized redirect URI:
`<KYBER_PUBLIC_URL>/api/v1/auth/google/callback`. Then set `KYBER_GOOGLE_CLIENT_ID` and
`KYBER_GOOGLE_CLIENT_SECRET`.

## OpenID Connect (Okta, Keycloak, Entra ID, Authentik, …)

Set `KYBER_OIDC_ISSUER`, `KYBER_OIDC_CLIENT_ID`, `KYBER_OIDC_CLIENT_SECRET` and optionally `KYBER_OIDC_NAME`
(button label, default `SSO`). The provider is found by discovery at
`<issuer>/.well-known/openid-configuration`. Register the redirect URI
`<KYBER_PUBLIC_URL>/api/v1/auth/oidc/callback`. The provider must send `email_verified=true`.

## GitHub

GitHub → Settings → Developer settings → OAuth Apps; callback URL
`<KYBER_PUBLIC_URL>/api/v1/auth/github/callback`. Set `KYBER_GITHUB_CLIENT_ID` and
`KYBER_GITHUB_CLIENT_SECRET`. Only the primary verified email links to an existing account.

## SAML 2.0

1. Set the identity provider's metadata: `KYBER_SAML_IDP_METADATA_URL` or `KYBER_SAML_IDP_METADATA_FILE`.
2. Give Kyber (the service provider) an RSA certificate and key in PEM: `KYBER_SAML_CERT_FILE`,
   `KYBER_SAML_KEY_FILE`.
3. Register `<KYBER_PUBLIC_URL>/api/v1/auth/saml/metadata` at the identity provider.
4. Optionally label the button with `KYBER_SAML_NAME` (default `SAML`).

Sign-in is SP-initiated only (IdP-initiated responses are refused). The email comes from the `email` / `mail`
attribute or an email-shaped NameID. Single logout is not implemented.

## SCIM 2.0 provisioning

Set `KYBER_SCIM_TOKEN` and configure your identity provider to call `<KYBER_PUBLIC_URL>/scim/v2` with that
value as a Bearer token. Supported: Users and ServiceProviderConfig; `userName` is the email; filters
`userName eq "…"`.

- Create provisions a password-less user who signs in through SSO.
- PUT/PATCH change the display name and `active`.
- DELETE **deactivates** the user (issues and comments keep their author). A deactivated user's sessions and
  personal access tokens stop working immediately; reactivation restores access.
- Groups are not provisioned — project roles stay in Kyber.

Design: [ADR-0015](https://github.com/bakhod1r/kyber/blob/main/docs/adr/0015-sso-providers.md),
[ADR-0017](https://github.com/bakhod1r/kyber/blob/main/docs/adr/0017-saml-and-scim.md).
