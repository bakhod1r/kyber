# ADR-0016: OAuth 2.1 for MCP clients

Status: accepted (2026-10-14)

## Context

MCP clients (Claude, Cursor and others) implement the MCP authorization spec (revision
2025-06-18): on a 401 they discover the authorization server, register themselves and run the
authorization code flow with PKCE. Until now a user had to create a personal access token
(ADR-0012) and paste it into the client's configuration.

## Decision

- **Kyber is both resource and authorization server.** A 401 on `/api/v1/mcp` carries
  `WWW-Authenticate: Bearer resource_metadata="…/.well-known/oauth-protected-resource"` (RFC 9728).
  `/.well-known/oauth-authorization-server` (RFC 8414) lists the endpoints. The base URL is
  `KYBER_PUBLIC_URL`, or the request's scheme and host.
- **Public clients only.** `POST /api/v1/oauth/register` (RFC 7591) registers a client without a
  secret. Redirect URIs must be https, loopback http (any port at use time, RFC 8252 §7.3) or a
  private-use scheme with a dot. Only `authorization_code` with PKCE `S256` is supported.
- **Consent in the web app.** `/oauth/authorize` is a page of the SPA: a signed-out user is sent
  through login (`?next=`) and back. Approval is `POST /api/v1/oauth/authorize`, which only a
  browser session may call (not a personal access token), and returns the redirect with
  `code`, `state` and `iss`. Deny returns `error=access_denied`; the SPA only redirects to an
  address registered for the client.
- **Codes.** 32 random bytes, stored as SHA-256 only (`oauth_codes`), single use (deleted on
  exchange in one statement), ten minutes. The exchange must repeat the client and redirect
  URI and present the PKCE verifier; any mismatch is `invalid_grant`.
- **Tokens.** The token endpoint (`POST /api/v1/oauth/token`, form-encoded, `Cache-Control:
  no-store`) issues a personal access token named `MCP: <client>` that lives 90 days. It is
  listed and revoked like any other token, carries the user's permissions, and cannot manage
  tokens or approve clients. No refresh tokens: the client authorizes again after 90 days.

## Consequences

- Connecting an assistant is: add the URL, sign in, Allow.
- Registration is open to anyone (as the spec expects); registered clients are inert until a user
  approves one, and the general per-user rate limit does not cover anonymous registration yet.
- Scoped tokens (read-only, per project) remain a possible extension of ADR-0012.
