# ADR-0004 — Workspaces on subdomains (`acme.kyber.example.com`)

- **Status:** Accepted (2026-10-09), implementation in Sprint 09 (S33)
- **Related:** ADR-0003 (access control), S38 (hosted sign-up)

## Context
Jira Cloud gives every organization its own site (`acme.atlassian.net`). Kyber must support the same for
the hosted service, while a self-hosted single-company install must keep working on one plain domain.

## Decision
1. **Workspace = tenant, addressed by a slug.** `acme` → `https://acme.kyber.example.com`. Slugs are
   `[a-z0-9-]{3,32}`, unique, and a reserved list is refused (`www api app admin auth login static status
   docs mail help billing`).
2. **Two modes, one binary.**
   - `KYBER_BASE_DOMAIN` unset → *single-tenant*: every request belongs to the `default` workspace
     (today's behaviour, no DNS work).
   - `KYBER_BASE_DOMAIN=kyber.example.com` → *multi-tenant*: the apex serves the landing page, sign-up,
     login and "choose your workspace"; `<slug>.<base>` serves that workspace's app and API.
3. **Tenant resolution is middleware, not routing.** It reads the request host (only `Host`, or
   `X-Forwarded-Host` from a configured trusted proxy), strips the base domain, looks the slug up and puts
   the workspace id into the request context. Unknown slug → 404 page. Every repository query is filtered
   by `workspace_id` (ADR-0003 adds it to every access decision); Postgres RLS is the second line of defence.
4. **Sessions are per workspace.** The session cookie is host-only (no `Domain=` attribute), so a session
   for `acme` is never sent to `globex`. A user may belong to several workspaces.
5. **One login surface on the apex.** OAuth providers need fixed redirect URIs, and the Telegram widget is
   bound to one domain, so Google/Telegram/password login all happen on the apex. Afterwards the apex
   redirects to `https://<slug>.<base>/api/v1/auth/handoff?t=<one-time token>`; the token (random, 60 s,
   single use, bound to user + workspace) is exchanged for a host-only session cookie there.
6. **Infrastructure.** Wildcard DNS `*.kyber.example.com` and a wildcard TLS certificate (Let's Encrypt
   DNS-01 via Caddy, Traefik or cert-manager). The Helm chart and docker-compose ship a Caddy example.
7. **Local development.** Browsers resolve `*.localhost` to 127.0.0.1, so `KYBER_BASE_DOMAIN=localhost`
   gives `http://acme.localhost:8080` with no hosts-file edits.
8. **Later: custom domains** (`jira.acme.com` → CNAME to Kyber, ownership proved by a TXT record,
   certificates via on-demand TLS).

## Consequences
- ✅ Jira-like URLs, strong isolation (cookies, queries, RLS), self-hosting unchanged.
- ⚠️ Every request pays a slug lookup (cached in memory, invalidated on rename).
- ⚠️ The Host header becomes security-relevant: only the configured base domain is accepted.
- ⚠️ Workspace rename changes the URL; old slugs redirect for 90 days and cannot be reused meanwhile.
