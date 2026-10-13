# ADR-0012: Personal access tokens

Status: accepted (2026-10-13)

## Context

Scripts and AI assistants (the MCP endpoint, ADR-0009) used session tokens, which expire after 30
days and are tied to a browser sign-in. They need a credential of their own that can be named,
listed and revoked.

## Decision

- **Shape.** A token is `kpat_` + 32 random bytes (base64url). Only its SHA-256 hash is stored
  (`personal_tokens`), with a name, creation time, optional expiry (1-365 days, or none) and the
  time of last use (recorded at most once a minute).
- **Use.** Sent as `Authorization: Bearer kpat_...`; `Authenticate` tells it from a session token
  by the prefix. The token acts with its owner's full permissions in every workspace the owner
  belongs to; workspace membership and project roles still decide every request.
- **Management.** `GET/POST /api/v1/me/tokens`, `DELETE /api/v1/me/tokens/{id}`; the secret is
  returned once, at creation. Requests authenticated by a token cannot manage tokens, so a leaked
  token can neither mint successors nor hide by revoking others; revocation takes effect at once.
- **UI.** Settings → Access tokens.

## Consequences

- MCP clients and scripts get stable credentials; the README shows Claude Code with one.
- Scopes (read-only tokens, per-project tokens) are a possible extension; today a token is as
  powerful as its owner.
