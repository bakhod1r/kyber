# ADR-0018: Several SSO connections and per-workspace SSO

Status: proposed (2026-10-15, S107) — awaiting approval before implementation.

## Context

ADR-0015 and ADR-0017 give an installation **one** OpenID Connect provider and **one** SAML identity
provider, configured by environment variables. A hosted or multi-company installation (ADR-0004)
needs more:

- several identity providers at once (company A on Okta, company B on Entra ID);
- a workspace that **requires** SSO, so its members cannot bypass the IdP with a password or Google;
- users routed to the right IdP by their email domain, without choosing from a long list.

## Decision

1. **SSO connection = a stored aggregate** in a new bounded context `internal/ssoconnect`
   (four layers, ADR-0005): `id`, `kind` (`oidc` | `saml`), `name` (button label), protocol settings
   (OIDC: issuer, client ID, client secret; SAML: IdP metadata URL or XML), `groups claim/attribute`
   (S106), `workspace_id` (nullable: null = installation-wide), `email domains` (verified, see 4),
   `enforced` (bool), `enabled`. Migration adds `sso_connections` and `sso_connection_domains`.
2. **Secrets.** Client secrets are encrypted at rest with AES-256-GCM under a new
   `KYBER_ENCRYPTION_KEY` (32 bytes, base64); without it, database-stored connections are disabled.
   The API never returns secrets (write-only field). SAML SP
   certificate and key stay installation-wide (`KYBER_SAML_CERT_FILE`/`KEY_FILE`) — one SP, many IdPs.
3. **Routing.** Routes become per connection: `/api/v1/auth/sso/{id}/start`, `/callback` (OIDC),
   `/acs` and `/metadata` (SAML; the SP entity ID gets the connection ID suffix). The login page asks
   for the email first when any connection has domains: `POST /api/v1/auth/sso/discover {email}`
   → the connection for that domain (or none → today's options). The existing env-configured
   providers become a read-only **"environment" connection** with fixed ID `env-oidc` / `env-saml`
   and keep their old callback URLs, so upgrades need no IdP changes.
4. **Domain ownership.** A domain is usable for routing and enforcement only after a DNS TXT record
   `kyber-verify=<token>` is found (as planned for custom domains, ADR-0004 §8). Installation-wide
   connections created by a site admin may skip verification (`--trusted`), since the operator owns
   the install.
5. **Enforcement.** When a workspace has an enforced connection, its members (except a break-glass
   list: workspace owners) can only enter it with a session whose `auth_method` is that connection.
   Password, Google, GitHub and Telegram logins still work for the account but the workspace answers
   `403 SSO_REQUIRED` with the connection to use. Sessions record `auth_method` (new column).
6. **Who manages what.** Site admins manage installation-wide connections; workspace admins manage
   their workspace's connections and its group mappings (S106 mappings gain `connection_id`,
   nullable = any connection). All changes are audited.
7. **Linking rules are unchanged** (ADR-0015): identities are keyed by `(provider = sso:<id>, subject)`;
   a verified email links to an existing account. A connection without verified-email support is
   accepted only for its verified domains.

## Consequences

- Hosted Kyber can onboard companies with their own IdP without a restart.
- More surface: secrets in the database, DNS verification, and a login page that asks for the email
  first. Threat model (`/threat-model`) required before shipping: IdP confusion between connections,
  domain takeover, enforcement bypass via API tokens (PATs and MCP tokens of an enforced workspace
  keep working only if issued under an SSO session).
- Delivery in three PRs: (a) connections + routing with the env connection, (b) domain verification
  + discovery, (c) enforcement + per-workspace admin UI.

## Alternatives considered

- **Keep env vars, allow lists (`KYBER_OIDC_1_*`)** — needs restarts and cannot be self-served by a
  workspace; rejected.
- **Delegate to an external broker (Keycloak, Dex)** — simplest for us but pushes operators to run
  another service, against the self-hosted single-binary goal; documented as an option, not chosen.
