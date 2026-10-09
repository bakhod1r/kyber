# Sprint 02 — "Safe to self-host" (persistence + auth + ops)

**Owner:** Product Team · **Executes:** Backend, Platform · **Verifies:** Quality, Security

## Problem
Sprint 01 alpha loses all data on restart and is open to anyone on the network. No team can
adopt a tracker that forgets its issues or that strangers can edit.

## Sprint goal (measurable)
A self-hosted instance backed by PostgreSQL keeps all projects/issues across restarts, and every
`/api/v1` call except signup/login requires an authenticated user.
**Done =** all ACs automated (unit + Postgres integration + acceptance), CI green with a Postgres service.

## Non-goals
OIDC/SSO, roles & permissions per project (Sprint 03), email verification, password reset, UI.

## Kill criterion
If optimistic locking cannot be expressed through the repository port without leaking SQL into
the domain, stop and revise ADR-0008.

## Stories

### KYB-S6 — Durable storage  ·  P0
- AC1: with `KYBER_DATABASE_URL` set, projects and issues persist in PostgreSQL and survive restart.
- AC2: schema migrations are embedded and applied automatically and idempotently at startup.
- AC3: without `KYBER_DATABASE_URL` the server runs in in-memory dev mode and logs a warning.
- AC4: memory and Postgres repositories pass the **same contract test suite**.

### KYB-S7 — Concurrent edits are safe  ·  P0
- AC1: each issue has a `version`; saving a stale version fails with `409 Conflict`.
- AC2: concurrent issue creation never produces duplicate keys (row lock on project sequence).

### KYB-S8 — Domain events are durable (outbox)  ·  P1
- AC1: issue changes and their domain events are written in the **same transaction** (`outbox` table).
- AC2: a failed save writes neither the issue change nor its events.

### KYB-S9 — Sign up / log in / log out  ·  P0
- AC1: `POST /api/v1/auth/signup {email,name,password}` → `201`; email normalised (trim, lower-case), unique (`409`); password ≥ 10 chars (`422`).
- AC2: `POST /api/v1/auth/login {email,password}` → `200`, sets `kyber_session` cookie (HttpOnly, SameSite=Lax, Secure configurable) and returns `token` for API clients.
- AC3: wrong email or password → `401` with the **same** message (no user enumeration).
- AC4: `GET /api/v1/me` → current user; `POST /api/v1/auth/logout` invalidates the session.
- AC5: passwords stored with argon2id; session tokens stored only as SHA-256 hashes; sessions expire after 30 days.

### KYB-S10 — API requires authentication  ·  P0
- AC1: every `/api/v1/*` route except `auth/signup` and `auth/login` → `401` without a valid session.
- AC2: accepts `Cookie: kyber_session=…` or `Authorization: Bearer …`.

### KYB-T2 — Operability  ·  P1
- `/readyz` → `200` when the database is reachable, else `503`.
- `/metrics` → Prometheus text format: request count by method/status and latency sum.
- CI runs integration tests against a Postgres service container.
