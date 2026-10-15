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

## Several identity providers (SSO connections)

Besides the one OpenID Connect provider set by environment variables, site administrators can add any
number of OpenID Connect providers on the **Administration** page (**SSO connections**), without a restart:

1. Set `KYBER_ENCRYPTION_KEY` (`openssl rand -base64 32`); client secrets are encrypted with it.
2. Add the connection: button label, issuer URL (https), client ID, client secret, and optionally the groups claim.
3. Register the callback URL shown under the connection (`<KYBER_PUBLIC_URL>/api/v1/auth/sso/<id>/callback`)
   at the provider.

Each enabled connection gets its own "Continue with …" button. Disabling one hides the button; deleting one keeps
the accounts, which still sign in with their other methods. Secrets are never shown again after saving.

### Route people by their email domain

Add the company's email domains to its connection. Kyber shows a TXT record (`kyber-verify=…`) to publish on
each domain; press **Verify** once it is live. A verified domain routes sign-in: on the login page people type
their work email and get their company's button. A site admin may mark a domain **trusted** to skip the DNS
check (for domains the installation's owner controls). A domain belongs to one connection only.

## Groups → roles

Site administrators can map an identity-provider group to a role on the **Administration** page
(**SSO group mappings**): a workspace role (`member`, `admin`) or a project role (`viewer`, `member`,
`admin`). At every OIDC or SAML sign-in Kyber grants the highest mapped role and removes roles it granted
earlier when the user has left the group. Roles given by hand are never removed. Every change is audited.

- OIDC: groups come from the `groups` claim; set `KYBER_OIDC_GROUPS_CLAIM` for another claim.
- SAML: groups come from the `groups` or `memberOf` attribute; set `KYBER_SAML_GROUPS_ATTRIBUTE` to override.

If the provider sends no groups, roles stay as they are.

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
