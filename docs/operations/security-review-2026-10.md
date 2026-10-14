# Security review — October 2026

Internal review before v0.1.0. It is not an external penetration test; that remains open in
`docs/PLAN.md`.

## Scope and method

- **Code review** of OAuth 2.1 for MCP (ADR-0016), SAML/SCIM and deactivation (ADR-0017) and API rate
  limiting: every finding traced end to end before it was accepted.
- **Static analysis:** `gosec` v2.22.4 over all Go packages; `npm audit` (web, site): 0 vulnerabilities.
  `govulncheck` runs in CI (`make audit`); vuln.go.dev was not reachable from the review host.
- **Black-box probes** against a running server with PostgreSQL: security headers, CORS, TRACE,
  oversized bodies, JQL injection, path traversal, cross-project access, cookie flags.

## Findings and fixes

| # | Severity | Finding | Fix |
|---|---|---|---|
| 1 | Medium | A SCIM-deactivated user kept receiving live MQTT events on an open connection | the broker re-checks that the user is active on every subscribe and delivery and disconnects an inactive one |
| 2 | Low | OAuth code exchange issued a token to a user deactivated after consent | exchange requires an active user (`invalid_grant`) |
| 3 | Low | OAuth dynamic client registration had no limit | 10 registrations per IP per 15 minutes, 429 + `Retry-After` |
| 4 | Low (hardening) | Consent POST accepted non-JSON bodies | JSON required (CSRF defence no longer rests on cookie attributes alone) |
| 5 | Low (hardening) | Rate limiting failed open when Redis was down | falls back to the in-process limiter |
| 6 | Low (hardening) | No HSTS behind HTTPS | `Strict-Transport-Security: max-age=31536000; includeSubDomains` when secure cookies are on |

## Checked and sound

- OAuth redirect matching (exact; RFC 8252 loopback ports), `?next=` and deny redirects, PKCE S256 with
  constant-time comparison, atomic single-use codes, tokens cannot approve clients or manage tokens.
- SCIM bearer token compared in constant time; bodies capped (64 KiB) and page size bounded.
- SAML responses must be signed by the configured IdP, answer this browser's request (no IdP-initiated
  responses), and match audience, destination and validity window.
- Black-box: CSP (`frame-ancestors 'none'`, `base-uri 'none'`), `nosniff`, `Referrer-Policy`; no CORS
  for foreign origins; 20 MB bodies refused; JQL injection is parsed as data; traversal ids return 404;
  other projects' issues, labels and reports answer 404 and search returns nothing; session cookie is
  HttpOnly, SameSite=Lax (Secure behind HTTPS).
- gosec findings triaged as false positives or accepted (see [release.md](release.md)).
