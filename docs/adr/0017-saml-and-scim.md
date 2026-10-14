# ADR-0017: SAML 2.0 sign-in and SCIM 2.0 provisioning

Status: accepted (2026-10-14)

## Context

Enterprises run identity providers (Okta, Entra ID, ADFS, Keycloak) that speak SAML rather than
OpenID Connect, and they provision and deprovision accounts centrally. Leavers must lose access the
moment HR disables them.

## Decision

- **SAML service provider** (`internal/identity/infrastructure/saml`, crewjam/saml): SP-initiated
  sign-in only. `/api/v1/auth/saml/start` redirects with an (unsigned) AuthnRequest over HTTP-Redirect
  and stores the request id in the `kyber_saml` cookie (SameSite=None on HTTPS, since the response is
  a cross-site POST). `/api/v1/auth/saml/acs` accepts the response only if it is signed by the
  configured identity provider, addressed to this ACS and audience, inside its validity window and
  answers the cookie's request id (IdP-initiated responses are refused). The email comes from the
  `email`/`mail` attribute or an email-shaped NameID; the identity provider is the organisation's
  directory, so it counts as verified and links to an existing account, like ADR-0015. Provider
  `saml`, subject = NameID. Configured by `KYBER_SAML_IDP_METADATA_URL` (or `_FILE`) and the SP's
  `KYBER_SAML_CERT_FILE`/`KYBER_SAML_KEY_FILE`; the SP metadata is served at
  `/api/v1/auth/saml/metadata`.
- **SCIM 2.0** at `/scim/v2` (Users, ServiceProviderConfig), authenticated by one bearer token
  (`KYBER_SCIM_TOKEN`, compared in constant time). `userName` is the email. Create provisions a
  password-less user (sign-in through SSO); PUT/PATCH change the display name and `active`; filters
  support `userName eq "…"`. DELETE **deactivates** rather than deleting, so issues and comments keep
  their author.
- **Deactivation** (`users.active`, migration 0084) is enforced where identity is established: no new
  session from any sign-in path, existing sessions deleted, and sessions and personal access tokens of
  an inactive user stop authenticating at once. Reactivation restores access (tokens included).

## Consequences

- Leavers lose access on the identity provider's next SCIM call; no Kyber admin action is needed.
- Groups are not provisioned (project roles stay in Kyber); single logout is not implemented.
