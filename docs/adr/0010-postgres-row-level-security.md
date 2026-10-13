# ADR-0010: PostgreSQL row-level security

Status: accepted (2026-10-13)

## Context

Workspaces share one schema (ADR-0004); every repository filters by `workspace_id`. One missed
filter in a hand-written query (ADR-0002) would leak another tenant's rows.

## Decision

- **Policy.** Migration 0047 enables and forces RLS on every table with a `workspace_id` column,
  with one `tenant_isolation` policy for reads and writes: rows must belong to the workspace in the
  session setting `kyber.workspace`, or the setting is empty.
- **Scoping.** The pool's `PrepareConn` hook (`db.OpenConfig`) sets `kyber.workspace` from the
  request's tenant (`tenant.From`) whenever a connection is acquired, and clears it for unscoped
  callers. The value is cached per connection, so the extra statement runs only when it changes.
- **Unscoped work.** Migrations, the outbox relay, dispatch and reminder jobs carry no tenant and
  keep full access; they are trusted code that spans workspaces by design.
- **Role.** Superusers and `BYPASSRLS` roles ignore policies. Kyber must connect as a plain role
  that owns its tables (`FORCE` applies the policy to the owner). The test suite runs as such a
  role (`dbtest.Role`), so every PostgreSQL test exercises the policies.

## Consequences

- RLS is defense in depth; repository queries keep their explicit `workspace_id` filters (and the
  indexes that serve them).
- Tables without `workspace_id` (users, sessions, workspaces) are not covered; their access stays
  in application code.
- The default docker-compose user is a superuser, so the policies are inert there; production
  deployments should create a dedicated non-superuser role.
